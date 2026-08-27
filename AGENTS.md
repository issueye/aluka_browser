# AGENTS.md

面向 AI 编码代理（以及人类协作者）的项目工作指南。

## 项目简介

**gio-browser**：基于 Gio UI + Microsoft WebView2 的无边框多标签桌面浏览器（仅 Windows）。内置篡改猴式用户脚本系统、Chrome/Edge 式扩展系统（开发者模式）与面向 AI Agent Tool Calling 的纯 Go 动作分发层。约 5700 行 Go 代码，无前端构建链，单二进制输出，零本地路径依赖。

## 常用命令

```bash
go build -o gio-browser.exe .   # 构建并运行 GUI 程序
go test ./...                   # 运行全部测试
go test ./internal/userscript/ -run TestMatchPattern -v   # 单测某个用例
gofmt -w <file>                 # 格式化
```

### 构建前提

- 运行需要 WebView2 Runtime；程序为 Windows 专属（大量 syscall），不能交叉编译到其他平台调试。
- Go 版本要求 1.25+（见 go.mod）。
- 无任何本地路径依赖：克隆后 `go build ./...` 即可编译全部包。

## 架构与依赖规则

分层严格单向，改动时必须遵守：

```
main.go → app(装配) ─┬─→ ui      ──只读──→ browser(领域模型)
                     └─→ webview ──实现──→ browser.Engine 接口
scripting ─→ browser / userscript / config
extension ─→ userscript(复用匹配算法)；webview ─→ extension(注入)
procs ─→ win32(进程枚举/终止)；ui(浮窗) ─→ procs / win32
webview / ui ─→ win32(syscall 封装)
```

- **win32 进程 API 的坑（已修复，勿回退）**：
  - 内存查询必须用 `kernel32!K32GetProcessMemoryInfo`（psapi.dll 转发存根返回垃圾数据），且 counters 结构体需按 `PROCESS_MEMORY_COUNTERS_EX` 尺寸 72 字节传入（传标准 64 会被以 ERROR_INSUFFICIENT_BUFFER 拒绝）。
  - 工作集裁剪用 `kernel32!SetProcessWorkingSetSize`（本机验证无 K32/Ex 变体导出，psapi.dll 亦无）；访问掩码需 `PROCESS_ALL_ACCESS`（SET_QUOTA|QUERY_LIMITED 对 Chromium 子进程可能失败）。
- **后台标签内存优化机制**：`SwitchTab` 必须同时隐藏子窗口**和**调用 `Chromium.Hide()`（controller 不可见）——漏掉后者 Chromium 会以全速继续渲染后台标签（实测每标签空烧 ~9% 单核）。janitor（`manager.go`）每 30s 扫描：后台空闲超阈值即 `win32.TrimProcessTree` 裁剪工作集（排除宿主自身）。`TrySuspendAsync`（`suspend.go`，槽位 68/69/70，已经 webview2-sys 官方绑定核对）在运行时 151.0.4129.107 上实测崩溃（上游 WebView2Feedback #2121），默认禁用、仅 `GIO_SUSPEND_MODE=api` 启用。
- **go-webview2 fork 的 vtable 忠实官方布局**（58+7+5，与 webview2-sys 逐一核对过），fork 额外提供了 `Chromium.GetICoreWebView2_3()` QI 封装（IID {A0D6DF20-…} 为官方值）。
- 多 Gio 窗口（进程浮窗）模式：`ui.RunProcessPanel` 在独立 goroutine 创建第二个 `app.Window`，装配层用 `atomic.CompareAndSwap` 保证单实例。

- **`browser.Engine` 是页面引擎的唯一抽象边界**（`internal/browser/browser.go:31`）。给浏览器增加新能力时，先扩展 `Engine` 接口 + 测试用 mockEngine（见 `internal/scripting/engine_test.go`），再在 `webview.Manager` 落地实现；禁止 `browser` 反向 import `webview` 或任何 GUI/win32 包。
- **`ui` 包不接触 WebView2、Win32、config 之外的存储细节**：它只读渲染 `browser.Browser` 模型，用户交互一律回调模型方法（改状态后再 `win.Invalidate()`）。
- **syscall 一律收敛在 `win32` 包**，上层不得直接出现 `syscall.SyscallN`/`unsafe.Pointer`（`webview/manager.go` 中对 COM vtable 的直接调用是唯一历史例外，勿扩散此模式）。

## 线程模型（改代码前必读）

1. **WebView2 全部操作必须运行在专有 STA 线程上**（COM 单元套间约束）。公开方法一律以 `m.dispatch(func(){...})` 投递（`webview/host.go`），通过 `PostThreadMessage(WM_APP_ACTION)` 唤醒消息循环串行执行——新方法照抄该模式，不要在调用方 goroutine 直接摸 `Chromium`。
2. 引擎回调（`onState`/`onOpenTab`）也在 STA 线程触发：回调内先加锁更新 `browser.Browser`，再调用 `win.Invalidate()` 让 Gio 主循环重绘。不要在回调里直接操作 UI 控件或长阻塞。
3. `Manager` 内部字段跨线程访问必须持 `m.mu`；读取后要在锁外使用。
4. `app.Run` 每帧把内容区像素矩形同步给 `engine.SetBounds`（物理像素 = DIP × 缩放），布局度量来自 `ui.FrameMetrics`，两处高度口径必须一致。

## 关键行为约定（有测试锁定，勿破坏）

- 标签页至少保留一个：关闭最后一个标签 = 导航回主页（`browser.CloseTab`）。
- 切换到已活跃标签不做任何事，绝不触发刷新（`browser.SwitchTab`）。
- 后台标签只 Hide 子窗口，不销毁实例（保持滚动位置与 JS 状态）。
- 地址栏输入归一化：带协议直接用；含 `.` 无空格补 `https://`；纯数字端口形式视为网址；否则送 DuckDuckGo 搜索（`browser.NormalizeInputURL`）。
- UserScript 匹配优先级 `@exclude` > `@match` > `@include`；`<all_urls>` 与 `*://*/*` 全匹配（`userscript.MatchPattern`）。
- 用户脚本注入发生在 NavigationCompleted 回调中（`@run-at` 目前仅解析未生效）。
- 新窗口类链接（window.open / target=_blank / 中键 / Ctrl+点击）由注入脚本 postMessage `{type:"open_tab"}` 拦截转新标签页——修改注入脚本（`webview/inject.go`）时保持消息 JSON 结构与 `webViewMessage` 对应（现含 `ext_message` 扩展上行通道）。
- 扩展 content_scripts 与用户脚本共用 `userscript.MatchPattern` 匹配算法，优先级 `exclude_matches` > `matches`；chrome.* 沙箱以 IIFE 局部变量遮蔽，**绝不覆盖页面原生 `window.chrome`**（WebView2 通道依赖它）。
- exe 图标由 `tools/genicon`（复用 `app.IconImage` 视觉定义）生成 `icon.ico`，再经 `rsrc` 打包为根目录 `rsrc_windows_amd64.syso`，Go 工具链自动链接；两产物均已入库，常规构建无需 rsrc，改图标视觉后需 `go generate ./internal/app` + rsrc 重新产出。

## 数据与持久化

| 位置 | 内容 | 访问入口 |
|---|---|---|
| `%APPDATA%\gio-browser\config.json` | 代理配置 | `config.Load / Current / Save` |
| `%APPDATA%\gio-browser\userscripts.json` | 用户脚本集合 | `userscript.GetGlobalManager()`（sync.Once 单例） |
| `%TEMP%\gio_browser_profile` | WebView2 共享数据目录 | — |

代理变更的生效路径：`config.Save` → `applyProxyEnvLocked` 设置 `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS`（需重启应用对已建实例生效的语义要留意）。

## 代码风格

- 注释、日志文案、commit message 均使用中文；commit 遵循既有格式：`feat: 描述` / `fix: 描述`。
- 每个包文件顶部有一段 `// Package xxx ...` 中文职责说明，新增包请保持该习惯。
- 导出类型/函数必须有中文 doc comment；接口方法与对外行为变化同步更新注释。
- 界面按钮需要图标时使用 `ui.iconLabelButton`（矢量图标组合按钮）；不要用 emoji 文本作为图标——默认字体缺字会渲染成方块（此前修过该 bug）。
- 错误处理：底层返回包装错误（`fmt.Errorf("...: %w", err)`）；GUI/回调路径允许仅记 log。

## 测试指引

- 领域逻辑必须配表驱动测试：现有用例分布在 `browser`、`config`、`userscript`、`extension`（临时目录注册表 + mock 清单）、`procs`（合成数据树构建/过滤）、`scripting`（mockEngine 方式，无需 WebView2）、`ui/favicon`。
- `webview` 与 `win32` 无测试（强系统耦合），重构时以编译通过 + 手工验证为主；如可拆出纯逻辑（如消息解析、参数拼接），优先拆出并测试。
- 全部包的测试不依赖任何本地外部检出，可直接在 CI 上运行。
