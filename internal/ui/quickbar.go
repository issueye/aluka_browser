package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// LayoutQuickBar 快捷访问栏：条目来自配置（设置中心「快捷访问」可管理），
// 每帧从模型读取，增删即时反映；控件按条目内容缓存复用。
func (u *UI) LayoutQuickBar(gtx layout.Context) layout.Dimensions {
	links := u.b.Bookmarks()

	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
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
		}),
	}

	if len(links) == 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Label(u.theme, unit.Sp(11), "暂无条目，可在设置中心「快捷访问」中添加")
			lbl.Color = CEditorHint
			return lbl.Layout(gtx)
		}))
	}

	alive := make(map[string]bool, len(links))
	for _, bm := range links {
		key := bm.Name + "\x00" + bm.URL
		alive[key] = true
		ctl := u.bmCtls[key]
		if ctl == nil {
			ctl = &bmCtl{}
			u.bmCtls[key] = ctl
		}
		c := ctl
		name, url := bm.Name, bm.URL
		if c.click.Clicked(gtx) {
			u.b.NavigateActive(url)
			u.b.FocusContent()
		}
		children = append(children,
			layout.Rigid(spacer(6)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(u.theme, &c.click, name)
				btn.Background = CSubBtnBG
				btn.Color = CSubBtnFG
				btn.TextSize = unit.Sp(11)
				btn.Inset = layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(10), Right: unit.Dp(10)}
				return btn.Layout(gtx)
			}),
		)
	}
	// 回收已删除条目的控件
	for k := range u.bmCtls {
		if !alive[k] {
			delete(u.bmCtls, k)
		}
	}

	return layout.Inset{
		Left: unit.Dp(10), Right: unit.Dp(10),
		Bottom: unit.Dp(8),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	})
}
