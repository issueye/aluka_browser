// Package app 负责应用装配：创建窗口、连接 UI 与页面引擎、驱动事件循环。
package app

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	gioapp "gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"gio-browser/internal/browser"
	"gio-browser/internal/config"
	"gio-browser/internal/ui"
	"gio-browser/internal/webview"
	"gio-browser/internal/win32"
)

// WindowTitle 窗口标题，同时用于 HWND 探测。
const WindowTitle = "Gio Browser"

// Main 阻塞运行 Gio 平台主循环（必须在 main goroutine 调用）。
func Main() { gioapp.Main() }

// Run 创建窗口并进入主事件循环，窗口关闭时返回。
func Run() error {
	// 加载用户本地配置并初始化网络代理环境变量
	config.Load()

	win := new(gioapp.Window)
	win.Option(
		gioapp.Title(WindowTitle),
		gioapp.Size(unit.Dp(1280), unit.Dp(800)),
		gioapp.MinSize(unit.Dp(600), unit.Dp(400)),
		gioapp.Decorated(false), // 无边框窗口
	)

	th := material.NewTheme()
	engine := webview.New()
	b := browser.New(engine)
	u := ui.New(th, b, win)

	// 进程管理浮窗：同一时刻仅允许一个实例
	var panelOpen atomic.Bool
	u.OnOpenProcessManager = func() {
		if !panelOpen.CompareAndSwap(false, true) {
			return
		}
		go func() {
			defer panelOpen.Store(false)
			ui.RunProcessPanel(WindowTitle + " · 进程管理")
		}()
	}

	startEngineAsync(WindowTitle, engine, b, win)

	var ops op.Ops
	for {
		switch e := win.Event().(type) {
		case gioapp.DestroyEvent:
			// 先收尾页面引擎（销毁 WebView2 视图、退出 STA 线程），
			// 再返回由 main 结束进程
			engine.Close()
			return e.Err

		case gioapp.FrameEvent:
			gtx := gioapp.NewContext(&ops, e)
			metrics := u.LayoutRoot(gtx)

			// 同步页面引擎到内容区矩形
			w := int32(gtx.Constraints.Max.X)
			h := int32(gtx.Constraints.Max.Y)
			bottom := h - int32(metrics.Status)
			if bottom < int32(metrics.Top) {
				bottom = int32(metrics.Top)
			}
			engine.SetBounds(0, int32(metrics.Top), w, bottom)

			e.Frame(gtx.Ops)
		}
	}
}

// startEngineAsync 异步探测宿主窗口 HWND 并启动 WebView2 引擎线程，
// 成功后为初始标签页挂载真实页面实例。
func startEngineAsync(titleSubstr string, engine *webview.Manager, b *browser.Browser, win *gioapp.Window) {
	go func() {
		var (
			found bool
			mu    sync.Mutex
		)

		for i := 0; i < 40; i++ {
			time.Sleep(100 * time.Millisecond)

			mu.Lock()
			ok := found
			mu.Unlock()
			if ok {
				return
			}

			hwnd := win32.FindWindowByTitle(titleSubstr)
			if hwnd == 0 {
				continue
			}

			log.Printf("[Gio Browser] 已捕获 Win32 宿主窗口 HWND: 0x%x", hwnd)

			// 任务栏 / Alt+Tab 应用图标
			iconBig, iconSmall := AppIcons()
			win32.SetWindowIcons(hwnd, iconBig, iconSmall)

			engine.Start(
				hwnd,
				func(tabID, url, title string, loading bool) {
					b.UpdateTabState(tabID, url, title)
					b.SetPageLoading(loading, url)
					win.Invalidate()
				},
				func(url string) {
					log.Printf("[Gio Browser] 拦截新窗口链接，转为新标签页: %s", url)
					b.CreateTab(url, "加载中…")
					win.Invalidate()
				},
			)

			engine.CreateTab(browser.DefaultTabID, browser.HomeURL)

			mu.Lock()
			found = true
			mu.Unlock()
			return
		}
		log.Println("[Gio Browser] 未能找到宿主窗口，页面引擎未启动")
	}()
}
