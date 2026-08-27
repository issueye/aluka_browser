package scripting

import (
	"strings"
	"testing"

	"gio-browser/internal/browser"
)

// mockEngine 实现 browser.Engine 接口，记录引擎调用供断言（无需 WebView2）。
type mockEngine struct {
	created   []string
	switched  []string
	closed    []string
	navigated []string
	evals     []string
}

func (m *mockEngine) CreateTab(tabID, url string) { m.created = append(m.created, tabID+":"+url) }
func (m *mockEngine) CreateExtensionTab(tabID, url, host, dir string) { m.created = append(m.created, tabID+":"+url) }
func (m *mockEngine) SwitchTab(tabID string)      { m.switched = append(m.switched, tabID) }
func (m *mockEngine) CloseTab(tabID string)       { m.closed = append(m.closed, tabID) }
func (m *mockEngine) Navigate(tabID, url string)  { m.navigated = append(m.navigated, url) }
func (m *mockEngine) GoBack()                     {}
func (m *mockEngine) GoForward()                  {}
func (m *mockEngine) Reload()                     {}
func (m *mockEngine) FocusContent()               {}
func (m *mockEngine) SetVisible(visible bool)     {}
func (m *mockEngine) Eval(script string)          { m.evals = append(m.evals, script) }

func newTestBrowser() (*browser.Browser, *mockEngine) {
	m := &mockEngine{}
	return browser.New(m), m
}

func TestExecuteAgentAction_Tabs(t *testing.T) {

	cases := []struct {
		name    string
		action  string
		params  map[string]any
		wantErr string // 非空表示期望错误信息包含该子串
		check   func(t *testing.T, b *browser.Browser)
	}{
		{
			name:   "create_tab 新建并激活",
			action: "create_tab",
			params: map[string]any{"url": "https://example.com", "title": "示例"},
			check: func(t *testing.T, b *browser.Browser) {
				if b.TabCount() != 2 {
					t.Fatalf("标签页数量 = %d, want 2", b.TabCount())
				}
				if b.ActiveIndex() != 1 {
					t.Fatalf("活跃下标 = %d, want 1", b.ActiveIndex())
				}
			},
		},
		{
			name:   "create_tab 缺省参数走默认值",
			action: "new_tab",
			params: nil,
			check: func(t *testing.T, b *browser.Browser) {
				tab, _ := b.ActiveTab()
				if tab.Title != "新标签页" || tab.URL != browser.HomeURL {
					t.Fatalf("默认标签 = %+v", tab)
				}
			},
		},
		{
			name:   "switch_tab 非法下标报错",
			action: "switch_tab",
			params: map[string]any{"index": 99},
			wantErr: "无效的标签页下标",
		},
		{
			name:   "close_tab 最后一个标签回主页",
			action: "close_tab",
			params: map[string]any{"index": 0},
			check: func(t *testing.T, b *browser.Browser) {
				if b.TabCount() != 1 {
					t.Fatalf("关闭后应仍保留一个标签, got %d", b.TabCount())
				}
				tab, _ := b.ActiveTab()
				if tab.URL != browser.HomeURL {
					t.Fatalf("最后标签应回到主页 %s, got %s", browser.HomeURL, tab.URL)
				}
			},
		},
		{
			name:   "未知动作报错",
			action: "nonsense",
			params: nil,
			wantErr: "未知 Agent Action",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 每个 case 用全新浏览器实例，避免相互影响。
			tb, _ := newTestBrowser()
			_, err := ExecuteAgentAction(tb, tc.action, tc.params)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("错误 = %v, want 包含 %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("意外错误: %v", err)
			}
			tc.check(t, tb)
		})
	}
}

func TestExecuteAgentAction_NavigationAndEval(t *testing.T) {
	b, m := newTestBrowser()

	if _, err := ExecuteAgentAction(b, "open_url", map[string]any{"url": ""}); err == nil {
		t.Fatal("open_url 空 url 应报错")
	}

	res, err := ExecuteAgentAction(b, "navigate", map[string]any{"url": "https://golang.org"})
	if err != nil {
		t.Fatalf("navigate 失败: %v", err)
	}
	if s, _ := res.(string); !strings.Contains(s, "https://golang.org") {
		t.Fatalf("返回信息异常: %v", res)
	}
	tab, _ := b.ActiveTab()
	if tab.URL != "https://golang.org" {
		t.Fatalf("模型 URL 未同步: %s", tab.URL)
	}
	if len(m.navigated) == 0 || m.navigated[0] != "https://golang.org" {
		t.Fatalf("引擎未收到导航指令: %v", m.navigated)
	}

	if _, err := ExecuteAgentAction(b, "page_eval", nil); err == nil {
		t.Fatal("page_eval 缺 script 应报错")
	}
	if _, err := ExecuteAgentAction(b, "eval_js", map[string]any{"script": "console.log(1)"}); err != nil {
		t.Fatalf("page_eval 失败: %v", err)
	}
	if len(m.evals) == 0 {
		t.Fatal("引擎未收到 Eval 调用")
	}
}

// TestExecuteAgentAction_ProxyRemoved 验证代理动作已随代理功能移除。
func TestExecuteAgentAction_ProxyRemoved(t *testing.T) {
	b, _ := newTestBrowser()
	for _, act := range []string{"get_proxy", "set_proxy"} {
		if _, err := ExecuteAgentAction(b, act, nil); err == nil {
			t.Fatalf("代理动作 %s 应已移除", act)
		}
	}
}

func TestExecuteAgentAction_UserscriptCRUD(t *testing.T) {
	b, _ := newTestBrowser()

	idRaw, err := ExecuteAgentAction(b, "add_userscript", map[string]any{
		"code": "// ==UserScript==\n// @name 测试脚本\n// @match *://example.com/*\n// ==/UserScript==\nconsole.log('hi');\n",
	})
	if err != nil {
		t.Fatalf("add_userscript 失败: %v", err)
	}
	id := ""
	if s, ok := idRaw.(string); !ok || !strings.Contains(s, "测试脚本") {
		t.Fatalf("add_userscript 返回异常: %v", idRaw)
	} else {
		id = s
	}

	listRaw, err := ExecuteAgentAction(b, "list_userscripts", nil)
	if err != nil {
		t.Fatalf("list_userscripts 失败: %v", err)
	}
	found := false
	for _, item := range listRaw.([]map[string]any) {
		if item["name"] == "测试脚本" {
			found = true
			id = item["id"].(string)
		}
	}
	if !found {
		t.Fatal("列表中未找到刚添加的脚本")
	}

	out, err := ExecuteAgentAction(b, "toggle_userscript", map[string]any{"id": id})
	if err != nil {
		t.Fatalf("toggle_userscript 失败: %v", err)
	}
	if !strings.Contains(out.(string), "已禁用") {
		t.Fatalf("toggle 返回异常: %v", out)
	}

	if _, err := ExecuteAgentAction(b, "delete_userscript", map[string]any{"id": id}); err != nil {
		t.Fatalf("delete_userscript 失败: %v", err)
	}
	if _, err := ExecuteAgentAction(b, "toggle_userscript", map[string]any{"id": id}); err == nil {
		t.Fatal("删除后再 toggle 应报错")
	}
}
