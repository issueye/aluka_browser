package scripting

import (
	"strings"
	"testing"

	"gio-browser/internal/browser"
	"gio-browser/internal/config"
)

type mockEngine struct {
	created   []string
	switched  []string
	closed    []string
	navigated []string
	evals     []string
}

func (m *mockEngine) CreateTab(tabID, url string) { m.created = append(m.created, tabID+":"+url) }
func (m *mockEngine) SwitchTab(tabID string)      { m.switched = append(m.switched, tabID) }
func (m *mockEngine) CloseTab(tabID string)       { m.closed = append(m.closed, tabID) }
func (m *mockEngine) Navigate(tabID, url string)  { m.navigated = append(m.navigated, url) }
func (m *mockEngine) GoBack()                     {}
func (m *mockEngine) GoForward()                  {}
func (m *mockEngine) Reload()                     {}
func (m *mockEngine) FocusContent()               {}
func (m *mockEngine) SetVisible(visible bool)     {}
func (m *mockEngine) Eval(script string)          { m.evals = append(m.evals, script) }

func TestScriptEngine_EvalMath(t *testing.T) {
	se, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("创建引擎失败: %v", err)
	}
	defer se.Close()

	val, err := se.Eval("1 + 2 * 3", "")
	if err != nil {
		t.Fatalf("Eval 失败: %v", err)
	}
	if n, ok := val.Int(); !ok || n != 7 {
		t.Errorf("期望返回 7, 实际得到: %v", val.String())
	}
}

func TestScriptEngine_BrowserTabControl(t *testing.T) {
	mock := &mockEngine{}
	b := browser.New(mock)

	se, err := NewEngine(b)
	if err != nil {
		t.Fatalf("创建引擎失败: %v", err)
	}
	defer se.Close()

	// 1. 测试 JS 脚本创建新标签页
	_, err = se.Eval(`browser.createTab("https://aluka.dev", "Aluka Engine");`, "")
	if err != nil {
		t.Fatalf("执行 createTab 失败: %v", err)
	}
	if b.TabCount() != 2 {
		t.Fatalf("创建后标签数应为 2, 实际为 %d", b.TabCount())
	}

	// 2. 测试 JS 脚本获取标签并切换
	script := `
		const tabs = browser.getTabs();
		if (tabs.length !== 2) {
			throw new Error("tabs 长度错误: " + tabs.length);
		}
		browser.switchTab(0);
	`
	if _, err := se.Eval(script, ""); err != nil {
		t.Fatalf("执行 switchTab 脚本失败: %v", err)
	}
	if b.ActiveIndex() != 0 {
		t.Errorf("活跃标签应切换回 0, 实际为 %d", b.ActiveIndex())
	}

	// 3. 测试 JS 脚本导航
	if _, err := se.Eval(`browser.navigate("https://golang.org");`, ""); err != nil {
		t.Fatalf("执行 navigate 失败: %v", err)
	}
	tab, _ := b.ActiveTab()
	if tab.URL != "https://golang.org" {
		t.Errorf("活跃标签 URL 期望为 https://golang.org, 实际为: %s", tab.URL)
	}

	// 4. 测试在 Webview 中执行 JS
	if _, err := se.Eval(`browser.eval("console.log('hello from aluka')");`, ""); err != nil {
		t.Fatalf("执行 browser.eval 失败: %v", err)
	}
	if len(mock.evals) != 1 || !strings.Contains(mock.evals[0], "hello from aluka") {
		t.Errorf("底层引擎未收到 eval, got: %v", mock.evals)
	}
}

func TestScriptEngine_ProxyControl(t *testing.T) {
	se, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("创建引擎失败: %v", err)
	}
	defer se.Close()

	script := `
		browser.setProxy(true, "127.0.0.1:8888", "localhost;127.0.0.1", "socks5");
		const p = browser.getProxy();
		if (!p.enabled || p.server !== "127.0.0.1:8888" || p.type !== "socks5") {
			throw new Error("proxy 配置未按预期更新: " + JSON.stringify(p));
		}
	`
	if _, err := se.Eval(script, ""); err != nil {
		t.Fatalf("执行代理设置脚本失败: %v", err)
	}

	cur := config.Current()
	if !cur.ProxyEnabled || cur.ProxyServer != "127.0.0.1:8888" || cur.ProxyType != "socks5" {
		t.Errorf("本地配置未同步: %+v", cur)
	}
}

func TestScriptEngine_AgentAction(t *testing.T) {
	mock := &mockEngine{}
	b := browser.New(mock)

	se, err := NewEngine(b)
	if err != nil {
		t.Fatalf("创建引擎失败: %v", err)
	}
	defer se.Close()

	// 1. Agent 创建标签动作
	res, err := se.ExecuteAgentAction("create_tab", map[string]any{
		"url":   "https://github.com/aluka-lang/aluka",
		"title": "Aluka Lang",
	})
	if err != nil {
		t.Fatalf("Agent Action create_tab 失败: %v", err)
	}
	if !strings.Contains(res.(string), "已创建新标签页") {
		t.Errorf("意外的返回信息: %v", res)
	}
	if b.TabCount() != 2 {
		t.Errorf("标签数应为 2, 实际为 %d", b.TabCount())
	}

	// 2. Agent 获取标签列表
	tabsRes, err := se.ExecuteAgentAction("get_tabs", nil)
	if err != nil {
		t.Fatalf("Agent Action get_tabs 失败: %v", err)
	}
	tabsList, ok := tabsRes.([]map[string]any)
	if !ok || len(tabsList) != 2 {
		t.Fatalf("期望得到 2 个标签, 实际得到: %v", tabsRes)
	}

	// 3. Agent 设置代理动作
	_, err = se.ExecuteAgentAction("set_proxy", map[string]any{
		"enabled": "true",
		"server":  "127.0.0.1:9090",
		"type":    "http",
	})
	if err != nil {
		t.Fatalf("Agent Action set_proxy 失败: %v", err)
	}
	cfg := config.Current()
	if !cfg.ProxyEnabled || cfg.ProxyServer != "127.0.0.1:9090" {
		t.Errorf("Agent 配置代理未生效: %+v", cfg)
	}
}

func TestScriptEngine_UserscriptHost(t *testing.T) {
	se, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("创建引擎失败: %v", err)
	}
	defer se.Close()

	script := `
		const id = userscript.add("// ==UserScript==\n// @name 动态助手\n// @match *://*/*\n// ==/UserScript==\nconsole.log(123);", true);
		if (!id || typeof id !== "string") {
			throw new Error("add 返回非法 ID: " + id);
		}
		const list = userscript.list();
		if (!list || list.length === 0) {
			throw new Error("list 为空");
		}
		const toggled = userscript.toggle(id);
		if (toggled !== false) {
			throw new Error("toggle 期望返回 false, got: " + toggled);
		}
		userscript.remove(id);
	`
	if _, err := se.Eval(script, ""); err != nil {
		t.Fatalf("userscript 宿主测试失败: %v", err)
	}
}
