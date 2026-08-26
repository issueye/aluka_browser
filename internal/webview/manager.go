// Package webview 封装 WebView2 多标签页引擎：每个标签页拥有独立的
// 原生 WebView2 实例，后台标签页保留完整运行状态（不销毁、不重载）。
package webview

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2/pkg/edge"

	"gio-browser/internal/userscript"
	"gio-browser/internal/win32"
)

// StateFunc 标签页状态变化回调（URL/标题/加载状态）。
type StateFunc func(tabID, url, title string, loading bool)

// OpenTabFunc 拦截到新窗口请求的回调。
type OpenTabFunc func(url string)

// tabView 代表一个独立存活的 WebView2 渲染视图。
type tabView struct {
	ID        string
	childHWND uintptr
	Chromium  *edge.Chromium
	URL       string
	Title     string
}

// Manager 管理所有 WebView2 标签页的创建、切换、销毁与布局。
//
// 公开方法均可从任意 goroutine 调用；内部通过 dispatch 序列化到
// Start 启动的 STA 线程上执行。本类型即 browser.Engine 的默认实现。
type Manager struct {
	mu          sync.Mutex
	parentHWND  uintptr
	threadID    uint32
	tasks       []func()
	views       map[string]*tabView
	activeTabID string
	onState     StateFunc
	onOpenTab   OpenTabFunc
	lastBounds  win32.Rect
}

func New() *Manager {
	return &Manager{views: make(map[string]*tabView)}
}

// CreateTab 为指定标签页创建独立存活的原生 WebView2 实例并导航到 url。
func (m *Manager) CreateTab(tabID, url string) {
	m.dispatch(func() { m.createTabLocked(tabID, url) })
}

func (m *Manager) createTabLocked(tabID, url string) {
	m.mu.Lock()
	parentHWND := m.parentHWND
	bounds := m.lastBounds
	m.mu.Unlock()

	if parentHWND == 0 {
		return
	}

	w, h := bounds.Width(), bounds.Height()
	if w <= 0 {
		w = 1280
	}
	if h <= 0 {
		h = 690
	}

	childHWND := win32.CreateStaticChild(parentHWND, bounds.Left, bounds.Top, w, h)
	if childHWND == 0 {
		log.Printf("[WebView2] 创建标签子窗口失败: %s", tabID)
		return
	}

	ch := edge.NewChromium()
	// 独立数据目录确保多实例稳定性
	ch.DataPath = filepath.Join(os.TempDir(), "gio_browser_profile")

	view := &tabView{
		ID:        tabID,
		childHWND: childHWND,
		Chromium:  ch,
		URL:       url,
		Title:     "加载中…",
	}

	id := tabID

	ch.NavigationCompletedCallback = func(_ *edge.ICoreWebView2, _ *edge.ICoreWebView2NavigationCompletedEventArgs) {
		m.mu.Lock()
		cb := m.onState
		u, t := view.URL, view.Title
		m.mu.Unlock()

		// 自动匹配并注入当前 URL 相关的用户脚本（篡改猴能力）
		if injectCode := userscript.GetGlobalManager().BuildInjectionForURL(u); injectCode != "" {
			view.Chromium.Eval(injectCode)
		}

		if cb != nil {
			cb(id, u, t, false)
		}
	}

	ch.MessageCallback = func(msg string) {
		m.handleWebMessage(view, msg)
	}

	if !ch.Embed(childHWND) {
		log.Printf("[WebView2] 标签页嵌入失败: %s", tabID)
		return
	}

	ch.Init(injectedScript)

	m.mu.Lock()
	m.views[tabID] = view
	m.mu.Unlock()

	m.setBoundsLocked(view, bounds)
	ch.Navigate(url)
	_ = ch.Show()

	// 新建即激活
	m.SwitchTab(tabID)
}

// handleWebMessage 处理页面注入脚本回传的消息（新窗口拦截 / 状态上报）。
func (m *Manager) handleWebMessage(view *tabView, msg string) {
	var parsed webViewMessage
	if err := json.Unmarshal([]byte(msg), &parsed); err != nil {
		return
	}
	id := view.ID

	switch parsed.Type {
	case "open_tab":
		if parsed.URL == "" {
			return
		}
		m.mu.Lock()
		cb := m.onOpenTab
		m.mu.Unlock()
		if cb != nil {
			cb(parsed.URL)
		}

	case "state":
		m.mu.Lock()
		if parsed.URL != "" {
			view.URL = parsed.URL
		}
		if parsed.Title != "" {
			view.Title = parsed.Title
		}
		cb := m.onState
		u, t := view.URL, view.Title
		m.mu.Unlock()

		if cb != nil {
			cb(id, u, t, false)
		}
	}
}

// SwitchTab 切换当前活跃标签页：
// 后台标签仅隐藏其子窗口，保留完整运行状态与滚动位置，切换零成本。
func (m *Manager) SwitchTab(tabID string) {
	m.dispatch(func() {
		m.mu.Lock()
		prevActive := m.activeTabID
		m.activeTabID = tabID
		bounds := m.lastBounds
		newView := m.views[tabID]
		prevView := m.views[prevActive]
		m.mu.Unlock()

		if prevView != nil && prevActive != tabID && prevView.childHWND != 0 {
			win32.Hide(prevView.childHWND)
		}
		if newView != nil && newView.childHWND != 0 {
			win32.Show(newView.childHWND)
			m.setBoundsLocked(newView, bounds)
			if newView.Chromium != nil {
				_ = newView.Chromium.Show()
				newView.Chromium.Focus()
			}
		}
	})
}

// CloseTab 关闭并销毁指定标签页的原生 WebView2。
func (m *Manager) CloseTab(tabID string) {
	m.dispatch(func() {
		m.mu.Lock()
		view := m.views[tabID]
		delete(m.views, tabID)
		if m.activeTabID == tabID {
			m.activeTabID = ""
		}
		m.mu.Unlock()

		win32.DestroyWindow(view.childHWND)
	})
}

// SetBounds 同步活跃标签页在宿主窗口客户区中的像素区域。
func (m *Manager) SetBounds(left, top, right, bottom int32) {
	m.mu.Lock()
	bounds := win32.Rect{Left: left, Top: top, Right: right, Bottom: bottom}
	if bounds == m.lastBounds {
		m.mu.Unlock()
		return
	}
	m.lastBounds = bounds
	view := m.views[m.activeTabID]
	m.mu.Unlock()

	if view != nil {
		m.dispatch(func() { m.setBoundsLocked(view, bounds) })
	}
}

// setBoundsLocked 在 STA 线程上对齐子容器窗口与 COM 控制器矩形。
func (m *Manager) setBoundsLocked(view *tabView, b win32.Rect) {
	if view == nil || view.childHWND == 0 || view.Chromium == nil {
		return
	}

	w, h := b.Width(), b.Height()
	if w <= 0 || h <= 0 {
		return
	}

	win32.PositionWindow(view.childHWND, b.Left, b.Top, w, h)

	controller := view.Chromium.GetController()
	if controller == nil {
		return
	}

	innerBounds := win32.Rect{Left: 0, Top: 0, Right: w, Bottom: h}

	type controllerWithVtbl struct {
		vtbl *[32]uintptr
	}
	c := (*controllerWithVtbl)(unsafe.Pointer(controller))
	if c != nil && c.vtbl != nil {
		putBoundsProc := c.vtbl[6] // ICoreWebView2Controller::PutBounds 位于虚表第 6 项
		if putBoundsProc != 0 {
			_, _, _ = syscall.SyscallN(
				putBoundsProc,
				uintptr(unsafe.Pointer(controller)),
				uintptr(unsafe.Pointer(&innerBounds)),
			)
		}
	}
}

// Navigate 导航指定标签页（tabID 为空则使用当前活跃标签）。
func (m *Manager) Navigate(tabID, url string) {
	m.dispatch(func() {
		m.mu.Lock()
		if tabID == "" {
			tabID = m.activeTabID
		}
		view := m.views[tabID]
		if view != nil {
			view.URL = url
		}
		m.mu.Unlock()

		if view != nil && view.Chromium != nil {
			view.Chromium.Navigate(url)
		}
	})
}

// GoBack 后退。
func (m *Manager) GoBack() { m.Eval("window.history.back()") }

// GoForward 前进。
func (m *Manager) GoForward() { m.Eval("window.history.forward()") }

// Reload 刷新当前活跃标签页。
func (m *Manager) Reload() { m.Eval("window.location.reload()") }

// FocusContent 将键盘焦点移入当前活跃 WebView2 控件。
func (m *Manager) FocusContent() {
	m.dispatch(func() {
		m.mu.Lock()
		view := m.views[m.activeTabID]
		m.mu.Unlock()

		if view == nil {
			return
		}
		win32.FocusKeyboard(view.childHWND)
		if view.Chromium != nil {
			view.Chromium.Focus()
		}
	})
}

// Eval 在当前活跃标签页执行 JavaScript。
func (m *Manager) Eval(script string) {
	m.dispatch(func() {
		m.mu.Lock()
		view := m.views[m.activeTabID]
		m.mu.Unlock()
		if view != nil && view.Chromium != nil {
			view.Chromium.Eval(script)
		}
	})
}

// SetVisible 控制当前活跃标签页的渲染视图显隐（进入全屏设置页时隐藏原生子窗口）。
func (m *Manager) SetVisible(visible bool) {
	m.dispatch(func() {
		m.mu.Lock()
		view := m.views[m.activeTabID]
		bounds := m.lastBounds
		m.mu.Unlock()

		if view == nil || view.childHWND == 0 {
			return
		}
		if visible {
			win32.Show(view.childHWND)
			m.setBoundsLocked(view, bounds)
			if view.Chromium != nil {
				_ = view.Chromium.Show()
				view.Chromium.Focus()
			}
		} else {
			win32.Hide(view.childHWND)
			if view.Chromium != nil {
				_ = view.Chromium.Hide()
			}
		}
	})
}

// webViewMessage 与注入脚本 postMessage 的 JSON 结构对应。
type webViewMessage struct {
	Type  string `json:"type"`
	TabID string `json:"tabId"`
	URL   string `json:"url"`
	Title string `json:"title"`
}
