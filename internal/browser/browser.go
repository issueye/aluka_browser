// Package browser 实现浏览器的领域模型：标签页集合、活跃状态与导航动作。
//
// 该层不依赖任何 GUI 框架；对页面引擎的依赖通过 Engine 接口表达，
// 由 webview.Manager 在应用装配时注入。所有方法可在任意 goroutine 调用
// （WebView2 回调运行在独立 STA 线程），内部以互斥锁保护。
package browser

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// DefaultTabID 初始标签页的固定 ID。
const DefaultTabID = "tab-default-1"

// SettingsURL 设置中心标签页的固定地址（不承载 WebView，由 Gio 绘制）。
const SettingsURL = "gio://settings"

// SettingsTitle 设置中心标签页标题。
const SettingsTitle = "设置"

// IsSettingsURL 判断地址是否为设置中心标签页。
func IsSettingsURL(url string) bool {
	return strings.HasPrefix(url, SettingsURL)
}

// Tab 标签页的纯数据模型（不包含任何控件状态）。
type Tab struct {
	ID    string
	Title string
	URL   string
}

// Bookmark 快捷访问条目（快捷访问栏与设置中心共用）。
type Bookmark struct {
	Name string
	URL  string
}

// Engine 浏览器模型依赖的页面引擎抽象；webview.Manager 是其默认实现。
type Engine interface {
	CreateTab(tabID, url string)
	CreateExtensionTab(tabID, url, host, dir string)
	SwitchTab(tabID string)
	CloseTab(tabID string)
	Navigate(tabID, url string)
	GoBack()
	GoForward()
	Reload()
	FocusContent()
	SetVisible(visible bool)
}

// Browser 浏览器状态机。
type Browser struct {
	mu           sync.Mutex
	engine       Engine
	tabs         []*Tab
	active       int
	status       string
	loading      bool
	bookmark     []Bookmark
	homePage     string
}

// New 创建浏览器并置入一个默认标签页。engine 允许为 nil（用于测试）。
// 主页与快捷访问初始为空，由装配层注入持久化值（见 SetHomePage/AddQuickLink）。
func New(engine Engine) *Browser {
	return &Browser{
		engine:   engine,
		tabs:     []*Tab{{ID: DefaultTabID, Title: "主页", URL: HomeURL}},
		status:   "就绪",
		homePage: HomeURL,
	}
}

// ---- 主页与快捷访问 ----

// HomePage 返回当前生效的主页地址。
func (b *Browser) HomePage() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.homePage == "" {
		return HomeURL
	}
	return b.homePage
}

// SetHomePage 设置主页地址（仅内存态，持久化由调用方写配置）。
func (b *Browser) SetHomePage(home string) {
	home = strings.TrimSpace(home)
	b.mu.Lock()
	defer b.mu.Unlock()
	if home == "" {
		home = HomeURL
	}
	b.homePage = home
}

// AddQuickLink 追加快捷访问条目（仅内存态）；名称或地址为空时忽略。
func (b *Browser) AddQuickLink(name, url string) (Bookmark, bool) {
	name, url = strings.TrimSpace(name), strings.TrimSpace(url)
	b.mu.Lock()
	defer b.mu.Unlock()
	if name == "" || url == "" {
		return Bookmark{}, false
	}
	bm := Bookmark{Name: name, URL: url}
	b.bookmark = append(b.bookmark, bm)
	return bm, true
}

// RemoveQuickLink 删除指定下标的快捷访问条目；下标非法返回 false。
func (b *Browser) RemoveQuickLink(i int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if i < 0 || i >= len(b.bookmark) {
		return false
	}
	b.bookmark = append(b.bookmark[:i], b.bookmark[i+1:]...)
	return true
}

// ---- 只读访问 ----

// TabCount 返回标签页数量（永不为 0，至少保留一个）。
func (b *Browser) TabCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.tabs)
}

// TabAt 返回第 i 个标签页的数据副本。
func (b *Browser) TabAt(i int) (Tab, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if i < 0 || i >= len(b.tabs) {
		return Tab{}, false
	}
	return *b.tabs[i], true
}

// Tabs 返回所有标签页的数据副本快照。
func (b *Browser) Tabs() []Tab {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Tab, len(b.tabs))
	for i, t := range b.tabs {
		out[i] = *t
	}
	return out
}

// ActiveIndex 返回当前活跃标签页下标。
func (b *Browser) ActiveIndex() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.active
}

// ActiveTab 返回当前活跃标签页副本。
func (b *Browser) ActiveTab() (Tab, bool) {
	return b.TabAt(b.ActiveIndex())
}

// StatusText 底部状态栏文本。
func (b *Browser) StatusText() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.status
}

// Loading 当前活跃页是否处于加载中。
func (b *Browser) Loading() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.loading
}

// Bookmarks 返回快捷书签快照。
func (b *Browser) Bookmarks() []Bookmark {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]Bookmark(nil), b.bookmark...)
}

// ---- 动作（UI 调用）----

// CreateTab 新建标签页并立即激活。url 为空时打开主页。
func (b *Browser) CreateTab(url, title string) {
	if url == "" {
		url = b.HomePage()
	}
	if title == "" {
		title = "新标签页"
	}
	id := fmt.Sprintf("tab-%d", time.Now().UnixNano())

	b.mu.Lock()
	b.tabs = append(b.tabs, &Tab{ID: id, Title: title, URL: url})
	b.active = len(b.tabs) - 1
	engine := b.engine
	b.mu.Unlock()

	if engine != nil {
		engine.CreateTab(id, url)
	}
}

// CreateExtensionTab 新建扩展弹窗标签（独立 http 方案，避免 file:// 受限）。
func (b *Browser) CreateExtensionTab(url, title, host, dir string) {
	if url == "" {
		url = b.HomePage()
	}
	if title == "" {
		title = "新标签页"
	}
	id := fmt.Sprintf("tab-%d", time.Now().UnixNano())

	b.mu.Lock()
	b.tabs = append(b.tabs, &Tab{ID: id, Title: title, URL: url})
	b.active = len(b.tabs) - 1
	engine := b.engine
	b.mu.Unlock()

	if engine != nil {
		engine.CreateExtensionTab(id, url, host, dir)
	}
}

// SwitchTab 激活第 i 个标签页；若已是活跃标签则不做任何事（绝不刷新）。
func (b *Browser) SwitchTab(i int) {
	b.mu.Lock()
	if i < 0 || i >= len(b.tabs) || i == b.active {
		b.mu.Unlock()
		return
	}
	b.active = i
	tab := b.tabs[i]
	engine := b.engine
	b.mu.Unlock()

	if engine != nil {
		engine.SwitchTab(tab.ID)
	}
}

// CloseTab 关闭第 i 个标签页。
// 若是最后一个标签页，则改为导航回主页（浏览器至少保留一个标签）。
func (b *Browser) CloseTab(i int) {
	b.mu.Lock()
	if i < 0 || i >= len(b.tabs) {
		b.mu.Unlock()
		return
	}

	closing := b.tabs[i]
	engine := b.engine

	if len(b.tabs) == 1 {
		home := b.homePage // 持锁期间直接读字段；b.HomePage() 会重入加锁死锁
		if home == "" {
			home = HomeURL
		}
		b.tabs[0].URL = home
		b.tabs[0].Title = "主页"
		b.mu.Unlock()

		if engine != nil {
			engine.Navigate(closing.ID, home)
		}
		return
	}

	b.tabs = append(b.tabs[:i], b.tabs[i+1:]...)
	if b.active >= len(b.tabs) {
		b.active = len(b.tabs) - 1
	}
	active := b.tabs[b.active]
	b.mu.Unlock()

	if engine != nil {
		engine.CloseTab(closing.ID)
		engine.SwitchTab(active.ID)
	}
}

// NavigateActive 将当前活跃标签页导航到 url 并同步模型状态。
// 活跃标签为设置中心时忽略导航（设置页不承载网页）。
func (b *Browser) NavigateActive(url string) {
	b.mu.Lock()
	if b.active >= len(b.tabs) {
		b.mu.Unlock()
		return
	}
	tab := b.tabs[b.active]
	if IsSettingsURL(tab.URL) || url == "" {
		b.mu.Unlock()
		return
	}
	tab.URL = url
	engine := b.engine
	b.mu.Unlock()

	if engine != nil {
		engine.Navigate(tab.ID, url)
	}
}

// GoHome 返回主页。
func (b *Browser) GoHome() {
	b.NavigateActive(b.HomePage())
}

// GoBack / GoForward / Reload 委托给引擎操作当前活跃页。
func (b *Browser) GoBack() {
	if e := b.currentEngine(); e != nil {
		e.GoBack()
	}
}

func (b *Browser) GoForward() {
	if e := b.currentEngine(); e != nil {
		e.GoForward()
	}
}

func (b *Browser) Reload() {
	if e := b.currentEngine(); e != nil {
		e.Reload()
	}
}

// FocusContent 把键盘焦点交还给页面内容区。
func (b *Browser) FocusContent() {
	if e := b.currentEngine(); e != nil {
		e.FocusContent()
	}
}

// EvalInWebview 在当前活跃标签页的网页环境中执行 JavaScript。
func (b *Browser) EvalInWebview(script string) {
	if e := b.currentEngine(); e != nil {
		if manager, ok := e.(interface{ Eval(string) }); ok {
			manager.Eval(script)
		}
	}
}

// SetStatusText 手动设置状态栏文案。
func (b *Browser) SetStatusText(text string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.status = text
}

func (b *Browser) currentEngine() Engine {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.engine
}

// ---- 引擎回调（STA 线程调用）----

// UpdateTabState 上报指定标签页的标题与 URL 变化。
func (b *Browser) UpdateTabState(tabID, url, title string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, t := range b.tabs {
		if t.ID != tabID {
			continue
		}
		if title != "" {
			t.Title = title
		}
		if url != "" {
			t.URL = url
		}
		return
	}
}

// SetPageLoading 更新加载指示与状态栏文案。
func (b *Browser) SetPageLoading(loading bool, url string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.loading = loading
	if loading {
		b.status = "正在加载… " + url
	} else {
		b.status = "就绪 · " + url
	}
}

// ---- 设置中心标签页 ----

// OpenSettings 打开设置中心：已存在则激活，否则新建设置标签并激活。
func (b *Browser) OpenSettings() {
	b.mu.Lock()
	for i, t := range b.tabs {
		if IsSettingsURL(t.URL) {
			b.mu.Unlock()
			b.SwitchTab(i) // 已是活跃标签时为安全空操作
			return
		}
	}
	b.mu.Unlock()
	b.CreateTab(SettingsURL, SettingsTitle)
}

// IsViewingSettings 当前活跃标签是否为设置中心。
func (b *Browser) IsViewingSettings() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.active < len(b.tabs) && IsSettingsURL(b.tabs[b.active].URL)
}
