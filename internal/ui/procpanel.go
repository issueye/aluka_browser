package ui

import (
	"fmt"
	"image"
	"image/color"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"gio-browser/internal/procs"
	"gio-browser/internal/win32"
)

// procPanelState 进程管理浮窗的交互状态。
type procPanelState struct {
	list   layout.List
	filter widget.Editor
	kill   map[uint32]*widget.Clickable
}

// RunProcessPanel 以独立置顶浮窗运行进程管理器（阻塞至浮窗关闭）。
//
// 展示全系统进程树：父子缩进、内存占用、浏览器自身子树高亮；
// 仅浏览器子树内的进程（自身进程除外）提供"结束"操作，
// 与 Chrome 任务管理器的安全边界一致。
// 由装配层在独立 goroutine 中调用；同一时刻仅应存在一个实例。
func RunProcessPanel(title string) {
	w := new(app.Window)
	w.Option(
		app.Title(title),
		app.Size(unit.Dp(800), unit.Dp(620)),
		app.MinSize(unit.Dp(560), unit.Dp(420)),
	)

	th := material.NewTheme()
	st := procPanelState{kill: make(map[uint32]*widget.Clickable)}
	st.list.Axis = layout.Vertical

	// 异步探测浮窗 HWND 并置顶（浮窗语义）
	go makeTopmostByTitle(title)

	selfPID := win32.CurrentPID()

	var ops op.Ops
	var (
		rows         []*procs.Node // 过滤后的展平行
		totalCount   int           // 过滤前进程总数
		browserMem   uint64        // 浏览器子树内存合计
		lastSnap     time.Time
		lastFilter   string
		statusMsg    string
		statusSticky string // 常驻说明（结束进程保护边界）
	)
	statusSticky = "仅浏览器自身子树内可结束进程（当前进程除外）"

	refresh := func() {
		tree := procs.BuildTree(procs.Snapshot(), selfPID)
		all := procs.Flatten(tree)
		totalCount = len(all)
		browserMem = 0
		for _, n := range all {
			if n.InBrowserSubtree {
				browserMem += n.Info.WorkingSet
			}
		}
		rows = procs.Flatten(procs.FilterTree(tree, st.filter.Text()))
		lastSnap = time.Now()
		lastFilter = st.filter.Text()
	}

	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return

		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)

			// 自动刷新（2s）或过滤条件变化时重建快照
			if time.Since(lastSnap) > 2*time.Second || st.filter.Text() != lastFilter {
				refresh()
			}

			// 处理"结束进程"点击
			for _, n := range rows {
				if !killable(n, selfPID) {
					continue
				}
				ctl := st.kill[n.Info.PID]
				if ctl != nil && ctl.Clicked(gtx) {
					if err := win32.TerminateProcess(n.Info.PID); err != nil {
						statusMsg = "结束失败: " + err.Error()
					} else {
						statusMsg = fmt.Sprintf("已结束进程 %s (PID %d)", n.Info.Name, n.Info.PID)
					}
					lastSnap = time.Time{} // 触发立即刷新
				}
			}

			// ---- 布局 ----
			paint.FillShape(gtx.Ops, CContentBG, clip.Rect{Max: gtx.Constraints.Max}.Op())

			layout.Inset{
				Top: unit.Dp(12), Bottom: unit.Dp(10),
				Left: unit.Dp(14), Right: unit.Dp(14),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layoutProcPanelHeader(gtx, th, &st, totalCount, browserMem, statusMsg, statusSticky)
					}),
					layout.Rigid(spacerVertical(8)),
					layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
						return st.list.Layout(gtx, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
							n := rows[i]
							return layoutProcRow(gtx, th, st, n, selfPID)
						})
					}),
					layout.Rigid(spacerVertical(4)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(th, unit.Sp(10),
							fmt.Sprintf("刷新于 %s · 共 %d 个进程 · 浏览器子树内存 %s",
								lastSnap.Format("15:04:05"), totalCount, procs.FormatBytes(browserMem)))
						lbl.Color = CEditorHint
						return lbl.Layout(gtx)
					}),
				)
			})

			e.Frame(gtx.Ops)
		}
	}
}

// killable 判断进程是否允许结束：浏览器子树内且不是自身进程。
func killable(n *procs.Node, selfPID uint32) bool {
	return n.InBrowserSubtree && n.Info.PID != selfPID
}

// layoutProcPanelHeader 渲染浮窗标题、过滤框与状态区。
func layoutProcPanelHeader(gtx layout.Context, th *material.Theme, st *procPanelState,
	total int, browserMem uint64, statusMsg, statusSticky string,
) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// 标题行
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return glyph(gtx, iconProc, 18, CAccent)
				}),
				layout.Rigid(spacer(8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(th, unit.Sp(15), "进程管理器")
					lbl.Color = CTabActiveText
					return lbl.Layout(gtx)
				}),
				layout.Rigid(spacer(8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(th, unit.Sp(11),
						fmt.Sprintf("浏览器子树内存 %s", procs.FormatBytes(browserMem)))
					lbl.Color = CAccentGreen
					return lbl.Layout(gtx)
				}),
				layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Point{X: gtx.Constraints.Max.X, Y: 0}}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(th, unit.Sp(10), "每 2s 自动刷新")
					lbl.Color = CEditorHint
					return lbl.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(spacerVertical(8)),
		// 过滤行
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			border := widget.Border{
				Color:        CInputBorder,
				CornerRadius: unit.Dp(6),
				Width:        unit.Dp(1),
			}
			return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(th, &st.filter, "按进程名过滤，如 msedgewebview2")
					ed.Color = CEditorFG
					ed.HintColor = CEditorHint
					ed.TextSize = unit.Sp(12)
					return ed.Layout(gtx)
				})
			})
		}),
		layout.Rigid(spacerVertical(6)),
		// 状态行
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			msg := statusSticky
			color := CEditorHint
			if statusMsg != "" {
				msg = statusMsg
				color = CAccentGreen
			}
			lbl := material.Label(th, unit.Sp(10), msg)
			lbl.Color = color
			return lbl.Layout(gtx)
		}),
	)
}

// layoutProcRow 渲染单行进程（缩进 + 名称 + PID + 内存 + 结束按钮）。
func layoutProcRow(gtx layout.Context, th *material.Theme, st procPanelState,
	n *procs.Node, selfPID uint32,
) layout.Dimensions {
	indent := unit.Dp(14 * min(n.Depth, 8))

	return layout.Inset{Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			macro := op.Record(gtx.Ops)
			dims := layout.Inset{Left: indent}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					// 树形连线提示
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if n.Depth == 0 {
							return layout.Dimensions{}
						}
						return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Label(th, unit.Sp(9), "└")
							lbl.Color = CTabInactiveText
							return lbl.Layout(gtx)
						})
					}),
					// 进程名
					layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Label(th, unit.Sp(12), n.Info.Name)
								if n.InBrowserSubtree {
									lbl.Color = CTabActiveText
								} else {
									lbl.Color = CTabInactiveText
								}
								return lbl.Layout(gtx)
							}),
							layout.Rigid(spacer(6)),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if !n.InBrowserSubtree {
									return layout.Dimensions{}
								}
								if n.Info.PID == selfPID {
									lbl := material.Label(th, unit.Sp(9), "本程序")
									lbl.Color = CAccentGreen
									return lbl.Layout(gtx)
								}
								lbl := material.Label(th, unit.Sp(9), "浏览器")
								lbl.Color = CAccent
								return lbl.Layout(gtx)
							}),
						)
					}),
					// PID
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(th, unit.Sp(10), fmt.Sprintf("%d", n.Info.PID))
						lbl.Color = CTabInactiveText
						return lbl.Layout(gtx)
					}),
					// 内存
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(th, unit.Sp(10), procs.FormatBytes(n.Info.WorkingSet))
						lbl.Color = CSubBtnFG
						return lbl.Layout(gtx)
					}),
					// 结束按钮
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !killable(n, selfPID) {
							return layout.Dimensions{Size: image.Point{X: gtx.Dp(unit.Dp(52)), Y: 0}}
						}
						ctl := st.kill[n.Info.PID]
						if ctl == nil {
							ctl = &widget.Clickable{}
							st.kill[n.Info.PID] = ctl
						}
						btn := material.Button(th, ctl, "结束")
						btn.Background = CBtnBG
						btn.Color = CCloseFill
						btn.TextSize = unit.Sp(10)
						btn.Inset = layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Left: unit.Dp(8), Right: unit.Dp(8)}
						return btn.Layout(gtx)
					}),
				)
			})
			call := macro.Stop()

			if n.InBrowserSubtree {
				paint.FillShape(gtx.Ops, colorBrowserRow, clip.Rect{Max: dims.Size}.Op())
			}
			call.Add(gtx.Ops)
			return dims
		})
}

// makeTopmostByTitle 按标题子串轮询查找本进程窗口并置顶（浮窗用）。
func makeTopmostByTitle(title string) {
	for i := 0; i < 40; i++ {
		time.Sleep(200 * time.Millisecond)
		if hwnd := win32.FindWindowByTitle(title); hwnd != 0 {
			win32.SetTopmost(hwnd, true)
			return
		}
	}
}

// colorBrowserRow 浏览器子树行的行底色。
var colorBrowserRow = color.NRGBA{R: 30, G: 38, B: 58, A: 255}
