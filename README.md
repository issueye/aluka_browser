# Aluka Browser (gio-browser)

基于 **Gio UI** 与 **Microsoft WebView2** 的无边框多标签桌面浏览器（仅 Windows）。内置篡改猴（Tampermonkey）风格的用户脚本系统，并集成 **Aluka 脚本引擎**，提供面向自动化与 AI Agent Tool Calling 的浏览器控制接口。

窗口标题栏、标签栏、工具栏、书签栏、状态栏、设置页等界面由 Gio 直接绘制（纯 GPU 渲染），网页内容则由每个标签页独享的原生 WebView2 实例承载，两层通过 Win32 子窗口无缝拼合。

## 功能特性

- **无边框自绘窗口**：macOS 风格红绿灯按钮、8px 圆角裁剪 + Win11 DWM 原生圆角、拖拽移动。
- **多标签页**：每个标签页拥有独立的 WebView2 渲染实例，后台标签只隐藏不销毁——保留运行状态与滚动位置，切换零成本重载。
- **地址栏智能归一化**：自动补全协议、识别 `host:port`，非网址输入转为 DuckDuckGo 搜索。
- **新窗口拦截**：`window.open`、`target=_blank`、中键/Ctrl+点击均被拦截转为新标签页。
- **快捷书签栏**：内置 GitHub / Google / Bilibili / MDN / HackerNews。
- **网络代理设置**：支持 HTTP / SOCKS5 代理及白名单绕过，通过 Chromium 启动参数全局生效。
- **用户脚本管理器**：类篡改猴体验——元数据解析、URL 匹配、启停开关、在线编辑、本地持久化，并预置三个实用脚本（暗黑模式 / 解除复制限制 / 回到顶部悬浮球）。
- **Aluka 脚本引擎 + Agent 接口**：向 JavaScript 注入 `browser` / `userscript` / `agent` 全局对象，可用于自动化脚本或对接大模型的 Function Calling。

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
│                     圆角/图标/HWND 探测）           │
├──────────────────────────────────────────────────┤
│ internal/scripting   Aluka 引擎桥接（browser/     │
│                      userscript/agent 宿主对象）   │
│ internal/userscript  UserScript 解析/匹配/GM_*    │
│                      沙箱/JSON 存储                 │
│ internal/config      配置持久化 + 代理环境变量      │
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
├── config/                    配置持久化（%APPDATA%）、代理参数生成
├── scripting/                 Aluka 引擎封装、宿主对象注册、Agent 动作分发
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
- **Aluka 语言源码**：`go.mod` 通过 `replace` 将 `github.com/aluka-lang/aluka` 指向本地目录：

  ```
  E:/codes/go_projects/aluka_lang/aluka_lang
  ```

  克隆 aluka 仓库到上述路径（或自行修改 replace 指向你的本地检出位置），否则 `scripting` 与 `app` 包无法编译。

```bash
git clone <本仓库>
cd aluka_browser
go build -o gio-browser.exe .
./gio-browser.exe
```

### 构建脚本

仓库内置一键构建脚本，自动完成**环境检查（Go、aluka 引擎本地源码）→ 单元测试 → 构建**三步，任一环节失败即中止并给出修复指引：

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

> 若本机不存在 Aluka 本地路径，`app` / `scripting` 两个包的测试会因模块解析失败而无法执行；其余包（browser / config / userscript / ui）可独立通过。

## 配置与数据存储

所有数据位于 `%APPDATA%\gio-browser\`：

| 文件 | 内容 |
|---|---|
| `config.json` | 代理开关、代理类型（http/socks5）、服务器地址、绕过白名单 |
| `userscripts.json` | 用户脚本集合（代码原文 + 解析后的元数据 + 启用状态） |

代理配置通过环境变量 `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS` 注入为
`--proxy-server=… --proxy-bypass-list=…`，对所有 WebView2 页面全局生效。

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

## 脚本 / Agent 宿主 API

Aluka 引擎全局对象一览（可在 Eval / RunFile / 自动化脚本中使用）：

```js
// —— browser ——
browser.createTab(url, title?)
browser.switchTab(index)
browser.closeTab(index)
browser.navigate(url)
browser.goBack() / goForward() / reload()
browser.eval(jsCode)          // 在当前活跃网页内执行 JS
browser.setStatus(text)
browser.getTabs()             // [{index,id,title,url,active}]
browser.getActiveTab()
browser.getProxy() / setProxy(enabled, server?, bypass?, type?)

// —— userscript ——
userscript.list()             // [{id,name,version,...,enabled,code}]
const id = userscript.add(code, enabled?)   // 按 @name 元数据去重更新
userscript.toggle(id)
userscript.remove(id)

// —— agent（结构化动作分发，适合 Tool Calling）——
agent.action('open_url',    { url })
agent.action('create_tab',  { url, title })
agent.action('switch_tab',  { index })
agent.action('close_tab',   { index })
agent.action('get_tabs')
agent.action('page_eval',   { script })
agent.action('go_back' | 'go_forward' | 'reload')
agent.action('get_proxy' | 'set_proxy', {...})
agent.action('list_userscript' | 'add_userscript' | 'toggle_userscript' | 'delete_userscript', {...})
agent.log(...args)
```

`ScriptEngine.ExecuteAgentAction(name, params)` 同时暴露为 Go 方法，可作为宿主程序接入 LLM 的动作层。

## 当前已知限制

- 仅支持 Windows（深度依赖 Win32 与 WebView2）。
- 无会话/历史记录/收藏夹持久化，重启后回到默认主页与默认书签。
- 关闭最后一个标签页会回到主页而非退出应用（强制保留至少一个标签）。
- `@run-at` 与 `GM_registerMenuCommand` 尚未完全实现语义。
- 各标签页共用同一个 WebView2 数据目录（临时目录下的 `gio_browser_profile`）。

## License

仅供学习研究使用。
