package ui

import (
	"fmt"
	"image"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"gio-browser/internal/extension"
)

// extCardCtl 单个扩展卡片的交互控件。
type extCardCtl struct {
	toggle    widget.Clickable
	openPopup widget.Clickable
	delete    widget.Clickable
}

// extensionsUIState 扩展管理面板的 UI 状态。
type extensionsUIState struct {
	list layout.List

	pathEditor widget.Editor
	loadBtn    widget.Clickable
	closeBtn   widget.Clickable

	cards map[string]*extCardCtl

	statusMsg  string
	statusTime time.Time
}

func (u *UI) initExtensionsUIState() {
	u.extUI = extensionsUIState{
		cards: make(map[string]*extCardCtl),
	}
	u.extUI.list.Axis = layout.Vertical
}

// LayoutExtensions 渲染扩展管理面板主体（参考 Edge edge://extensions 的
// 开发者模式形态，v1 以路径输入代替系统文件夹选择对话框）。
func (u *UI) LayoutExtensions(gtx layout.Context) layout.Dimensions {
	u.handleExtensionEvents(gtx)

	paint.FillShape(gtx.Ops, CContentBG, clip.Rect{Max: gtx.Constraints.Max}.Op())

	return layout.Inset{
		Top: unit.Dp(16), Bottom: unit.Dp(16),
		Left: unit.Dp(24), Right: unit.Dp(24),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			maxWidth := gtx.Dp(unit.Dp(760))
			if gtx.Constraints.Max.X > maxWidth {
				gtx.Constraints.Max.X = maxWidth
				gtx.Constraints.Min.X = maxWidth
			}
			return u.layoutExtensionList(gtx)
		})
	})
}

// handleExtensionEvents 处理扩展面板交互事件。
func (u *UI) handleExtensionEvents(gtx layout.Context) {
	st := &u.extUI
	mgr := extension.GetGlobalManager()

	setStatus := func(format string, args ...any) {
		st.statusMsg = fmt.Sprintf(format, args...)
		st.statusTime = time.Now()
	}

	if st.closeBtn.Clicked(gtx) {
		u.showExtensions = false
		u.b.SetSettingsOpen(false)
		return
	}

	if st.loadBtn.Clicked(gtx) {
		dir := strings.TrimSpace(st.pathEditor.Text())
		if dir == "" {
			setStatus("请先输入扩展目录路径")
			return
		}
		e, err := mgr.LoadUnpacked(dir)
		if err != nil {
			setStatus("加载失败: %v", err)
			return
		}
		st.pathEditor.SetText("")
		setStatus("已加载扩展: %s v%s", e.Manifest.Name, e.Manifest.Version)
	}

	for _, e := range mgr.List() {
		ctl := st.cards[e.ID]
		if ctl == nil {
			ctl = &extCardCtl{}
			st.cards[e.ID] = ctl
		}

		if ctl.toggle.Clicked(gtx) {
			enabled, _ := mgr.Toggle(e.ID)
			state := "已启用"
			if !enabled {
				state = "已停用"
			}
			setStatus("扩展 [%s] %s", e.Manifest.Name, state)
		}

		if ctl.openPopup.Clicked(gtx) {
			if popup := e.PopupURL(); popup != "" {
				u.b.CreateTab(popup, e.Manifest.Name)
			}
		}

		if ctl.delete.Clicked(gtx) {
			_ = mgr.Delete(e.ID)
			delete(st.cards, e.ID)
			setStatus("已移除扩展: %s（源目录未删除）", e.Manifest.Name)
		}
	}
}

// layoutExtensionList 渲染扩展列表视图。
func (u *UI) layoutExtensionList(gtx layout.Context) layout.Dimensions {
	exts := extension.GetGlobalManager().List()

	items := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return u.layoutExtensionHeader(gtx)
		},
		spacerVertical(12),
	}

	for _, e := range exts {
		ext := e
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return u.layoutExtensionCard(gtx, ext)
		}, spacerVertical(10))
	}

	return u.extUI.list.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
		return items[index](gtx)
	})
}

// layoutExtensionHeader 面板标题栏与加载操作。
func (u *UI) layoutExtensionHeader(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// 标题行
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return glyph(gtx, iconExtensions, 20, CAccent)
				}),
				layout.Rigid(spacer(10)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							lbl := material.Label(u.theme, unit.Sp(17), "扩展管理 · 开发者模式")
							lbl.Color = CTabActiveText
							return lbl.Layout(gtx)
						}),
						layout.Rigid(spacerVertical(2)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							lbl := material.Label(u.theme, unit.Sp(11), "加载已解压的 Chrome/Edge 扩展目录，content_scripts 自动注入匹配页面")
							lbl.Color = CEditorHint
							return lbl.Layout(gtx)
						}),
					)
				}),
				layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Point{X: gtx.Constraints.Max.X, Y: 0}}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return iconLabelButton(gtx, u.theme, &u.extUI.closeBtn,
						iconClose, "返回网页",
						CBtnBG, CBtnFG, unit.Sp(13), 12, 6, 14)
				}),
			)
		}),
		layout.Rigid(spacerVertical(10)),
		// 路径输入 + 加载按钮行
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
					border := widget.Border{
						Color:        CInputBorder,
						CornerRadius: unit.Dp(6),
						Width:        unit.Dp(1),
					}
					return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(7)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(u.theme, &u.extUI.pathEditor, "输入已解压扩展的目录路径，如 D:\\extensions\\my-ext")
							ed.Color = CEditorFG
							ed.HintColor = CEditorHint
							return ed.Layout(gtx)
						})
					})
				}),
				layout.Rigid(spacer(8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return iconLabelButton(gtx, u.theme, &u.extUI.loadBtn,
						iconAdd, "加载扩展",
						CAccent, COnAccent, unit.Sp(13), 13, 7, 14)
				}),
			)
		}),
		layout.Rigid(spacerVertical(4)),
		// 状态消息行
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if u.extUI.statusMsg == "" {
				return layout.Dimensions{}
			}
			lbl := material.Label(u.theme, unit.Sp(11), u.extUI.statusMsg)
			lbl.Color = CEditorHint
			return lbl.Layout(gtx)
		}),
	)
}

// layoutExtensionCard 渲染单个扩展卡片。
func (u *UI) layoutExtensionCard(gtx layout.Context, e *extension.Extension) layout.Dimensions {
	ctl := u.extUI.cards[e.ID]
	if ctl == nil {
		ctl = &extCardCtl{}
		u.extUI.cards[e.ID] = ctl
	}

	return u.cardBox(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// 第一行：名称 + 版本 + MV 徽标 + 启停状态
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return glyph(gtx, iconExtensions, 15, CAccent)
					}),
					layout.Rigid(spacer(6)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(u.theme, unit.Sp(13), e.Manifest.Name)
						lbl.Color = CTabActiveText
						return lbl.Layout(gtx)
					}),
					layout.Rigid(spacer(8)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.badgePill(gtx, "v"+e.Manifest.Version, CBtnBG, CTabInactiveText)
					}),
					layout.Rigid(spacer(6)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.badgePill(gtx, fmt.Sprintf("MV%d", e.Manifest.ManifestVersion), CBtnBG, CTabInactiveText)
					}),
					layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Point{X: gtx.Constraints.Max.X, Y: 0}}
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						statusText, statusColor := "已启用", CAccentGreen
						if !e.Enabled {
							statusText, statusColor = "已停用", CTabInactiveText
						}
						lbl := material.Label(u.theme, unit.Sp(11), statusText)
						lbl.Color = statusColor
						return lbl.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(spacerVertical(6)),

			// 描述
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				desc := e.Manifest.Description
				if desc == "" {
					desc = "无描述"
				}
				lbl := material.Label(u.theme, unit.Sp(11), desc)
				lbl.Color = CEditorHint
				return lbl.Layout(gtx)
			}),
			layout.Rigid(spacerVertical(6)),

			// 权限与匹配范围
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				matchCount := 0
				for _, cs := range e.Manifest.ContentScripts {
					matchCount += len(cs.JS) + len(cs.CSS)
				}
				perm := strings.Join(e.Manifest.Permissions, ", ")
				if perm == "" {
					perm = "无"
				}
				lbl := material.Label(u.theme, unit.Sp(10),
					fmt.Sprintf("权限: %s · 注入文件: %d · ID: %s", perm, matchCount, e.ID))
				lbl.Color = CTabInactiveText
				return lbl.Layout(gtx)
			}),
			layout.Rigid(spacerVertical(8)),

			// 操作按钮行
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Point{X: gtx.Constraints.Max.X, Y: 0}}
					}),
					// 启停
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btnText, btnBg := "停用", CBtnBG
						if !e.Enabled {
							btnText, btnBg = "启用", CAccent
						}
						btn := material.Button(u.theme, &ctl.toggle, btnText)
						btn.Background = btnBg
						btn.Color = COnAccent
						btn.TextSize = unit.Sp(11)
						btn.Inset = layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(10), Right: unit.Dp(10)}
						return btn.Layout(gtx)
					}),
					layout.Rigid(spacer(6)),
					// 打开 popup（配置了 default_popup 才展示）
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if e.PopupURL() == "" {
							return layout.Dimensions{}
						}
						return iconLabelButton(gtx, u.theme, &ctl.openPopup,
							iconPlay, "打开弹窗",
							CBtnBG, CBtnFG, unit.Sp(11), 11, 4, 10)
					}),
					layout.Rigid(spacer(6)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return iconLabelButton(gtx, u.theme, &ctl.delete,
							iconDelete, "移除",
							CBtnBG, CCloseFill, unit.Sp(11), 11, 4, 8)
					}),
				)
			}),
		)
	})
}
