// Package webview 封装 WebView2 多标签页引擎：每个标签页拥有独立的
// 原生 WebView2 实例，后台标签页保留完整运行状态（不销毁、不重载）。
package webview

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2/pkg/edge"

	"gio-browser/internal/extension"
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

	// 后台挂起状态（内存优化：空闲后台标签挂起以释放工作集）
	i3         *edge.ICoreWebView2_3
	lastActive time.Time
	suspended  bool
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
	stopped     chan struct{} // STA 消息循环退出信号（Start 后有效）
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

		// 自动匹配并注入当前 URL 相关的扩展内容脚本（chrome.* 兼容沙箱）
		if injectCode := extension.GetGlobalManager().BuildInjectionForURL(u); injectCode != "" {
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

	// 获取 ICoreWebView2_3（挂起/恢复能力）；旧运行时可能返回 nil，此时跳过挂起
	view.i3 = ch.GetICoreWebView2_3()
	view.lastActive = time.Now()

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

	case "ext_message":
		// 扩展内容脚本经 chrome.runtime.sendMessage 上行的消息：
		// v1 仅记录日志，为未来宿主侧消息总线预留
		log.Printf("[Extension] 收到扩展消息 %s (%s): %v", parsed.ExtName, parsed.ExtID, parsed.Payload)
	}
}

// SwitchTab 切换当前活跃标签页：
// 后台标签隐藏其子窗口与控制器（停止后台渲染），空闲超时后挂起释放内存；
// 切回时先唤醒再显示。
func (m *Manager) SwitchTab(tabID string) {
	m.dispatch(func() {
		m.mu.Lock()
		prevActive := m.activeTabID
		m.activeTabID = tabID
		bounds := m.lastBounds
		newView := m.views[tabID]
		prevView := m.views[prevActive]
		m.mu.Unlock()

		now := time.Now()
		if prevView != nil && prevActive != tabID {
			prevView.lastActive = now
			// 控制器不可见必须设置：否则 Chromium 不知道自己进了后台，
			// 渲染/定时器/rAF 全速继续（实测每标签空烧 ~9% 单核）
			if prevView.Chromium != nil {
				_ = prevView.Chromium.Hide()
			}
			if prevView.childHWND != 0 {
				win32.Hide(prevView.childHWND)
			}
		}
		if newView != nil {
			newView.lastActive = now
			// 唤醒已挂起的标签（对未挂起对象是安全空操作）
			if newView.suspended && newView.i3 != nil {
				if err := resumeView(newView.i3); err == nil {
					newView.suspended = false
				} else {
					log.Printf("[WebView2] 恢复标签 %s 失败: %v", tabID, err)
				}
			}
			if newView.childHWND != 0 {
				win32.Show(newView.childHWND)
				m.setBoundsLocked(newView, bounds)
				if newView.Chromium != nil {
					_ = newView.Chromium.Show()
					newView.Chromium.Focus()
				}
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

// Close 销毁全部标签视图并停止 STA 消息循环线程，供应用退出前调用。
// 引擎未启动（Start 未成功）时为空操作；最多等待 2 秒即返回，
// 避免个别情况下线程退出异常拖住关闭流程。
func (m *Manager) Close() {
	m.mu.Lock()
	tid := m.threadID
	stopped := m.stopped
	m.mu.Unlock()
	if tid == 0 {
		return
	}

	// 先投递子窗口销毁任务（队列 FIFO，保证先于 WM_QUIT 被处理）
	m.dispatch(func() {
		m.mu.Lock()
		views := m.views
		m.views = make(map[string]*tabView)
		m.activeTabID = ""
		m.mu.Unlock()
		for _, v := range views {
			win32.DestroyWindow(v.childHWND)
		}
	})

	postThreadMessage(tid, WM_QUIT)
	if stopped != nil {
		select {
		case <-stopped:
		case <-time.After(2 * time.Second):
			log.Println("[WebView2] 等待引擎线程退出超时，继续关闭流程")
		}
	}
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
			// 导航前唤醒已挂起的标签，否则导航会被忽略
			if view.suspended && view.i3 != nil {
				if resumeView(view.i3) == nil {
					view.suspended = false
				}
			}
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

// suspendAfter 读取后台标签挂起阈值：环境变量 GIO_SUSPEND_AFTER_SEC（秒），
// 最小 10 秒（便于测试），默认 3 分钟。
func suspendAfter() time.Duration {
	if v := os.Getenv("GIO_SUSPEND_AFTER_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 10 {
			return time.Duration(n) * time.Second
		}
	}
	return 3 * time.Minute
}

// startSuspendJanitor 周期执行后台标签的内存回收。
//
// 默认模式：后台标签空闲超阈值时，对浏览器进程树（宿主 + WebView2 全家桶）
// 做工作集裁剪。后台标签已被 SwitchTab 置为控制器不可见、无渲染活动，
// 被换出的页面不会回流，实测可压缩 90% 以上物理内存且零崩溃。
//
// 实验模式（GIO_SUSPEND_MODE=api）：逐个对空闲后台标签调用
// ICoreWebView2_3::TrySuspendAsync。该调用在部分运行时版本上存在崩溃问题
// （MicrosoftEdge/WebView2Feedback #2121），故默认不启用。
func (m *Manager) startSuspendJanitor() {
	after := suspendAfter()
	apiMode := os.Getenv("GIO_SUSPEND_MODE") == "api"
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			m.mu.Lock()
			active := m.activeTabID
			idleBackground := 0
			var candidates []*tabView
			for id, v := range m.views {
				if id == active {
					continue
				}
				if time.Since(v.lastActive) > after {
					idleBackground++
					if !v.suspended && v.i3 != nil {
						candidates = append(candidates, v)
					}
				}
			}
			m.mu.Unlock()

			if idleBackground == 0 {
				continue
			}

			if apiMode {
				for _, v := range candidates {
					cand := v
					m.dispatch(func() {
						// STA 线程上二次确认（期间可能被切回或已挂起）
						m.mu.Lock()
						if m.activeTabID == cand.ID || cand.suspended {
							m.mu.Unlock()
							return
						}
						m.mu.Unlock()

						if cand.Chromium != nil {
							_ = cand.Chromium.Hide()
						}
						if cand.childHWND != 0 {
							win32.Hide(cand.childHWND)
						}
						if err := suspendView(cand.i3); err != nil {
							log.Printf("[WebView2] 挂起标签 %s 失败: %v", cand.ID, err)
							return
						}
						cand.suspended = true
						log.Printf("[WebView2] 已挂起后台标签: %s (%s)", cand.Title, cand.URL)
					})
				}
				continue
			}

			// 默认裁剪模式（工作集裁剪为普通 syscall，无需 STA）；
			// 排除宿主 UI 自身，保证前台交互不被换出干扰。
			// 单进程裁剪失败会在 win32 层留日志，这里只报成功汇总。
			if n := win32.TrimProcessTree(win32.CurrentPID(), map[uint32]bool{win32.CurrentPID(): true}); n > 0 {
				log.Printf("[内存] 后台空闲标签 %d 个，已裁剪浏览器进程树 %d 个进程的工作集", idleBackground, n)
			}
		}
	}()
}

// webViewMessage 与注入脚本 postMessage 的 JSON 结构对应。
type webViewMessage struct {
	Type    string `json:"type"`
	TabID   string `json:"tabId"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	ExtID   string `json:"extId,omitempty"`
	ExtName string `json:"extName,omitempty"`
	Payload any    `json:"payload,omitempty"`
}
