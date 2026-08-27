// Package scripting 提供 AI Agent / Tool Calling 的浏览器控制动作分发层。
//
// 本包历史上曾封装 Aluka 脚本引擎（向 JS 注入 browser/userscript/agent 宿主
// 对象并提供 Eval/RunFile）。由于上游将实现收敛进 internal 包且未提供稳定的
// 公共嵌入 API，运行时依赖已被移除，仅保留纯 Go 实现的结构化动作分发：
//
//	res, err := scripting.ExecuteAgentAction(b, "create_tab", map[string]any{
//		"url": "https://example.com", "title": "示例",
//	})
//
// 未来接入 LLM 编排时，直接为本分发器挂上本地 IPC/HTTP 入口即可，
// 无需重新引入脚本语言运行时。
package scripting

import (
	"fmt"
	"strings"

	"gio-browser/internal/browser"
	"gio-browser/internal/config"
	"gio-browser/internal/userscript"
)

// ExecuteAgentAction 分发一个结构化 Agent 动作到浏览器领域模型。
// 动作名大小写不敏感；params 中的值统一按字符串取用（fmt.Sprintf 语义）。
func ExecuteAgentAction(b *browser.Browser, action string, params map[string]any) (any, error) {
	if b == nil {
		return nil, fmt.Errorf("浏览器模型未就绪")
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
	getBool := func(key string) bool {
		if v, ok := params[key]; ok {
			return fmt.Sprintf("%v", v) == "true"
		}
		return false
	}

	switch strings.ToLower(strings.TrimSpace(action)) {
	case "open_url", "navigate":
		url := getString("url")
		if url == "" {
			return nil, fmt.Errorf("缺少 url 参数")
		}
		b.NavigateActive(url)
		return fmt.Sprintf("已导航到: %s", url), nil

	case "create_tab", "new_tab":
		url := getString("url")
		title := getString("title")
		if title == "" {
			title = "新标签页"
		}
		b.CreateTab(url, title)
		return fmt.Sprintf("已创建新标签页: %s (%s)", title, url), nil

	case "switch_tab":
		idx := getInt("index", -1)
		if idx < 0 || idx >= b.TabCount() {
			return nil, fmt.Errorf("无效的标签页下标: %d", idx)
		}
		b.SwitchTab(idx)
		return fmt.Sprintf("已切换到标签页 %d", idx), nil

	case "close_tab":
		idx := getInt("index", -1)
		if idx < 0 || idx >= b.TabCount() {
			return nil, fmt.Errorf("无效的标签页下标: %d", idx)
		}
		b.CloseTab(idx)
		return fmt.Sprintf("已关闭标签页 %d", idx), nil

	case "get_tabs", "list_tabs":
		tabs := b.Tabs()
		active := b.ActiveIndex()
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
		b.EvalInWebview(script)
		return "已在当前网页执行 JavaScript", nil

	case "go_back":
		b.GoBack()
		return "已执行后退", nil

	case "go_forward":
		b.GoForward()
		return "已执行前进", nil

	case "reload":
		b.Reload()
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
		if _, ok := params["enabled"]; ok {
			cfg.ProxyEnabled = getBool("enabled")
		}
		if s := getString("server"); s != "" {
			cfg.ProxyServer = s
		}
		if bp := getString("bypass"); bp != "" {
			cfg.ProxyBypass = bp
		}
		if pt := getString("type"); pt != "" {
			cfg.ProxyType = pt
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
		s := userscript.NewScript(code, true)
		if _, ok := params["enabled"]; ok {
			s.Enabled = getBool("enabled")
		}
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
