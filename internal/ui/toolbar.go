package ui

import (
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"gio-browser/internal/browser"
)

// LayoutTopBar 主工具栏：前进/后退/刷新/主页 + 地址栏 + 前往按钮。
func (u *UI) LayoutTopBar(gtx layout.Context) layout.Dimensions {
	u.syncAddressBar(gtx)

	switch {
	case u.backBtn.Clicked(gtx):
		u.b.GoBack()
	case u.forwardBtn.Clicked(gtx):
		u.b.GoForward()
	case u.reloadBtn.Clicked(gtx):
		u.b.Reload()
	case u.homeBtn.Clicked(gtx):
		u.b.GoHome()
	case u.goBtn.Clicked(gtx):
		u.submitAddress(gtx)
	}

	// 地址栏回车提交
	for {
		event, ok := u.urlEditor.Update(gtx)
		if !ok {
			break
		}
		if _, isSubmit := event.(widget.SubmitEvent); isSubmit {
			u.submitAddress(gtx)
		}
	}

	return layout.Inset{
		Top: unit.Dp(6), Bottom: unit.Dp(6),
		Left: unit.Dp(10), Right: unit.Dp(10),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			u.navIconButton(&u.backBtn, iconBack, "后退"),
			layout.Rigid(spacer(6)),
			u.navIconButton(&u.forwardBtn, iconForward, "前进"),
			layout.Rigid(spacer(6)),
			u.navIconButton(&u.reloadBtn, iconRefresh, "刷新"),
			layout.Rigid(spacer(6)),
			u.navIconButton(&u.homeBtn, iconHome, "主页"),
			layout.Rigid(spacer(10)),
			// 地址栏输入框（带安全状态前缀图标）
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				border := widget.Border{
					Color:        CInputBorder,
					CornerRadius: unit.Dp(6),
					Width:        unit.Dp(1),
				}
				return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(7)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								ic := u.securityIcon()
								if ic == nil {
									return layout.Dimensions{}
								}
								return glyph(gtx, ic, 12, CEditorHint)
							}),
							layout.Rigid(spacer(6)),
							layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
								ed := material.Editor(u.theme, &u.urlEditor, "输入网址或搜索关键词...")
								ed.Color = CEditorFG
								ed.HintColor = CEditorHint
								return ed.Layout(gtx)
							}),
						)
					})
				})
			}),
			layout.Rigid(spacer(8)),
			// 前往按钮
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(u.theme, &u.goBtn, "前往")
				btn.Background = CAccent
				btn.Color = COnAccent
				btn.Inset = layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(16), Right: unit.Dp(16)}
				return btn.Layout(gtx)
			}),
		)
	})
}

// submitAddress 归一化地址栏输入并导航，随后把焦点交还页面。
func (u *UI) submitAddress(gtx layout.Context) {
	target := browser.NormalizeInputURL(u.urlEditor.Text())
	if target == "" {
		return
	}
	u.urlEditor.SetText(target)
	u.lastSyncedURL = target
	u.b.NavigateActive(target)
	gtx.Execute(key.FocusCmd{Tag: nil})
	u.b.FocusContent()
}

// securityIcon 依据活跃标签的 URL scheme 返回地址栏前缀图标：
// https 显示锁（安全），其余显示地球。
func (u *UI) securityIcon() *widget.Icon {
	t, ok := u.b.ActiveTab()
	if ok && strings.HasPrefix(t.URL, "https://") {
		return iconLock
	}
	return iconGlobe
}

// navIconButton 工具栏图标按钮（后退/前进/刷新/主页）。
func (u *UI) navIconButton(click *widget.Clickable, icon *widget.Icon, desc string) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		btn := material.IconButton(u.theme, click, icon, desc)
		btn.Background = CBtnBG
		btn.Color = CBtnFG
		btn.Size = unit.Dp(18)
		btn.Inset = layout.UniformInset(unit.Dp(7))
		return btn.Layout(gtx)
	})
}
