# Gio Browser (gio-browser)

基于 **Gio UI** 与 **Microsoft WebView2** 的无边框多标签桌面浏览器（仅 Windows）。内置篡改猴（Tampermonkey）风格的用户脚本系统，并提供面向 AI Agent Tool Calling 的结构化动作分发层。

窗口标题栏、标签栏、工具栏、书签栏、状态栏、设置页等界面由 Gio 直接绘制（纯 GPU 渲染），网页内容则由每个标签页独享的原生 WebView2 实例承载，两层通过 Win32 子窗口无缝拼合。

## 功能特性

- **无边框自绘窗口**：macOS 风格红绿灯按钮、直角外观、拖拽移动。
- **多标签页**：每个标签页拥有独立的 WebView2 渲染实例，后台标签只隐藏不销毁——保留运行状态与滚动位置，切换零成本重载。
- **地址栏智能归一化**：自动补全协议、识别 `host:port`，非网址输入转为 DuckDuckGo 搜索。
- **新窗口拦截**：`window.open`、`target=_blank`、中键/Ctrl+点击均被拦截转为新标签页。
- **快捷访问栏**：条目由配置驱动，可在设置中心「快捷访问」中增删，即时生效。
- **设置中心标签页**：以 `gio://settings` 标签呈现（左侧导航 + 右侧内容），聚合主页设置、快捷访问管理、用户脚本（篡改猴）与扩展管理。
- **主页可配置**：主页地址持久化到配置，新标签页、回主页与关闭最后一个标签均使用该地址。
- **用户脚本管理器**：类篡改猴体验——元数据解析、URL 匹配、启停开关、在线编辑、本地持久化，并预置三个实用脚本（暗黑模式 / 解除复制限制 / 回到顶部悬浮球）。
- **扩展系统（开发者模式）**：参考 Chrome/Edge 的"加载已解压扩展"——解析 manifest.json（MV2/MV3）、content_scripts 自动注入、chrome.runtime/storage 最小兼容沙箱、扩展管理面板。
- **进程管理浮窗**：独立置顶小窗展示全系统进程树（父子缩进、内存占用、按名过滤、2s 自动刷新），高亮浏览器自身子树（含各标签的 msedgewebview2 进程族），子树内支持一键结束进程。
- **后台标签内存优化**：后台标签切走时即停止渲染活动；空闲超过阈值后自动对浏览器进程树做工作集裁剪，实测多标签场景物理内存可压缩 **-85%**（3 标签 559MB → 190MB），切回标签页面按需恢复、状态无损。阈值可用环境变量 `GIO_SUSPEND_AFTER_SEC`（秒，默认 180）调节；`GIO_SUSPEND_MODE=api` 可切换为实验性的 TrySuspendAsync 挂起路径（部分运行时版本存在崩溃问题，见 AGENTS.md）。
- **Agent 动作分发层**：14 个结构化动作（导航/标签/用户脚本管理），纯 Go 实现，可直接对接大模型 Function Calling 或自动化编排，无需嵌入任何脚本运行时。

## 架构

```
┌──────────────────────────────────────────────────┐
│ main.go                                          │
│   ├── goroutine: app.Run()  装配 + Gio 事件循环   │
│   └── 主 goroutine: app.Main() Gio 平台循环       │
├──────────────────────────────────────────────────┤
│ internal/app        应用装配：创建窗口、连接各层    │
│      │ 启动时轮询探测宿主 HWND（按标题匹配），       │
│      │ 找到后启动 WebView2 引擎线程                │
│      ▼                                            │
│ internal/browser    领域模型（纯 Go，无 GUI 依赖）  │
│   Browser 状态机 + Engine 接口 ←── 唯一抽象边界     │
│      ▲                    │                       │
│      │                    ▼                       │
│ internal/ui         internal/webview             │
│ Gio 全部界面组件      Engine 默认实现：            │
│ 只读渲染模型，        · 每标签独立 WebView2 实例    │
│ 交互回调模型方法      · 专有 STA 线程 + Win32       │
│                       消息循环，dispatch 序列化    │
│ internal/win32      Win32 API 封装（子窗口/焦点/  │
│                     图标/HWND 探测）               │
├──────────────────────────────────────────────────┤
│ internal/scripting   Agent 动作分发器：将结构化    │
│                      指令转发到各领域模型           │
│ internal/userscript  UserScript 解析/匹配/GM_*    │
│                      沙箱/JSON 存储                 │
│ internal/config      配置持久化（主页/快捷访问）         │
└──────────────────────────────────────────────────┘
```

### 线程模型

| 执行体 | 说明 |
|---|---|
| 主 goroutine | `gioapp.Main()` 平台主循环 |
| 应用 goroutine | `app.Run()`：窗口事件处理、每帧布局、把内容区矩形同步给页面引擎 |
| WebView2 STA 线程 | `runtime.LockOSThread` + `CoInitializeEx(APARTMENTTHREADED)`；所有 WebView2 操作经 `PostThreadMessage(WM_APP_ACTION)` 派发到该线程串行执行 |
| 回调汇聚 | 引擎状态回调（URL/标题/加载）回写 `browser.Browser`（内部互斥锁保护），随后 `win.Invalidate()` 触发重绘 |

依赖方向约束：`ui` 与 `browser` 互不感知页面引擎；`webview.Manager` 仅实现 `browser.Engine` 接口并在装配层注入。

## 目录结构

```
main.go                        入口
internal/
├── app/                       应用装配、事件循环、应用图标
├── browser/                   标签页状态机、Engine 接口、URL 归一化
├── config/                    配置持久化（主页、快捷访问）
├── scripting/                 Agent 动作分发器（纯 Go，无运行时依赖）
├── extension/                 Chrome/Edge 式扩展：manifest/注册表/注入/chrome.* 沙箱
├── procs/                     进程快照与树构建（纯逻辑，可独立测试）
├── ui/                        标签栏/工具栏/书签栏/状态栏/设置页/脚本面板
├── userscript/                元数据解析、@match 匹配算法、GM_* 沙箱、存储
├── webview/                   WebView2 多标签引擎、STA 宿主线程、注入脚本
└── win32/                     Win32 系统调用封装
```

## 构建与运行

### 环境要求

- **Windows 10 及以上**
- **Go 1.25+**
- **Microsoft Edge WebView2 Runtime**（Win11 一般自带，缺失时从 [微软官网](https://developer.microsoft.com/microsoft-edge/webview2/) 安装）

无本地路径依赖，克隆后即可直接构建：

```bash
git clone <本仓库>
cd aluka_browser
go build -o gio-browser.exe .
./gio-browser.exe
```

### 构建脚本

仓库内置一键构建脚本，自动完成**环境检查 → 单元测试 → 构建**三步，任一环节失败即中止并给出修复指引：

```bash
./build.sh          # Git Bash / Linux
build.bat           # Windows CMD
```

两脚本参数一致：

| 参数 | 说明 |
|---|---|
| （无） | 发布构建：`-trimpath -ldflags "-s -w -H windowsgui"`，隐藏控制台窗口 |
| `--dev` | 开发构建：保留控制台窗口，可查看运行日志 |
| `--skip-test` | 跳过单元测试直接构建 |
| `--test-only` | 只运行测试，不产出可执行文件 |
| `--clean` | 清理构建输出目录后退出 |

产物输出到 `build/gio-browser.exe`，输出目录可通过环境变量 `OUT_DIR` 覆盖。

> 说明：`build.bat` 的输出刻意采用纯 ASCII——CMD 在不同控制台代码页下解析含非 ASCII 字符的批处理文件会导致指令错乱；需要中文提示请用 `build.sh` 或参阅本文档。

### 测试

```bash
go test ./...
```

所有包均可在 Windows 环境独立构建与测试，无需额外检出任何外部仓库。

## 配置与数据存储

所有数据位于 `%APPDATA%\gio-browser\`：

| 文件 | 内容 |
|---|---|
| `config.json` | 主页地址、快捷访问列表（名称 + URL） |
| `userscripts.json` | 用户脚本集合（代码原文 + 解析后的元数据 + 启用状态） |
| `extensions/extensions.json` | 已加载扩展注册表 |

网络代理功能已移除，规划中将通过扩展机制实现。

## 用户脚本系统

完整解析篡改猴元数据块：

```
// ==UserScript==
// @name         脚本名称
// @namespace    https://example.com/
// @version      1.0.0
// @description  描述
// @author       作者
// @match        *://*.github.com/*
// @include      ...
// @exclude      *://github.com/settings/*
// @run-at       document-end | document-start | document-idle
// @grant        GM_addStyle
// ==/UserScript==
```

- **匹配优先级**：`@exclude` > `@match` > `@include`；支持 `<all_urls>`、`*://*.host/*` 通配语法。
- **注入时机**：当前固定在页面导航完成（NavigationCompleted）时注入，`@run-at` 已解析但尚未严格区分时机。
- **沙箱 GM_* API**：`GM_info`、`GM_addStyle`、`GM_setValue/getValue/deleteValue/listValues`（基于 localStorage 的脚本级命名空间持久化）、`GM_log`、`GM_setClipboard`、`GM_registerMenuCommand`、`unsafeWindow`。

## 扩展系统（开发者模式）

工具栏拼贴图标按钮打开扩展管理面板，输入已解压扩展的目录路径即可加载（不复制文件，指向源目录；同一目录重复加载视为更新）：

| 能力 | v1 支持范围 |
|---|---|
| manifest 解析 | MV2 / MV3；`name`、`version`、`description`、`permissions`（仅展示）、`icons`（未渲染） |
| content_scripts | `matches` / `exclude_matches`（复用用户脚本匹配算法）/ `js` / `css`；`run_at` 仅解析、注入固定在页面导航完成时 |
| chrome.* 沙箱 | `runtime.sendMessage/getManifest/onMessage(空)`、`storage.local|sync`（localStorage 持久化，Promise 风格）、`i18n.getMessage(透传)`；以局部变量遮蔽，不污染页面原生 `window.chrome` |
| 工具按钮 | `action` / `browser_action` 的 `default_popup` 以新标签页打开 |
| background | **未实现**（service worker / 事件页不加载） |

扩展 ID 由源目录路径派生（8 位十六进制，同目录恒定）；注册表持久化于 `%APPDATA%\gio-browser\extensions\extensions.json`，移除扩展不会删除源目录。

## Agent 动作分发接口

`scripting.ExecuteAgentAction(browser, action, params)` 是面向 AI Agent / Tool Calling 的唯一结构化入口，动作名大小写不敏感：

| 动作 | 参数 | 说明 |
|---|---|---|
| `open_url` / `navigate` | `url` | 当前活跃标签导航 |
| `create_tab` / `new_tab` | `url`, `title?` | 新建并激活标签页 |
| `switch_tab` / `close_tab` | `index` | 切换 / 关闭指定下标 |
| `get_tabs` / `list_tabs` | — | 全部标签信息（含 active 标记） |
| `page_eval` / `eval_js` | `script` | 在当前网页内执行 JavaScript |
| `go_back` / `go_forward` / `reload` | — | 导航控制 |
| `list_userscripts` / `add_userscript` / `toggle_userscript` / `delete_userscript` | 见源码 | 用户脚本管理 |

Go 调用示例：

```go
res, err := scripting.ExecuteAgentAction(b, "create_tab", map[string]any{
	"url": "https://example.com", "title": "示例",
})
```

该层为纯 Go 实现，无脚本运行时依赖；接入大模型编排时为其挂上本地 IPC/HTTP 入口即可。

## 当前已知限制

- 仅支持 Windows（深度依赖 Win32 与 WebView2）。
- 无会话/历史记录/收藏夹持久化，重启后回到默认主页与默认书签。
- 关闭最后一个标签页会回到主页而非退出应用（强制保留至少一个标签）。
- `@run-at` 与 `GM_registerMenuCommand` 尚未完全实现语义。
- 各标签页共用同一个 WebView2 数据目录（临时目录下的 `gio_browser_profile`）。
- 曾集成的 Aluka 脚本运行时已移除：上游将其实现收敛进 internal 包且未提供公共嵌入 API；现仅保留纯 Go 的 Agent 动作分发层。

## License

仅供学习研究使用。
