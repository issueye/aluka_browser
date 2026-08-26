package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// LayoutQuickBar 快捷书签栏。
func (u *UI) LayoutQuickBar(gtx layout.Context) layout.Dimensions {
	var children []layout.FlexChild

	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return glyph(gtx, iconBookmark, 13, CTabInactiveText)
			}),
			layout.Rigid(spacer(4)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(u.theme, unit.Sp(12), "快捷访问: ")
				lbl.Color = CTabInactiveText
				return lbl.Layout(gtx)
			}),
		)
	}))

	for _, bm := range u.bmCtls {
		b := bm
		if b.click.Clicked(gtx) {
			u.b.NavigateActive(b.data.URL)
			u.b.FocusContent()
		}
		children = append(children,
			layout.Rigid(spacer(6)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(u.theme, &b.click, b.data.Name)
				btn.Background = CSubBtnBG
				btn.Color = CSubBtnFG
				btn.TextSize = unit.Sp(11)
				btn.Inset = layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(10), Right: unit.Dp(10)}
				return btn.Layout(gtx)
			}),
		)
	}

	return layout.Inset{
		Left: unit.Dp(10), Right: unit.Dp(10),
		Bottom: unit.Dp(8),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	})
}
