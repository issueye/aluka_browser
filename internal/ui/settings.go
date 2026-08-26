package ui

import (
	"image"
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

// settingsState 保存设置页面的交互控件与草稿状态。
type settingsState struct {
	list layout.List

	proxyEnabled bool
	proxyType    string // "http" 或 "socks5"

	serverEditor widget.Editor
	bypassEditor widget.Editor

	directModeBtn widget.Clickable
	proxyModeBtn  widget.Clickable

	protoHTTPBtn   widget.Clickable
	protoSocks5Btn widget.Clickable

	saveBtn  widget.Clickable
	closeBtn widget.Clickable
	resetBtn widget.Clickable

	statusMsg  string
	statusTime time.Time
}

// initSettingsState 初始化或重置设置页控件状态。
func (u *UI) initSettingsState() {
	cfg := config.Current()
	u.settings = settingsState{
		proxyEnabled: cfg.ProxyEnabled,
		proxyType:    cfg.ProxyType,
	}
	u.settings.list.Axis = layout.Vertical

	u.settings.serverEditor.SingleLine = true
	u.settings.serverEditor.SetText(cfg.ProxyServer)

	u.settings.bypassEditor.SingleLine = true
	u.settings.bypassEditor.SetText(cfg.ProxyBypass)
}

// LayoutSettings 渲染设置页面主体。
func (u *UI) LayoutSettings(gtx layout.Context) layout.Dimensions {
	u.handleSettingsEvents(gtx)

	// 背景全屏深色铺底
	paint.FillShape(gtx.Ops, CContentBG, clip.Rect{Max: gtx.Constraints.Max}.Op())

	return layout.Inset{
		Top: unit.Dp(16), Bottom: unit.Dp(16),
		Left: unit.Dp(24), Right: unit.Dp(24),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			maxWidth := gtx.Dp(unit.Dp(680))
			if gtx.Constraints.Max.X > maxWidth {
				gtx.Constraints.Max.X = maxWidth
				gtx.Constraints.Min.X = maxWidth
			}

			// 使用 List 支持自适应纵向滚动
			items := []layout.Widget{
				// 1. 顶部标题栏
				func(gtx layout.Context) layout.Dimensions {
					return u.layoutSettingsHeader(gtx)
				},
				spacerVertical(16),
				// 2. 代理设置卡片
				func(gtx layout.Context) layout.Dimensions {
					return u.layoutProxyCard(gtx)
				},
				spacerVertical(16),
				// 3. 关于卡片
				func(gtx layout.Context) layout.Dimensions {
					return u.layoutAboutCard(gtx)
				},
			}

			return u.settings.list.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
				return items[index](gtx)
			})
		})
	})
}

// handleSettingsEvents 处理设置页面的交互事件。
func (u *UI) handleSettingsEvents(gtx layout.Context) {
	st := &u.settings

	if st.closeBtn.Clicked(gtx) {
		u.showSettings = false
		u.b.SetSettingsOpen(false)
		return
	}

	if st.directModeBtn.Clicked(gtx) {
		st.proxyEnabled = false
	}
	if st.proxyModeBtn.Clicked(gtx) {
		st.proxyEnabled = true
	}

	if st.protoHTTPBtn.Clicked(gtx) {
		st.proxyType = "http"
	}
	if st.protoSocks5Btn.Clicked(gtx) {
		st.proxyType = "socks5"
	}

	if st.resetBtn.Clicked(gtx) {
		u.initSettingsState()
		st.statusMsg = "已重置为当前生效的配置"
		st.statusTime = time.Now()
	}

	if st.saveBtn.Clicked(gtx) {
		newCfg := config.Config{
			ProxyEnabled: st.proxyEnabled,
			ProxyType:    st.proxyType,
			ProxyServer:  strings.TrimSpace(st.serverEditor.Text()),
			ProxyBypass:  strings.TrimSpace(st.bypassEditor.Text()),
		}
		if err := config.Save(newCfg); err != nil {
			st.statusMsg = "保存失败: " + err.Error()
		} else {
			st.statusMsg = "代理配置已保存并应用！新打开标签页与重启后完全生效"
		}
		st.statusTime = time.Now()
	}
}

// layoutSettingsHeader 设置页顶部标题栏与返回按钮。
func (u *UI) layoutSettingsHeader(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return glyph(gtx, iconSettings, 20, CAccent)
				}),
				layout.Rigid(spacer(10)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							lbl := material.Label(u.theme, unit.Sp(17), "浏览器设置")
							lbl.Color = CTabActiveText
							return lbl.Layout(gtx)
						}),
						layout.Rigid(spacerVertical(2)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							lbl := material.Label(u.theme, unit.Sp(11), "管理网络访问代理与运行环境配置")
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
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(u.theme, &u.settings.closeBtn, "返回网页 ✕")
			btn.Background = CBtnBG
			btn.Color = CBtnFG
			btn.Inset = layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(14), Right: unit.Dp(14)}
			return btn.Layout(gtx)
		}),
	)
}

// layoutProxyCard 渲染网络访问代理配置卡片。
func (u *UI) layoutProxyCard(gtx layout.Context) layout.Dimensions {
	return u.cardBox(gtx, func(gtx layout.Context) layout.Dimensions {
		st := &u.settings
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// 卡片主标题
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return glyph(gtx, iconTune, 15, CAccent)
					}),
					layout.Rigid(spacer(8)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(u.theme, unit.Sp(13), "网络访问代理配置 (Proxy Settings)")
						lbl.Color = CTabActiveText
						return lbl.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(spacerVertical(14)),

			// 模式切换单选按钮组
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.choicePill(gtx, &st.directModeBtn, "直连模式 (不使用代理)", !st.proxyEnabled)
					}),
					layout.Rigid(spacer(10)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.choicePill(gtx, &st.proxyModeBtn, "启用网络代理", st.proxyEnabled)
					}),
				)
			}),
			layout.Rigid(spacerVertical(14)),

			// 如果启用了代理，展示详细配置字段
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !st.proxyEnabled {
					lbl := material.Label(u.theme, unit.Sp(12), "当前处于直连模式，网页请求直接连接目标服务器。")
					lbl.Color = CEditorHint
					return lbl.Layout(gtx)
				}

				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					// 协议选择
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Label(u.theme, unit.Sp(12), "代理协议类型：")
								lbl.Color = CTabInactiveText
								return lbl.Layout(gtx)
							}),
							layout.Rigid(spacer(8)),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return u.choicePill(gtx, &st.protoHTTPBtn, "HTTP / HTTPS", st.proxyType != "socks5")
							}),
							layout.Rigid(spacer(8)),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return u.choicePill(gtx, &st.protoSocks5Btn, "SOCKS5", st.proxyType == "socks5")
							}),
						)
					}),
					layout.Rigid(spacerVertical(12)),

					// 服务器地址输入框
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.layoutInputField(gtx, "代理服务器地址与端口", "例如 127.0.0.1:7890 或 proxy.example.com:8080", &st.serverEditor)
					}),
					layout.Rigid(spacerVertical(12)),

					// 不走代理名单输入框
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.layoutInputField(gtx, "不代理白名单 (Bypass List)", "<local>;localhost;127.0.0.1 (多个以分号 ; 分隔)", &st.bypassEditor)
					}),
				)
			}),
			layout.Rigid(spacerVertical(16)),

			// 底部操作按钮与状态提示
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(u.theme, &st.saveBtn, "保存并应用配置")
						btn.Background = CAccent
						btn.Color = COnAccent
						btn.Inset = layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(18), Right: unit.Dp(18)}
						return btn.Layout(gtx)
					}),
					layout.Rigid(spacer(10)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(u.theme, &st.resetBtn, "重置")
						btn.Background = CBtnBG
						btn.Color = CBtnFG
						btn.Inset = layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(14), Right: unit.Dp(14)}
						return btn.Layout(gtx)
					}),
					layout.Rigid(spacer(14)),
					// 状态反馈文案
					layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
						if st.statusMsg == "" {
							return layout.Dimensions{}
						}
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return glyph(gtx, iconDone, 13, CAccentGreen)
							}),
							layout.Rigid(spacer(5)),
							layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Label(u.theme, unit.Sp(11), st.statusMsg)
								lbl.Color = CAccentGreen
								return lbl.Layout(gtx)
							}),
						)
					}),
				)
			}),
		)
	})
}

// layoutAboutCard 渲染系统与关于信息卡片。
func (u *UI) layoutAboutCard(gtx layout.Context) layout.Dimensions {
	return u.cardBox(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return glyph(gtx, iconInfo, 14, CTabActiveText)
					}),
					layout.Rigid(spacer(8)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(u.theme, unit.Sp(13), "浏览器运行架构说明")
						lbl.Color = CTabActiveText
						return lbl.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(spacerVertical(8)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(u.theme, unit.Sp(11), "• 界面引擎：Gio UI 原生硬件加速 (DirectX/Metal)\n• 网页内核：Microsoft Edge WebView2 (Chromium 内核，独立 STA 架构)\n• 代理生效范围：已针对 WebView2 进程环境变量注入参数")
				lbl.Color = CEditorHint
				return lbl.Layout(gtx)
			}),
		)
	})
}

// layoutInputField 统一输入框组件渲染。
func (u *UI) layoutInputField(gtx layout.Context, title, hint string, ed *widget.Editor) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Label(u.theme, unit.Sp(11), title)
			lbl.Color = CTabInactiveText
			return lbl.Layout(gtx)
		}),
		layout.Rigid(spacerVertical(4)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			border := widget.Border{
				Color:        CInputBorder,
				CornerRadius: unit.Dp(6),
				Width:        unit.Dp(1),
			}
			return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					editor := material.Editor(u.theme, ed, hint)
					editor.Color = CEditorFG
					editor.HintColor = CEditorHint
					return editor.Layout(gtx)
				})
			})
		}),
	)
}

// choicePill 胶囊形单选按钮控件（先录制内容计算精确尺寸，再绘制底色，避免无限撑大）。
func (u *UI) choicePill(gtx layout.Context, click *widget.Clickable, label string, selected bool) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := CBtnBG
		textColor := CTabInactiveText
		if selected {
			bg = CAccent
			textColor = COnAccent
		} else if click.Hovered() {
			bg = CToolbarBG
			textColor = CTabActiveText
		}

		// 精确测量内容尺寸
		macro := op.Record(gtx.Ops)
		dims := layout.Inset{
			Top: unit.Dp(6), Bottom: unit.Dp(6),
			Left: unit.Dp(14), Right: unit.Dp(14),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if selected {
						return glyph(gtx, iconDone, 12, textColor)
					}
					return layout.Dimensions{}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if selected {
						return spacer(6)(gtx)
					}
					return layout.Dimensions{}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(u.theme, unit.Sp(12), label)
					lbl.Color = textColor
					return lbl.Layout(gtx)
				}),
			)
		})
		call := macro.Stop()

		radius := gtx.Dp(unit.Dp(6))
		rrect := clip.RRect{
			Rect: image.Rectangle{Max: dims.Size},
			SE:   radius, SW: radius, NE: radius, NW: radius,
		}
		stack := rrect.Push(gtx.Ops)
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: dims.Size}.Op())
		stack.Pop()

		call.Add(gtx.Ops)
		return dims
	})
}

// cardBox 绘制带圆角与深色边框的容器卡片（精确测量子组件高度贴合包装）。
func (u *UI) cardBox(gtx layout.Context, w layout.Widget) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := layout.UniformInset(unit.Dp(18)).Layout(gtx, w)
	call := macro.Stop()

	radius := gtx.Dp(unit.Dp(10))
	rrect := clip.RRect{
		Rect: image.Rectangle{Max: dims.Size},
		SE:   radius, SW: radius, NE: radius, NW: radius,
	}
	stack := rrect.Push(gtx.Ops)
	paint.FillShape(gtx.Ops, CToolbarBG, clip.Rect{Max: dims.Size}.Op())
	stack.Pop()

	// 1px 细边框描边
	paint.FillShape(gtx.Ops, CBorder, clip.Stroke{
		Path:  rrect.Path(gtx.Ops),
		Width: float32(gtx.Dp(unit.Dp(1))),
	}.Op())

	call.Add(gtx.Ops)
	return dims
}

// spacerVertical 返回固定垂直间距。
func spacerVertical(dp int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Spacer{Height: unit.Dp(dp)}.Layout(gtx)
	}
}
