package browser

import (
	"fmt"
	"testing"
)

// stubEngine 记录模型对引擎的全部调用，用于断言交互序列。
type stubEngine struct {
	created   []string
	switched  []string
	closed    []string
	navigated []string
	others    int
}

func (s *stubEngine) CreateTab(tabID, url string) { s.created = append(s.created, tabID+"\x00"+url) }
func (s *stubEngine) SwitchTab(tabID string)      { s.switched = append(s.switched, tabID) }
func (s *stubEngine) CloseTab(tabID string)       { s.closed = append(s.closed, tabID) }
func (s *stubEngine) Navigate(tabID, url string)  { s.navigated = append(s.navigated, url) }
func (s *stubEngine) GoBack()                     { s.others++ }
func (s *stubEngine) GoForward()                  { s.others++ }
func (s *stubEngine) Reload()                     { s.others++ }
func (s *stubEngine) FocusContent()               { s.others++ }

func TestNormalizeInputURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"  ", ""},
		{"https://github.com", "https://github.com"},
		{"http://example.com/x?y=1", "http://example.com/x?y=1"},
		{"github.com", "https://github.com"},
		{"localhost:8080", "https://localhost:8080"},
		{" Gio Browser ", "https://duckduckgo.com/?q=Gio+Browser"},
		{"1.1.1.1", "https://1.1.1.1"},
	}
	for _, c := range cases {
		if got := NormalizeInputURL(c.in); got != c.want {
			t.Errorf("NormalizeInputURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCreateAndSwitchTab(t *testing.T) {
	e := &stubEngine{}
	b := New(e)

	if n := b.TabCount(); n != 1 {
		t.Fatalf("初始标签数 = %d, want 1", n)
	}

	b.CreateTab("https://example.org", "Example")
	if n := b.TabCount(); n != 2 {
		t.Fatalf("新建后标签数 = %d, want 2", n)
	}
	if len(e.created) != 1 {
		t.Fatalf("引擎应收到 1 次 CreateTab, got %d", len(e.created))
	}

	b.SwitchTab(0)
	if idx := b.ActiveIndex(); idx != 0 {
		t.Fatalf("切换后活跃下标 = %d, want 0", idx)
	}

	// 重复点击已激活标签不应触发引擎
	b.SwitchTab(0)
	if len(e.switched) != 1 {
		t.Fatalf("重复激活不应触发 SwitchTab, 收到 %d 次", len(e.switched))
	}

	active, _ := b.ActiveTab()
	if active.ID != DefaultTabID {
		t.Errorf("活跃标签 ID = %q, want %q", active.ID, DefaultTabID)
	}
}

func TestCloseTab(t *testing.T) {
	e := &stubEngine{}
	b := New(e)
	b.CreateTab("https://a.com", "A")
	id := ""
	for _, tb := range b.Tabs() {
		if tb.Title == "A" {
			id = tb.ID
		}
	}

	b.CloseTab(0)
	if got := b.TabCount(); got != 1 {
		t.Fatalf("关闭一个后标签数 = %d, want 1", got)
	}
	active, _ := b.ActiveTab()
	if active.ID != id {
		t.Errorf("关闭后活跃标签应为剩余的 %q, got %q", id, active.ID)
	}

	// 关闭最后一个标签：应导航回主页而非销毁标签
	b.CloseTab(0)
	if b.TabCount() != 1 {
		t.Fatalf("最后一个标签不应被移除, 标签数=%d", b.TabCount())
	}
	last := e.navigated[len(e.navigated)-1]
	if last != HomeURL {
		t.Errorf("最后标签关闭应导航主页, got %q", last)
	}
}

func TestUpdateTabState(t *testing.T) {
	b := New(nil)
	b.UpdateTabState(DefaultTabID, "https://new.io", "New")
	tab, _ := b.ActiveTab()
	if tab.URL != "https://new.io" || tab.Title != "New" {
		t.Errorf("状态未同步: %+v", tab)
	}

	// 未知名应安全忽略且不 panic
	b.UpdateTabState(fmt.Sprintf("tab-%d-unknown", 12345), "https://x.io", "X")
}
