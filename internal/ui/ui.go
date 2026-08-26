// Package ui 实现浏览器全部 Gio 界面：标签栏、工具栏、书签栏与状态栏。
//
// 该包只读渲染 browser.Browser 模型，用户交互一律调用模型方法；
// 自身不接触 WebView2 与 Win32 细节。
package ui

import (
	"image"
	"image/color"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"gio-browser/internal/browser"
)

// FrameMetrics 描述一次布局后各 chrome 区域的高度（物理像素，DIP 转换后）。
type FrameMetrics struct {
	Top    int // 标签栏+工具栏+书签栏 总高
	Status int // 底部状态栏高
}

// tabCtl 单个标签页的交互控件状态（与 browser.Tab 数据模型分离）。
type tabCtl struct {
	click widget.Clickable
	close widget.Clickable
}

// bmCtl 单个书签按钮。
type bmCtl struct {
	data  browser.Bookmark
	click widget.Clickable
}

// UI 浏览器界面。一个实例对应一个应用窗口。
type UI struct {
	theme *material.Theme
	b     *browser.Browser
	win   *app.Window

	// 窗口控制红绿灯
	closeBtn  widget.Clickable
	minBtn    widget.Clickable
	maxBtn    widget.Clickable
	maximized bool

	// 工具栏
	newTabBtn  widget.Clickable
	backBtn    widget.Clickable
	forwardBtn widget.Clickable
	reloadBtn  widget.Clickable
	homeBtn    widget.Clickable
	goBtn      widget.Clickable
	urlEditor  widget.Editor

	// 动态控件集合
	tabCtls map[string]*tabCtl
	bmCtls  []*bmCtl

	// 地址栏与当前页 URL 的同步水位
	lastSyncedURL string
}

// New 构建界面并从模型读取初始书签列表。
func New(th *material.Theme, b *browser.Browser, win *app.Window) *UI {
	u := &UI{
		theme:   th,
		b:       b,
		win:     win,
		tabCtls: make(map[string]*tabCtl),
	}
	u.urlEditor.SingleLine = true
	u.urlEditor.Submit = true
	if t, ok := b.ActiveTab(); ok {
		u.urlEditor.SetText(t.URL)
		u.lastSyncedURL = t.URL
	}
	for _, bm := range b.Bookmarks() {
		u.bmCtls = append(u.bmCtls, &bmCtl{data: bm})
	}
	return u
}

// LayoutRoot 绘制整窗（圆角裁剪、各区背景、状态栏边框），并返回区域度量，
// 供 app 层把页面引擎同步到正确矩形。
func (u *UI) LayoutRoot(gtx layout.Context) (m FrameMetrics) {
	// 全局 8px 圆角裁剪
	radius := gtx.Dp(unit.Dp(8))
	roundClip := clip.RRect{
		Rect: image.Rectangle{Max: gtx.Constraints.Max},
		SE:   radius, SW: radius, NE: radius, NW: radius,
	}
	stack := roundClip.Push(gtx.Ops)

	paint.FillShape(gtx.Ops, CWindowBG, clip.Rect{Max: gtx.Constraints.Max}.Op())

	var top int

	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims := background(gtx, CWindowBG, func(gtx layout.Context) layout.Dimensions {
				return u.LayoutTabBar(gtx)
			})
			top += dims.Size.Y
			return dims
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims := background(gtx, CToolbarBG, func(gtx layout.Context) layout.Dimensions {
				return u.LayoutTopBar(gtx)
			})
			top += dims.Size.Y
			return dims
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims := background(gtx, CQuickBG, func(gtx layout.Context) layout.Dimensions {
				return u.LayoutQuickBar(gtx)
			})
			top += dims.Size.Y
			return dims
		}),
		// 页面内容区占位（WebView2 子窗口将覆盖在此区域之上）
		layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, CContentBG, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims := background(gtx, CStatusBG, func(gtx layout.Context) layout.Dimensions {
				return u.LayoutStatusBar(gtx)
			})
			m.Status = dims.Size.Y
			return dims
		}),
	)

	// 无边框外围 1px 微质感描边
	paint.FillShape(gtx.Ops, CBorder, clip.Stroke{
		Path:  roundClip.Path(gtx.Ops),
		Width: float32(gtx.Dp(unit.Dp(1))),
	}.Op())

	stack.Pop()

	m.Top = top
	return m
}

// syncAddressBar 将活跃标签 URL 同步到地址栏；输入中不打扰。
func (u *UI) syncAddressBar(gtx layout.Context) {
	t, ok := u.b.ActiveTab()
	if !ok || t.URL == "" || t.URL == u.lastSyncedURL {
		return
	}
	if gtx.Focused(&u.urlEditor) {
		return
	}
	u.urlEditor.SetText(t.URL)
	u.lastSyncedURL = t.URL
}

// drawBackground 先录制子布局内容，再在其下填充底色（记录顺序换位技巧）。
func background(gtx layout.Context, bg color.NRGBA, w layout.Widget) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := w(gtx)
	call := macro.Stop()

	paint.FillShape(gtx.Ops, bg, clip.Rect{Max: dims.Size}.Op())
	call.Add(gtx.Ops)
	return dims
}

// trafficLight 绘制 macOS 风格窗口控制圆点按钮。
func trafficLight(gtx layout.Context, click *widget.Clickable, fill, hoverFill color.NRGBA) layout.Dimensions {
	size := gtx.Dp(unit.Dp(12))
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		btnColor := fill
		if click.Hovered() {
			btnColor = hoverFill
		}
		d := image.Point{X: size, Y: size}
		rr := clip.RRect{
			Rect: image.Rectangle{Max: d},
			SE:   size / 2, SW: size / 2, NE: size / 2, NW: size / 2,
		}
		defer rr.Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, btnColor, clip.Rect{Max: d}.Op())
		return layout.Dimensions{Size: d}
	})
}

// spacer 返回固定水平间距。
func spacer(dp int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Spacer{Width: unit.Dp(dp)}.Layout(gtx)
	}
}

// glyph 以指定尺寸绘制矢量图标（Gio 图标尺寸取自约束 Min）。
func glyph(gtx layout.Context, ic *widget.Icon, size int, c color.NRGBA) layout.Dimensions {
	s := gtx.Dp(unit.Dp(size))
	gtx.Constraints = layout.Exact(image.Point{X: s, Y: s})
	return ic.Layout(gtx, c)
}

var transparent = color.NRGBA{}

func dp(gtx layout.Context, v int) int { return gtx.Dp(unit.Dp(v)) }
