package ui

import (
	"image"
	"image/color"

	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"gio-browser/internal/browser"
)

// LayoutTabBar 顶部多标签栏：
// 左侧红绿灯窗口控制，中间胶囊形标签页列表（含站点头像）与新建按钮，
// LayoutTabBar 顶部多标签栏：
// 左侧红绿灯窗口控制，中间胶囊形标签页列表（含站点头像）与新建按钮，
// 右侧空白区注册为系统原生拖拽移动区域。
func (u *UI) LayoutTabBar(gtx layout.Context) layout.Dimensions {
	u.handleWindowControls(gtx)

	if u.newTabBtn.Clicked(gtx) {
		u.b.CreateTab("", "新标签页")
	}

	// 优先处理所有标签页的点击与关闭事件，确保当前帧即时响应高亮
	tabs := u.b.Tabs()
	for i := range tabs {
		tab := tabs[i]
		ctl := u.tabCtls[tab.ID]
		if ctl == nil {
			ctl = &tabCtl{}
			u.tabCtls[tab.ID] = ctl
		}

		if ctl.close.Clicked(gtx) {
			u.b.CloseTab(i)
			u.pruneTabControls()
			break
		}
		if ctl.click.Clicked(gtx) {
			u.b.SwitchTab(i)
			break
		}
	}

	var children []layout.FlexChild

	// 1. 红绿灯 (🔴 🟡 🟢)
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Left: unit.Dp(12), Right: unit.Dp(14),
			Top: unit.Dp(10), Bottom: unit.Dp(6),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return trafficLight(gtx, &u.closeBtn, CCloseFill, CCloseHover)
				}),
				layout.Rigid(spacer(8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return trafficLight(gtx, &u.minBtn, CMinimizeFill, CMinimizeHover)
				}),
				layout.Rigid(spacer(8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return trafficLight(gtx, &u.maxBtn, CMaxFill, CMaxHover)
				}),
			)
		})
	}))

	// 2. 标签页列表（以最新数据为准）
	tabs = u.b.Tabs()
	activeIdx := u.b.ActiveIndex()
	for i := range tabs {
		tab := tabs[i]
		idx := i

		ctl := u.tabCtls[tab.ID]
		if ctl == nil {
			ctl = &tabCtl{}
			u.tabCtls[tab.ID] = ctl
		}

		children = append(children, u.tabChild(tab, idx == activeIdx, ctl))
	}

	// 3. ＋ 新建标签按钮
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		btn := material.IconButton(u.theme, &u.newTabBtn, iconAdd, "新建标签页")
		btn.Background = transparent
		btn.Color = CSubBtnFG
		btn.Size = unit.Dp(14)
		btn.Inset = layout.UniformInset(unit.Dp(6))
		return btn.Layout(gtx)
	}))

	// 4. 剩余空白：注册为系统原生拖拽移动区域。
	// Gio Windows 后端会在 WM_NCHITTEST 对该区域返回 HTCAPTION，
	// 由窗口所属线程原生完成拖动（支持 Aero Snap / 双击最大化），
	// 不经过用户代码，不会阻塞渲染循环。
	barHeight := dp(gtx, 34)
	children = append(children, layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
		dims := image.Point{X: gtx.Constraints.Max.X, Y: barHeight}
		defer clip.Rect{Max: dims}.Push(gtx.Ops).Pop()
		system.ActionInputOp(system.ActionMove).Add(gtx.Ops)
		return layout.Dimensions{Size: dims}
	}))

	return layout.Inset{
		Top: unit.Dp(6), Bottom: unit.Dp(2),
		Left: unit.Dp(4), Right: unit.Dp(10),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx, children...)
	})
}

// tabChild 渲染单个胶囊形标签：站点头像 + 标题 + 关闭按钮；
// 活跃标签有底色胶囊与底部亮条，非活跃标签悬停时浮现淡色胶囊。
func (u *UI) tabChild(tab browser.Tab, isActive bool, ctl *tabCtl) layout.FlexChild {
	hovered := ctl.click.Hovered()

	bg := transparent
	switch {
	case isActive:
		bg = CTabActiveBG
	case hovered:
		bg = CTabHoverBG
	}
	textColor := CTabInactiveText
	if isActive || hovered {
		textColor = CTabActiveText
	}
	closeColor := CTabIdleClose
	if isActive || hovered {
		closeColor = CTabActiveText
	}
	title := tab.Title
	if title == "" {
		title = "新标签页"
	}

	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Right: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			// 先录制内容以获得尺寸，再垫入胶囊底色与活跃亮条
			macro := op.Record(gtx.Ops)
			dims := u.tabContent(tab, title, textColor, closeColor, ctl)(gtx)
			call := macro.Stop()

			radius := gtx.Dp(unit.Dp(8))
			pill := clip.RRect{
				Rect: image.Rectangle{Max: dims.Size},
				SE:   radius, SW: radius, NE: radius, NW: radius,
			}
			stack := pill.Push(gtx.Ops)
			if bg.A > 0 {
				paint.FillShape(gtx.Ops, bg, clip.Rect{Max: dims.Size}.Op())
			}
			if isActive {
				barH := gtx.Dp(unit.Dp(3))
				barW := dims.Size.X * 3 / 5
				x0 := (dims.Size.X - barW) / 2
				paint.FillShape(gtx.Ops, CUnderline,
					clip.Rect{Min: image.Point{X: x0, Y: dims.Size.Y - barH}, Max: image.Point{X: x0 + barW, Y: dims.Size.Y}}.Op())
			}
			stack.Pop()

			call.Add(gtx.Ops)
			return dims
		})
	})
}

func (u *UI) tabContent(tab browser.Tab, title string, textColor, closeColor color.NRGBA, ctl *tabCtl) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top: unit.Dp(5), Bottom: unit.Dp(5),
			Left: unit.Dp(8), Right: unit.Dp(6),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				// 可点击的主体区域（头像 + 标题）
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ctl.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							// 站点头像
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if badge, ok := SiteBadgeFor(tab.URL); ok {
									return u.siteBadge(gtx, badge)
								}
								return layout.Dimensions{}
							}),
							layout.Rigid(spacer(6)),
							// 标题
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Label(u.theme, unit.Sp(11), title)
								lbl.Color = textColor
								lbl.MaxLines = 1
								lbl.Truncator = "..."
								gtx.Constraints.Max.X = dp(gtx, 120)
								return lbl.Layout(gtx)
							}),
						)
					})
				}),
				layout.Rigid(spacer(4)),
				// 关闭按钮 ×（常驻占位避免宽度抖动，非活跃时淡化）
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.IconButton(u.theme, &ctl.close, iconClose, "关闭标签页")
					btn.Background = transparent
					btn.Color = closeColor
					btn.Size = unit.Dp(11)
					btn.Inset = layout.UniformInset(unit.Dp(2))
					return btn.Layout(gtx)
				}),
			)
		})
	}
}

// siteBadge 绘制站点彩色圆形头像（首字符占位）。
func (u *UI) siteBadge(gtx layout.Context, badge SiteBadge) layout.Dimensions {
	s := gtx.Dp(unit.Dp(16))
	sq := image.Rectangle{Max: image.Point{X: s, Y: s}}
	gtx.Constraints = layout.Exact(image.Point{X: s, Y: s})

	defer clip.RRect{Rect: sq, SE: s / 2, SW: s / 2, NE: s / 2, NW: s / 2}.Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, badge.BG[0], clip.Rect{Max: sq.Max}.Op())

	layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		lbl := material.Label(u.theme, unit.Sp(9), badge.Initial)
		lbl.Color = badge.BG[1]
		return lbl.Layout(gtx)
	})
	return layout.Dimensions{Size: image.Point{X: s, Y: s}}
}

// handleWindowControls 红绿灯行为：关闭 / 最小化 / 最大化-还原。
func (u *UI) handleWindowControls(gtx layout.Context) {
	if u.closeBtn.Clicked(gtx) {
		u.win.Perform(system.ActionClose)
		return
	}
	if u.minBtn.Clicked(gtx) {
		u.win.Perform(system.ActionMinimize)
		return
	}
	if u.maxBtn.Clicked(gtx) {
		u.maximized = !u.maximized
		if u.maximized {
			u.win.Perform(system.ActionMaximize)
		} else {
			u.win.Perform(system.ActionUnmaximize)
		}
	}
}

// pruneTabControls 清理已关闭标签页的控件状态，防止 map 无界增长。
func (u *UI) pruneTabControls() {
	live := make(map[string]struct{}, len(u.tabCtls))
	for _, t := range u.b.Tabs() {
		live[t.ID] = struct{}{}
	}
	for id := range u.tabCtls {
		if _, ok := live[id]; !ok {
			delete(u.tabCtls, id)
		}
	}
}
