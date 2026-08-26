// Package scripting 封装 Aluka JavaScript/TypeScript 脚本引擎，
// 为浏览器自动化控制与未来的 AI Agent 交互提供高效的原生脚本执行环境。
package scripting

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/aluka-lang/aluka/pkg/aluka"

	"gio-browser/internal/browser"
	"gio-browser/internal/config"
	"gio-browser/internal/userscript"
)

// ScriptEngine 封装 Aluka 引擎与浏览器宿主 API 桥接。
type ScriptEngine struct {
	mu sync.Mutex
	rt *aluka.Runtime
	b  *browser.Browser
}

// NewEngine 创建并初始化 Aluka 脚本引擎实例。
func NewEngine(b *browser.Browser) (*ScriptEngine, error) {
	rt, err := aluka.NewRuntime()
	if err != nil {
		return nil, fmt.Errorf("创建 Aluka 运行时失败: %w", err)
	}

	se := &ScriptEngine{
		rt: rt,
		b:  b,
	}

	// 注入 browser 与 agent 宿主对象
	if err := se.registerHostObjects(); err != nil {
		_ = rt.Close()
		return nil, fmt.Errorf("注册宿主对象失败: %w", err)
	}

	return se, nil
}

// registerHostObjects 向 JS 全局注入 browser 与 agent 对象。
func (se *ScriptEngine) registerHostObjects() error {
	browserObj := aluka.NewObject()

	// 1. browser.createTab(url, title)
	_ = browserObj.Set("createTab", aluka.NewFunction("createTab", func(args []aluka.Value) (aluka.Value, error) {
		url := ""
		title := "新标签页"
		if len(args) > 0 && !args[0].IsUndefined() && !args[0].IsNull() {
			url = args[0].String()
		}
		if len(args) > 1 && !args[1].IsUndefined() && !args[1].IsNull() {
			title = args[1].String()
		}
		if se.b != nil {
			se.b.CreateTab(url, title)
		}
		return aluka.Undefined(), nil
	}))

	// 2. browser.switchTab(index)
	_ = browserObj.Set("switchTab", aluka.NewFunction("switchTab", func(args []aluka.Value) (aluka.Value, error) {
		if len(args) > 0 {
			if idx, ok := args[0].Int(); ok && se.b != nil {
				se.b.SwitchTab(idx)
			}
		}
		return aluka.Undefined(), nil
	}))

	// 3. browser.closeTab(index)
	_ = browserObj.Set("closeTab", aluka.NewFunction("closeTab", func(args []aluka.Value) (aluka.Value, error) {
		if len(args) > 0 {
			if idx, ok := args[0].Int(); ok && se.b != nil {
				se.b.CloseTab(idx)
			}
		}
		return aluka.Undefined(), nil
	}))

	// 4. browser.navigate(url)
	_ = browserObj.Set("navigate", aluka.NewFunction("navigate", func(args []aluka.Value) (aluka.Value, error) {
		if len(args) > 0 && se.b != nil {
			url := args[0].String()
			se.b.NavigateActive(url)
		}
		return aluka.Undefined(), nil
	}))

	// 5. 导航控制：goBack, goForward, reload
	_ = browserObj.Set("goBack", aluka.NewFunction("goBack", func(args []aluka.Value) (aluka.Value, error) {
		if se.b != nil {
			se.b.GoBack()
		}
		return aluka.Undefined(), nil
	}))
	_ = browserObj.Set("goForward", aluka.NewFunction("goForward", func(args []aluka.Value) (aluka.Value, error) {
		if se.b != nil {
			se.b.GoForward()
		}
		return aluka.Undefined(), nil
	}))
	_ = browserObj.Set("reload", aluka.NewFunction("reload", func(args []aluka.Value) (aluka.Value, error) {
		if se.b != nil {
			se.b.Reload()
		}
		return aluka.Undefined(), nil
	}))

	// 6. browser.eval(script) - 在当前网页环境中执行 JS
	_ = browserObj.Set("eval", aluka.NewFunction("eval", func(args []aluka.Value) (aluka.Value, error) {
		if len(args) > 0 && se.b != nil {
			script := args[0].String()
			se.b.EvalInWebview(script)
		}
		return aluka.Undefined(), nil
	}))

	// 7. browser.setStatus(statusText)
	_ = browserObj.Set("setStatus", aluka.NewFunction("setStatus", func(args []aluka.Value) (aluka.Value, error) {
		if len(args) > 0 && se.b != nil {
			se.b.SetStatusText(args[0].String())
		}
		return aluka.Undefined(), nil
	}))

	// 8. browser.getTabs() - 返回全部标签信息
	_ = browserObj.Set("getTabs", aluka.NewFunction("getTabs", func(args []aluka.Value) (aluka.Value, error) {
		if se.b == nil {
			return aluka.NewArray(nil), nil
		}
		tabs := se.b.Tabs()
		activeIdx := se.b.ActiveIndex()

		items := make([]aluka.Value, len(tabs))
		for i, t := range tabs {
			item := aluka.NewObject()
			_ = item.Set("index", aluka.IntValue(i))
			_ = item.Set("id", aluka.Str(t.ID))
			_ = item.Set("title", aluka.Str(t.Title))
			_ = item.Set("url", aluka.Str(t.URL))
			_ = item.Set("active", aluka.Boolean(i == activeIdx))
			items[i] = item
		}
		return aluka.NewArray(items), nil
	}))

	// 9. browser.getActiveTab() - 返回当前活跃标签
	_ = browserObj.Set("getActiveTab", aluka.NewFunction("getActiveTab", func(args []aluka.Value) (aluka.Value, error) {
		if se.b == nil {
			return aluka.Null(), nil
		}
		t, ok := se.b.ActiveTab()
		if !ok {
			return aluka.Null(), nil
		}
		item := aluka.NewObject()
		_ = item.Set("index", aluka.IntValue(se.b.ActiveIndex()))
		_ = item.Set("id", aluka.Str(t.ID))
		_ = item.Set("title", aluka.Str(t.Title))
		_ = item.Set("url", aluka.Str(t.URL))
		return item, nil
	}))

	// 10. browser.getProxy() 与 browser.setProxy(enabled, server, bypass, type)
	_ = browserObj.Set("getProxy", aluka.NewFunction("getProxy", func(args []aluka.Value) (aluka.Value, error) {
		cfg := config.Current()
		obj := aluka.NewObject()
		_ = obj.Set("enabled", aluka.Boolean(cfg.ProxyEnabled))
		_ = obj.Set("type", aluka.Str(cfg.ProxyType))
		_ = obj.Set("server", aluka.Str(cfg.ProxyServer))
		_ = obj.Set("bypass", aluka.Str(cfg.ProxyBypass))
		return obj, nil
	}))

	_ = browserObj.Set("setProxy", aluka.NewFunction("setProxy", func(args []aluka.Value) (aluka.Value, error) {
		cfg := config.Current()
		if len(args) > 0 {
			if b, ok := args[0].Bool(); ok {
				cfg.ProxyEnabled = b
			}
		}
		if len(args) > 1 && !args[1].IsUndefined() && !args[1].IsNull() {
			cfg.ProxyServer = strings.TrimSpace(args[1].String())
		}
		if len(args) > 2 && !args[2].IsUndefined() && !args[2].IsNull() {
			cfg.ProxyBypass = strings.TrimSpace(args[2].String())
		}
		if len(args) > 3 && !args[3].IsUndefined() && !args[3].IsNull() {
			cfg.ProxyType = strings.TrimSpace(args[3].String())
		}
		if err := config.Save(cfg); err != nil {
			return aluka.Boolean(false), err
		}
		return aluka.Boolean(true), nil
	}))

	// 注册到全局 globalThis.browser
	_ = se.rt.Global().Set("browser", browserObj)

	// 11. 注册 userscript (篡改猴) 对象
	usObj := aluka.NewObject()
	_ = usObj.Set("list", aluka.NewFunction("list", func(args []aluka.Value) (aluka.Value, error) {
		list := userscript.GetGlobalManager().List()
		items := make([]aluka.Value, len(list))
		for i, s := range list {
			item := aluka.NewObject()
			_ = item.Set("id", aluka.Str(s.ID))
			_ = item.Set("name", aluka.Str(s.Meta.Name))
			_ = item.Set("version", aluka.Str(s.Meta.Version))
			_ = item.Set("description", aluka.Str(s.Meta.Description))
			_ = item.Set("author", aluka.Str(s.Meta.Author))
			_ = item.Set("enabled", aluka.Boolean(s.Enabled))
			_ = item.Set("code", aluka.Str(s.Code))
			items[i] = item
		}
		return aluka.NewArray(items), nil
	}))

	_ = usObj.Set("add", aluka.NewFunction("add", func(args []aluka.Value) (aluka.Value, error) {
		if len(args) == 0 {
			return aluka.Undefined(), fmt.Errorf("缺少 code 参数")
		}
		code := args[0].String()
		enabled := true
		if len(args) > 1 {
			if b, ok := args[1].Bool(); ok {
				enabled = b
			}
		}
		s := userscript.NewScript(code, enabled)
		if err := userscript.GetGlobalManager().AddOrUpdate(s); err != nil {
			return aluka.Undefined(), err
		}
		return aluka.Str(s.ID), nil
	}))

	_ = usObj.Set("toggle", aluka.NewFunction("toggle", func(args []aluka.Value) (aluka.Value, error) {
		if len(args) == 0 {
			return aluka.Undefined(), fmt.Errorf("缺少 id 参数")
		}
		id := args[0].String()
		enabled, err := userscript.GetGlobalManager().Toggle(id)
		if err != nil {
			return aluka.Undefined(), err
		}
		return aluka.Boolean(enabled), nil
	}))

	_ = usObj.Set("remove", aluka.NewFunction("remove", func(args []aluka.Value) (aluka.Value, error) {
		if len(args) == 0 {
			return aluka.Undefined(), fmt.Errorf("缺少 id 参数")
		}
		id := args[0].String()
		if err := userscript.GetGlobalManager().Delete(id); err != nil {
			return aluka.Boolean(false), err
		}
		return aluka.Boolean(true), nil
	}))

	_ = se.rt.Global().Set("userscript", usObj)

	// 注册 agent 辅助对象
	agentObj := aluka.NewObject()
	_ = agentObj.Set("action", aluka.NewFunction("action", func(args []aluka.Value) (aluka.Value, error) {
		if len(args) == 0 {
			return aluka.Undefined(), fmt.Errorf("action 缺少名称参数")
		}
		actionName := args[0].String()
		params := make(map[string]any)
		if len(args) > 1 {
			if pObj, ok := args[1].AsObject(); ok {
				for _, k := range pObj.Keys() {
					val, _ := pObj.Get(k)
					params[k] = val.String()
				}
			}
		}
		res, err := se.ExecuteAgentAction(actionName, params)
		if err != nil {
			return aluka.Undefined(), err
		}
		return aluka.Str(fmt.Sprintf("%v", res)), nil
	}))

	_ = agentObj.Set("log", aluka.NewFunction("log", func(args []aluka.Value) (aluka.Value, error) {
		var parts []string
		for _, a := range args {
			parts = append(parts, a.String())
		}
		log.Printf("[AI Agent] %s", strings.Join(parts, " "))
		return aluka.Undefined(), nil
	}))

	_ = se.rt.Global().Set("agent", agentObj)

	return nil
}

// Eval 执行一段 JavaScript/TypeScript 代码字符串，并排空微任务。
func (se *ScriptEngine) Eval(code, filename string) (aluka.Value, error) {
	se.mu.Lock()
	defer se.mu.Unlock()

	return se.rt.Eval(code, filename)
}

// RunFile 执行指定的脚本文件（支持 ESM/CJS/TS）。
func (se *ScriptEngine) RunFile(path string) error {
	se.mu.Lock()
	defer se.mu.Unlock()

	return se.rt.RunFile(path)
}

// ExecuteAgentAction 提供面向 AI Agent / Tool Calling 的结构化动作分发器。
func (se *ScriptEngine) ExecuteAgentAction(action string, params map[string]any) (any, error) {
	if se.b == nil {
		return nil, fmt.Errorf("浏览器引擎未就绪")
	}

	getString := func(key string) string {
		if v, ok := params[key]; ok {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}
	getInt := func(key string, def int) int {
		if v, ok := params[key]; ok {
			var n int
			if _, err := fmt.Sscanf(fmt.Sprintf("%v", v), "%d", &n); err == nil {
				return n
			}
		}
		return def
	}

	switch strings.ToLower(strings.TrimSpace(action)) {
	case "open_url", "navigate":
		url := getString("url")
		if url == "" {
			return nil, fmt.Errorf("缺少 url 参数")
		}
		se.b.NavigateActive(url)
		return fmt.Sprintf("已导航到: %s", url), nil

	case "create_tab", "new_tab":
		url := getString("url")
		title := getString("title")
		if title == "" {
			title = "新标签页"
		}
		se.b.CreateTab(url, title)
		return fmt.Sprintf("已创建新标签页: %s (%s)", title, url), nil

	case "switch_tab":
		idx := getInt("index", -1)
		if idx < 0 || idx >= se.b.TabCount() {
			return nil, fmt.Errorf("无效的标签页下标: %d", idx)
		}
		se.b.SwitchTab(idx)
		return fmt.Sprintf("已切换到标签页 %d", idx), nil

	case "close_tab":
		idx := getInt("index", -1)
		if idx < 0 || idx >= se.b.TabCount() {
			return nil, fmt.Errorf("无效的标签页下标: %d", idx)
		}
		se.b.CloseTab(idx)
		return fmt.Sprintf("已关闭标签页 %d", idx), nil

	case "get_tabs", "list_tabs":
		tabs := se.b.Tabs()
		active := se.b.ActiveIndex()
		var res []map[string]any
		for i, t := range tabs {
			res = append(res, map[string]any{
				"index":  i,
				"id":     t.ID,
				"title":  t.Title,
				"url":    t.URL,
				"active": i == active,
			})
		}
		return res, nil

	case "page_eval", "eval_js":
		script := getString("script")
		if script == "" {
			return nil, fmt.Errorf("缺少 script 参数")
		}
		se.b.EvalInWebview(script)
		return "已在当前网页执行 JavaScript", nil

	case "go_back":
		se.b.GoBack()
		return "已执行后退", nil

	case "go_forward":
		se.b.GoForward()
		return "已执行前进", nil

	case "reload":
		se.b.Reload()
		return "已执行刷新", nil

	case "get_proxy":
		cfg := config.Current()
		return map[string]any{
			"enabled": cfg.ProxyEnabled,
			"type":    cfg.ProxyType,
			"server":  cfg.ProxyServer,
			"bypass":  cfg.ProxyBypass,
		}, nil

	case "set_proxy":
		cfg := config.Current()
		if enVal, ok := params["enabled"]; ok {
			cfg.ProxyEnabled = fmt.Sprintf("%v", enVal) == "true"
		}
		if s := getString("server"); s != "" {
			cfg.ProxyServer = s
		}
		if b := getString("bypass"); b != "" {
			cfg.ProxyBypass = b
		}
		if t := getString("type"); t != "" {
			cfg.ProxyType = t
		}
		if err := config.Save(cfg); err != nil {
			return nil, fmt.Errorf("保存代理配置失败: %w", err)
		}
		return "代理配置已更新并生效", nil

	case "list_userscripts":
		list := userscript.GetGlobalManager().List()
		var res []map[string]any
		for _, s := range list {
			res = append(res, map[string]any{
				"id":          s.ID,
				"name":        s.Meta.Name,
				"version":     s.Meta.Version,
				"description": s.Meta.Description,
				"author":      s.Meta.Author,
				"enabled":     s.Enabled,
				"match":       s.Meta.Match,
			})
		}
		return res, nil

	case "add_userscript":
		code := getString("code")
		if code == "" {
			return nil, fmt.Errorf("缺少 code 参数")
		}
		enabled := true
		if enVal, ok := params["enabled"]; ok {
			enabled = fmt.Sprintf("%v", enVal) == "true"
		}
		s := userscript.NewScript(code, enabled)
		if err := userscript.GetGlobalManager().AddOrUpdate(s); err != nil {
			return nil, fmt.Errorf("添加用户脚本失败: %w", err)
		}
		return fmt.Sprintf("已成功添加用户脚本: %s (ID: %s)", s.Meta.Name, s.ID), nil

	case "toggle_userscript":
		id := getString("id")
		if id == "" {
			return nil, fmt.Errorf("缺少 id 参数")
		}
		enabled, err := userscript.GetGlobalManager().Toggle(id)
		if err != nil {
			return nil, err
		}
		stateStr := "已启用"
		if !enabled {
			stateStr = "已禁用"
		}
		return fmt.Sprintf("脚本 %s %s", id, stateStr), nil

	case "delete_userscript":
		id := getString("id")
		if id == "" {
			return nil, fmt.Errorf("缺少 id 参数")
		}
		if err := userscript.GetGlobalManager().Delete(id); err != nil {
			return nil, err
		}
		return fmt.Sprintf("已删除脚本 %s", id), nil

	default:
		return nil, fmt.Errorf("未知 Agent Action: %s", action)
	}
}

// Close 释放脚本引擎资源。
func (se *ScriptEngine) Close() error {
	se.mu.Lock()
	defer se.mu.Unlock()

	if se.rt != nil {
		return se.rt.Close()
	}
	return nil
}
