package ui

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"gio-browser/internal/config"
)

// settingsSection 设置中心左侧导航项。
type settingsSection int

const (
	secGeneral settingsSection = iota
	secQuickLinks
	secUserscripts
	secExtensions
)

// settingsUI 设置中心状态：左导航选中项 + 各内容面板控件。
type settingsUI struct {
	section   settingsSection
	navClicks map[settingsSection]*widget.Clickable

	homeEditor  widget.Editor
	saveHomeBtn widget.Clickable

	qlNameEditor widget.Editor
	qlURLEditor  widget.Editor
	qlAddBtn     widget.Clickable
	qlList       layout.List
	qlDelete     map[string]*widget.Clickable // 按条目 URL 复用删除按钮

	us  userscriptUIState
	ext extensionsUIState

	statusMsg  string
	statusTime time.Time
}

// initSettingsState 初始化或重置设置中心状态（ui.New 时调用）。
func (u *UI) initSettingsState() {
	u.settings = settingsUI{
		section:   secGeneral,
		navClicks: make(map[settingsSection]*widget.Clickable),
		qlDelete:  make(map[string]*widget.Clickable),
	}
	u.settings.us = userscriptUIState{
		mode:  usViewList,
		cards: make(map[string]*scriptCardCtl),
	}
	u.settings.us.list.Axis = layout.Vertical
	u.settings.ext = extensionsUIState{
		cards: make(map[string]*extCardCtl),
	}
	u.settings.ext.list.Axis = layout.Vertical
	u.settings.qlList.Axis = layout.Vertical // List 零值轴为横向，必须显式指定
	u.settings.homeEditor.SingleLine = true
	u.settings.homeEditor.SetText(u.b.HomePage())
	u.settings.qlNameEditor.SingleLine = true
	u.settings.qlURLEditor.SingleLine = true
}

// setStatus 设置右下状态提示。
func (u *UI) setStatus(format string, args ...any) {
	u.settings.statusMsg = fmt.Sprintf(format, args...)
	u.settings.statusTime = time.Now()
}

// persistBrowserPrefs 将浏览器内存中的主页/快捷访问写入配置文件。
func (u *UI) persistBrowserPrefs() {
	cfg := config.Config{HomePage: u.b.HomePage()}
	for _, bm := range u.b.Bookmarks() {
		cfg.QuickLinks = append(cfg.QuickLinks, config.QuickLink{Name: bm.Name, URL: bm.URL})
	}
	if err := config.Save(cfg); err != nil {
		u.setStatus("保存失败: %v", err)
	}
}

// LayoutSettingsHub 渲染设置中心（活跃标签为 gio://settings 时的内容区）：
// 左侧导航 + 右侧内容区。
func (u *UI) LayoutSettingsHub(gtx layout.Context) layout.Dimensions {
	u.handleSettingsHubEvents(gtx)

	paint.FillShape(gtx.Ops, CContentBG, clip.Rect{Max: gtx.Constraints.Max}.Op())

	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		// 左侧导航
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.layoutSettingsNav(gtx)
		}),
		// 分隔线
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10)}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					paint.FillShape(gtx.Ops, CBorder, clip.Rect{Max: image.Point{X: gtx.Dp(unit.Dp(1)), Y: gtx.Constraints.Max.Y}}.Op())
					return layout.Dimensions{Size: image.Point{X: gtx.Dp(unit.Dp(1)), Y: gtx.Constraints.Max.Y}}
				})
		}),
		// 右侧内容
		layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top: unit.Dp(14), Bottom: unit.Dp(12),
				Left: unit.Dp(20), Right: unit.Dp(20),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				switch u.settings.section {
				case secQuickLinks:
					return u.layoutQuickLinksPanel(gtx)
				case secUserscripts:
					return u.layoutUserscriptPanel(gtx)
				case secExtensions:
					return u.layoutExtensionPanel(gtx)
				default:
					return u.layoutGeneralPanel(gtx)
				}
			})
		}),
	)
}

// handleSettingsHubEvents 处理通用交互（导航切换、主页保存、快捷访问增删）。
func (u *UI) handleSettingsHubEvents(gtx layout.Context) {
	st := &u.settings

	// 左侧导航切换
	for _, s := range []settingsSection{secGeneral, secQuickLinks, secUserscripts, secExtensions} {
		if ctl := st.navClicks[s]; ctl != nil && ctl.Clicked(gtx) {
			st.section = s
			break
		}
	}

	if st.saveHomeBtn.Clicked(gtx) {
		home := strings.TrimSpace(st.homeEditor.Text())
		if home == "" {
			u.setStatus("主页地址不能为空")
			return
		}
		u.b.SetHomePage(home)
		u.persistBrowserPrefs()
		st.homeEditor.SetText(u.b.HomePage())
		u.setStatus("主页已保存: %s", u.b.HomePage())
	}

	if st.qlAddBtn.Clicked(gtx) {
		name := strings.TrimSpace(st.qlNameEditor.Text())
		url := strings.TrimSpace(st.qlURLEditor.Text())
		if _, ok := u.b.AddQuickLink(name, url); !ok {
			u.setStatus("名称与地址均不能为空")
			return
		}
		u.persistBrowserPrefs()
		st.qlNameEditor.SetText("")
		st.qlURLEditor.SetText("")
		u.setStatus("已添加快捷访问: %s", name)
	}

	// 快捷访问删除（按 URL 定位）
	for _, bm := range u.b.Bookmarks() {
		ctl := st.qlDelete[bm.URL]
		if ctl == nil || !ctl.Clicked(gtx) {
			continue
		}
		for i, b2 := range u.b.Bookmarks() {
			if b2.URL == bm.URL && b2.Name == bm.Name {
				if u.b.RemoveQuickLink(i) {
					u.persistBrowserPrefs()
					u.setStatus("已删除: %s", bm.Name)
				}
				break
			}
		}
	}
}

// settingsNavItem 左导航单项定义。
type settingsNavItem struct {
	section settingsSection
	icon    *widget.Icon
	title   string
}

// layoutSettingsNav 渲染左侧导航列。
func (u *UI) layoutSettingsNav(gtx layout.Context) layout.Dimensions {
	items := []settingsNavItem{
		{secGeneral, iconTune, "常规"},
		{secQuickLinks, iconBookmark, "快捷访问"},
		{secUserscripts, iconExtension, "用户脚本"},
		{secExtensions, iconExtensions, "扩展"},
	}

	return layout.Inset{Top: unit.Dp(14), Left: unit.Dp(10), Right: unit.Dp(10)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(u.theme, unit.Sp(16), "设置")
					lbl.Color = CTabActiveText
					return layout.Inset{Left: unit.Dp(8), Bottom: unit.Dp(10)}.Layout(gtx, lbl.Layout)
				}),
			}
			for _, it := range items {
				item := it
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					selected := u.settings.section == item.section
					bg, fg := colorTransparentRow, CTabInactiveText
					if selected {
						bg = CTabActiveBG
						fg = CTabActiveText
					}
					return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx,
						func(gtx layout.Context) layout.Dimensions {
							return iconLabelButton(gtx, u.theme, u.settingsNavClick(item.section),
								item.icon, item.title, bg, fg, unit.Sp(12), 14, 7, 12)
						})
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
}

// settingsNavClick 返回导航项的 Clickable（按 section 复用）。
func (u *UI) settingsNavClick(section settingsSection) *widget.Clickable {
	if u.settings.navClicks == nil {
		u.settings.navClicks = make(map[settingsSection]*widget.Clickable)
	}
	ctl := u.settings.navClicks[section]
	if ctl == nil {
		ctl = &widget.Clickable{}
		u.settings.navClicks[section] = ctl
	}
	return ctl
}

// layoutGeneralPanel 常规设置：主页地址 + 关于。
func (u *UI) layoutGeneralPanel(gtx layout.Context) layout.Dimensions {
	st := &u.settings
	if st.homeEditor.Text() == "" {
		st.homeEditor.SetText(u.b.HomePage())
	}

	items := []layout.Widget{
		// 主页卡片
		func(gtx layout.Context) layout.Dimensions {
			return u.cardBox(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(u.theme, unit.Sp(13), "主页")
						lbl.Color = CTabActiveText
						return lbl.Layout(gtx)
					}),
					layout.Rigid(spacerVertical(4)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(u.theme, unit.Sp(10), "新标签页、回主页与关闭最后一个标签时打开的地址")
						lbl.Color = CEditorHint
						return lbl.Layout(gtx)
					}),
					layout.Rigid(spacerVertical(8)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
								border := widget.Border{Color: CInputBorder, CornerRadius: unit.Dp(6), Width: unit.Dp(1)}
								return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.UniformInset(unit.Dp(7)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										ed := material.Editor(u.theme, &st.homeEditor, "https://...")
										ed.Color = CEditorFG
										ed.HintColor = CEditorHint
										ed.TextSize = unit.Sp(12)
										return ed.Layout(gtx)
									})
								})
							}),
							layout.Rigid(spacer(8)),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return iconLabelButton(gtx, u.theme, &st.saveHomeBtn,
									iconSave, "保存", CAccent, COnAccent, unit.Sp(12), 12, 6, 14)
							}),
						)
					}),
				)
			})
		},
		spacerVertical(12),
		// 关于卡片
		func(gtx layout.Context) layout.Dimensions {
			return u.cardBox(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(u.theme, unit.Sp(13), "关于")
						lbl.Color = CTabActiveText
						return lbl.Layout(gtx)
					}),
					layout.Rigid(spacerVertical(6)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(u.theme, unit.Sp(11),
							"Gio Browser · 基于 Gio UI 与 Microsoft WebView2 的多标签浏览器")
						lbl.Color = CEditorHint
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(u.theme, unit.Sp(11),
							"网页渲染由 WebView2 提供，界面与设置中心由 Gio 直接绘制")
						lbl.Color = CEditorHint
						return lbl.Layout(gtx)
					}),
				)
			})
		},
	}

	generalList := layout.List{Axis: layout.Vertical}
	return generalList.Layout(gtx, len(items), func(gtx layout.Context, i int) layout.Dimensions {
		return items[i](gtx)
	})
}

// layoutQuickLinksPanel 快捷访问管理面板。
func (u *UI) layoutQuickLinksPanel(gtx layout.Context) layout.Dimensions {
	st := &u.settings
	links := u.b.Bookmarks()

	rows := make([]layout.Widget, 0, len(links)+3)
	rows = append(rows,
		func(gtx layout.Context) layout.Dimensions {
			lbl := material.Label(u.theme, unit.Sp(13), "快捷访问")
			lbl.Color = CTabActiveText
			return lbl.Layout(gtx)
		},
		spacerVertical(4),
		func(gtx layout.Context) layout.Dimensions {
			lbl := material.Label(u.theme, unit.Sp(10), "条目显示在快捷访问栏，修改立即保存")
			lbl.Color = CEditorHint
			return lbl.Layout(gtx)
		},
		spacerVertical(8),
	)

	for _, bm := range links {
		bm := bm
		ctl := st.qlDelete[bm.URL]
		if ctl == nil {
			ctl = &widget.Clickable{}
			st.qlDelete[bm.URL] = ctl
		}
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return u.cardBox(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(u.theme, unit.Sp(12), bm.Name)
						lbl.Color = CTabActiveText
						return lbl.Layout(gtx)
					}),
					layout.Rigid(spacer(10)),
					layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(u.theme, unit.Sp(11), bm.URL)
						lbl.Color = CTabInactiveText
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return iconLabelButton(gtx, u.theme, ctl,
							iconDelete, "删除", CBtnBG, CCloseFill, unit.Sp(10), 10, 3, 8)
					}),
				)
			})
		}, spacerVertical(6))
	}

	// 添加行
	rows = append(rows, spacerVertical(8), func(gtx layout.Context) layout.Dimensions {
		return u.cardBox(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						border := widget.Border{Color: CInputBorder, CornerRadius: unit.Dp(6), Width: unit.Dp(1)}
						gtx.Constraints.Min.X = gtx.Dp(unit.Dp(150))
						return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								ed := material.Editor(u.theme, &st.qlNameEditor, "名称")
								ed.Color = CEditorFG
								ed.HintColor = CEditorHint
								ed.TextSize = unit.Sp(12)
								return ed.Layout(gtx)
							})
						})
					})
				}),
				layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
					border := widget.Border{Color: CInputBorder, CornerRadius: unit.Dp(6), Width: unit.Dp(1)}
					return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(u.theme, &st.qlURLEditor, "https:// 地址")
							ed.Color = CEditorFG
							ed.HintColor = CEditorHint
							ed.TextSize = unit.Sp(12)
							return ed.Layout(gtx)
						})
					})
				}),
				layout.Rigid(spacer(8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return iconLabelButton(gtx, u.theme, &st.qlAddBtn,
						iconAdd, "添加", CAccent, COnAccent, unit.Sp(12), 12, 6, 14)
				}),
			)
		})
	})

	// 状态行
	rows = append(rows, spacerVertical(6), func(gtx layout.Context) layout.Dimensions {
		if st.statusMsg == "" {
			return layout.Dimensions{}
		}
		lbl := material.Label(u.theme, unit.Sp(10), st.statusMsg)
		lbl.Color = CAccentGreen
		return lbl.Layout(gtx)
	})

	return st.qlList.Layout(gtx, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
		return rows[i](gtx)
	})
}

// 未选中导航项底色（透明，仅保持命中区域一致）。
var colorTransparentRow = color.NRGBA{}

// spacerVertical 返回固定纵向间距。
func spacerVertical(dp int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Spacer{Height: unit.Dp(dp)}.Layout(gtx)
	}
}

// cardBox 绘制带圆角与深色边框的容器卡片（精确测量子组件高度贴合包装）。
func (u *UI) cardBox(gtx layout.Context, w layout.Widget) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := layout.Inset{
		Top: unit.Dp(10), Bottom: unit.Dp(10),
		Left: unit.Dp(12), Right: unit.Dp(12),
	}.Layout(gtx, w)
	call := macro.Stop()

	r := gtx.Dp(unit.Dp(8))
	rr := clip.RRect{Rect: image.Rectangle{Max: dims.Size}, SE: r, SW: r, NE: r, NW: r}
	stack := rr.Push(gtx.Ops)
	paint.FillShape(gtx.Ops, CToolbarBG, clip.Rect{Max: dims.Size}.Op())
	stack.Pop()
	paint.FillShape(gtx.Ops, CBorder, clip.Stroke{
		Path:  rr.Path(gtx.Ops),
		Width: float32(gtx.Dp(unit.Dp(1))),
	}.Op())

	call.Add(gtx.Ops)
	return dims
}
