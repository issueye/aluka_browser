package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// LayoutStatusBar 底部状态栏：左侧页面标题/状态，右侧引擎标识。
func (u *UI) LayoutStatusBar(gtx layout.Context) layout.Dimensions {
	display := u.b.StatusText()
	if t, ok := u.b.ActiveTab(); ok && t.Title != "" {
		display = t.Title
	}

	return layout.Inset{
		Top: unit.Dp(6), Bottom: unit.Dp(6),
		Left: unit.Dp(12), Right: unit.Dp(12),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Alignment: layout.Middle,
			Spacing:   layout.SpaceBetween,
		}.Layout(gtx,
			// 左侧：状态与标题
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return glyph(gtx, iconDescription, 12, CStatusText)
					}),
					layout.Rigid(spacer(6)),
					layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(u.theme, unit.Sp(11), display)
						lbl.MaxLines = 1
						lbl.Truncator = "..."
						lbl.Color = CStatusText
						return lbl.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(spacer(12)),
			// 右侧：引擎标识
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return glyph(gtx, iconFlashOn, 12, CAccentGreen)
					}),
					layout.Rigid(spacer(4)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(u.theme, unit.Sp(11), "无边框 8px 圆角 · 多标签独立存活")
						lbl.Color = CAccentGreen
						return lbl.Layout(gtx)
					}),
				)
			}),
		)
	})
}
