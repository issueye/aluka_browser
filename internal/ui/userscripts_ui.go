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

	"gio-browser/internal/userscript"
)

type usViewMode int

const (
	usViewList usViewMode = iota
	usViewEditor
)

// scriptCardCtl 单个脚本卡片的交互控件。
type scriptCardCtl struct {
	toggle widget.Clickable
	edit   widget.Clickable
	delete widget.Clickable
}

// userscriptUIState 用户脚本管理面板的 UI 状态。
type userscriptUIState struct {
	mode usViewMode
	list layout.List

	// 顶部操作
	newBtn      widget.Clickable
	closeBtn    widget.Clickable
	backListBtn widget.Clickable

	// 脚本列表控件
	cards map[string]*scriptCardCtl

	// 编辑器状态
	editingID   string
	codeEditor  widget.Editor
	saveCodeBtn widget.Clickable

	statusMsg  string
	statusTime time.Time
}

func (u *UI) initUserscriptsUIState() {
	u.usUI = userscriptUIState{
		mode:  usViewList,
		cards: make(map[string]*scriptCardCtl),
	}
	u.usUI.list.Axis = layout.Vertical
}

// LayoutUserscripts 渲染篡改猴（用户脚本）管理面板主体。
func (u *UI) LayoutUserscripts(gtx layout.Context) layout.Dimensions {
	u.handleUserscriptEvents(gtx)

	// 全屏深色背景
	paint.FillShape(gtx.Ops, CContentBG, clip.Rect{Max: gtx.Constraints.Max}.Op())

	return layout.Inset{
		Top: unit.Dp(16), Bottom: unit.Dp(16),
		Left: unit.Dp(24), Right: unit.Dp(24),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			maxWidth := gtx.Dp(unit.Dp(720))
			if gtx.Constraints.Max.X > maxWidth {
				gtx.Constraints.Max.X = maxWidth
				gtx.Constraints.Min.X = maxWidth
			}

			if u.usUI.mode == usViewEditor {
				return u.layoutUserscriptEditor(gtx)
			}
			return u.layoutUserscriptList(gtx)
		})
	})
}

// handleUserscriptEvents 处理用户脚本面板交互事件。
func (u *UI) handleUserscriptEvents(gtx layout.Context) {
	st := &u.usUI
	mgr := userscript.GetGlobalManager()

	if st.closeBtn.Clicked(gtx) {
		u.showUserscripts = false
		u.b.SetSettingsOpen(false)
		return
	}

	if st.newBtn.Clicked(gtx) {
		st.mode = usViewEditor
		st.editingID = ""
		st.codeEditor.SetText(userscript.DefaultScriptTemplate)
		return
	}

	if st.backListBtn.Clicked(gtx) {
		st.mode = usViewList
		return
	}

	if st.saveCodeBtn.Clicked(gtx) {
		code := st.codeEditor.Text()
		var s *userscript.Script
		if st.editingID != "" {
			if existing, ok := mgr.Get(st.editingID); ok {
				s = existing
				s.UpdateCode(code)
			}
		}
		if s == nil {
			s = userscript.NewScript(code, true)
		}

		if err := mgr.AddOrUpdate(s); err != nil {
			st.statusMsg = "保存失败: " + err.Error()
		} else {
			st.statusMsg = fmt.Sprintf("已成功保存脚本: %s", s.Meta.Name)
			st.mode = usViewList
		}
		st.statusTime = time.Now()
		return
	}

	// 处理卡片事件
	scripts := mgr.List()
	for _, s := range scripts {
		ctl := st.cards[s.ID]
		if ctl == nil {
			ctl = &scriptCardCtl{}
			st.cards[s.ID] = ctl
		}

		if ctl.toggle.Clicked(gtx) {
			enabled, _ := mgr.Toggle(s.ID)
			stateStr := "已启用"
			if !enabled {
				stateStr = "已停用"
			}
			st.statusMsg = fmt.Sprintf("脚本 [%s] %s", s.Meta.Name, stateStr)
			st.statusTime = time.Now()
		}

		if ctl.edit.Clicked(gtx) {
			st.mode = usViewEditor
			st.editingID = s.ID
			st.codeEditor.SetText(s.Code)
		}

		if ctl.delete.Clicked(gtx) {
			_ = mgr.Delete(s.ID)
			delete(st.cards, s.ID)
			st.statusMsg = fmt.Sprintf("已删除脚本: %s", s.Meta.Name)
			st.statusTime = time.Now()
		}
	}
}

// layoutUserscriptList 渲染用户脚本列表视图。
func (u *UI) layoutUserscriptList(gtx layout.Context) layout.Dimensions {
	mgr := userscript.GetGlobalManager()
	scripts := mgr.List()

	items := []layout.Widget{
		// 1. 顶部 Header
		func(gtx layout.Context) layout.Dimensions {
			return u.layoutUserscriptHeader(gtx)
		},
		spacerVertical(14),
	}

	// 2. 脚本卡片列表
	for _, scr := range scripts {
		s := scr
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return u.layoutScriptCard(gtx, s)
		}, spacerVertical(10))
	}

	return u.usUI.list.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
		return items[index](gtx)
	})
}

// layoutUserscriptHeader 列表顶部标题栏与新建操作。
func (u *UI) layoutUserscriptHeader(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return glyph(gtx, iconExtension, 20, CAccent)
				}),
				layout.Rigid(spacer(10)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							lbl := material.Label(u.theme, unit.Sp(17), "篡改猴 · 用户脚本管理中心")
							lbl.Color = CTabActiveText
							return lbl.Layout(gtx)
						}),
						layout.Rigid(spacerVertical(2)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							lbl := material.Label(u.theme, unit.Sp(11), "在匹配的网页中注入执行自定义 JavaScript，扩展网页功能与去限制")
							lbl.Color = CEditorHint
							return lbl.Layout(gtx)
						}),
					)
				}),
			)
		}),
		layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Point{X: gtx.Constraints.Max.X, Y: 0}}
		}),
		// ➕ 新建脚本按钮
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(u.theme, &u.usUI.newBtn, "➕ 新建脚本")
			btn.Background = CAccent
			btn.Color = COnAccent
			btn.Inset = layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(12), Right: unit.Dp(12)}
			return btn.Layout(gtx)
		}),
		layout.Rigid(spacer(8)),
		// 返回网页按钮
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(u.theme, &u.usUI.closeBtn, "返回网页 ✕")
			btn.Background = CBtnBG
			btn.Color = CBtnFG
			btn.Inset = layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(12), Right: unit.Dp(12)}
			return btn.Layout(gtx)
		}),
	)
}

// layoutScriptCard 渲染单个用户脚本卡片。
func (u *UI) layoutScriptCard(gtx layout.Context, s *userscript.Script) layout.Dimensions {
	ctl := u.usUI.cards[s.ID]
	if ctl == nil {
		ctl = &scriptCardCtl{}
		u.usUI.cards[s.ID] = ctl
	}

	return u.cardBox(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// 1. 第一行：标题 + 版本 Badge + 作者 + 状态指示
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return glyph(gtx, iconCode, 15, CAccent)
					}),
					layout.Rigid(spacer(6)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(u.theme, unit.Sp(13), s.Meta.Name)
						lbl.Color = CTabActiveText
						return lbl.Layout(gtx)
					}),
					layout.Rigid(spacer(8)),
					// 版本胶囊
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.badgePill(gtx, "v"+s.Meta.Version, CBtnBG, CTabInactiveText)
					}),
					layout.Rigid(spacer(6)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(u.theme, unit.Sp(10), "by "+s.Meta.Author)
						lbl.Color = CEditorHint
						return lbl.Layout(gtx)
					}),
					layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Point{X: gtx.Constraints.Max.X, Y: 0}}
					}),
					// 状态文案
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						statusText := "已启用"
						statusColor := CAccentGreen
						if !s.Enabled {
							statusText = "已停用"
							statusColor = CTabInactiveText
						}
						lbl := material.Label(u.theme, unit.Sp(11), statusText)
						lbl.Color = statusColor
						return lbl.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(spacerVertical(6)),

			// 2. 描述文案
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				desc := s.Meta.Description
				if desc == "" {
					desc = "无描述"
				}
				lbl := material.Label(u.theme, unit.Sp(11), desc)
				lbl.Color = CEditorHint
				return lbl.Layout(gtx)
			}),
			layout.Rigid(spacerVertical(8)),

			// 3. 匹配范围与操作按钮栏
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				matchStr := strings.Join(s.Meta.Match, ", ")
				if matchStr == "" {
					matchStr = "*://*/*"
				}

				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(u.theme, unit.Sp(10), "生效站点: "+matchStr)
						lbl.Color = CTabInactiveText
						return lbl.Layout(gtx)
					}),
					layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Point{X: gtx.Constraints.Max.X, Y: 0}}
					}),
					// 启停切换按钮
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btnText := "停用"
						btnBg := CBtnBG
						if !s.Enabled {
							btnText = "启用"
							btnBg = CAccent
						}
						btn := material.Button(u.theme, &ctl.toggle, btnText)
						btn.Background = btnBg
						btn.Color = COnAccent
						btn.TextSize = unit.Sp(11)
						btn.Inset = layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(10), Right: unit.Dp(10)}
						return btn.Layout(gtx)
					}),
					layout.Rigid(spacer(6)),
					// 编辑按钮
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(u.theme, &ctl.edit, "编辑 ✏️")
						btn.Background = CBtnBG
						btn.Color = CBtnFG
						btn.TextSize = unit.Sp(11)
						btn.Inset = layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(10), Right: unit.Dp(10)}
						return btn.Layout(gtx)
					}),
					layout.Rigid(spacer(6)),
					// 删除按钮
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(u.theme, &ctl.delete, "删除 🗑️")
						btn.Background = CBtnBG
						btn.Color = CCloseFill
						btn.TextSize = unit.Sp(11)
						btn.Inset = layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(8), Right: unit.Dp(8)}
						return btn.Layout(gtx)
					}),
				)
			}),
		)
	})
}

// layoutUserscriptEditor 渲染在线代码编辑器视图。
func (u *UI) layoutUserscriptEditor(gtx layout.Context) layout.Dimensions {
	title := "新建用户脚本"
	if u.usUI.editingID != "" {
		if s, ok := userscript.GetGlobalManager().Get(u.usUI.editingID); ok {
			title = "编辑用户脚本: " + s.Meta.Name
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// 1. 顶部操作栏
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return glyph(gtx, iconCode, 18, CAccent)
				}),
				layout.Rigid(spacer(8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(u.theme, unit.Sp(15), title)
					lbl.Color = CTabActiveText
					return lbl.Layout(gtx)
				}),
				layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Point{X: gtx.Constraints.Max.X, Y: 0}}
				}),
				// 保存按钮
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(u.theme, &u.usUI.saveCodeBtn, "💾 保存脚本")
					btn.Background = CAccent
					btn.Color = COnAccent
					btn.Inset = layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(14), Right: unit.Dp(14)}
					return btn.Layout(gtx)
				}),
				layout.Rigid(spacer(8)),
				// 取消返回列表
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(u.theme, &u.usUI.backListBtn, "取消")
					btn.Background = CBtnBG
					btn.Color = CBtnFG
					btn.Inset = layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(12), Right: unit.Dp(12)}
					return btn.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(spacerVertical(12)),

		// 2. 代码编辑器卡片容器
		layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
			border := widget.Border{
				Color:        CInputBorder,
				CornerRadius: unit.Dp(8),
				Width:        unit.Dp(1),
			}
			return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						paint.FillShape(gtx.Ops, CToolbarBG, clip.Rect{Max: gtx.Constraints.Max}.Op())
						return layout.Dimensions{Size: gtx.Constraints.Max}
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(u.theme, &u.usUI.codeEditor, "请输入 JavaScript 用户脚本代码...")
							ed.Color = CEditorFG
							ed.HintColor = CEditorHint
							ed.TextSize = unit.Sp(12)
							return ed.Layout(gtx)
						})
					}),
				)
			})
		}),
	)
}

// badgePill 绘制小型状态/版本胶囊。
func (u *UI) badgePill(gtx layout.Context, text string, bg, fg color.NRGBA) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := layout.Inset{
		Top: unit.Dp(2), Bottom: unit.Dp(2),
		Left: unit.Dp(6), Right: unit.Dp(6),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		lbl := material.Label(u.theme, unit.Sp(9), text)
		lbl.Color = fg
		return lbl.Layout(gtx)
	})
	call := macro.Stop()

	radius := gtx.Dp(unit.Dp(4))
	rrect := clip.RRect{
		Rect: image.Rectangle{Max: dims.Size},
		SE:   radius, SW: radius, NE: radius, NW: radius,
	}
	stack := rrect.Push(gtx.Ops)
	paint.FillShape(gtx.Ops, bg, clip.Rect{Max: dims.Size}.Op())
	stack.Pop()

	call.Add(gtx.Ops)
	return dims
}

