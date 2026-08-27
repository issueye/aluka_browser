package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"gio-browser/internal/config"
	"gio-browser/internal/extension"
)

// pluginSidebarState 右侧插件侧栏状态。
//
// 展开时侧栏占用 300dp 宽度，FrameMetrics.Sidebar 同步反馈给 app 层，
// WebView 宽度相应扣除（临时挤压）；收起后零占位，网页立即复原满宽。
type pluginSidebarState struct {
	visible bool
	width   int // dp，展开态宽度
	toggle  widget.Clickable
	openSet widget.Clickable
	list    layout.List
	cards   map[string]*widget.Clickable // per-extension toggle cache
}

func (u *UI) initPluginSidebarState() {
	defVis, defW := config.PluginSidebarDefaults()
	cfg := config.Current()
	vis := defVis
	if cfg.PluginSidebarVisible != nil {
		vis = *cfg.PluginSidebarVisible
	}
	w := cfg.PluginSidebarWidth
	if w == 0 {
		w = defW
	}
	u.pluginSidebar = pluginSidebarState{
		visible: vis,
		width:   w,
		cards:   make(map[string]*widget.Clickable),
	}
	u.pluginSidebar.list.Axis = layout.Vertical
}

// persistPluginSidebar 将侧栏显隐与宽度写入配置。
func (u *UI) persistPluginSidebar() {
	vis := u.pluginSidebar.visible
	cfg := config.Current()
	cfg.PluginSidebarVisible = &vis
	if u.pluginSidebar.width == 0 {
		_, defW := config.PluginSidebarDefaults()
		cfg.PluginSidebarWidth = defW
	} else {
		cfg.PluginSidebarWidth = u.pluginSidebar.width
	}
	_ = config.Save(cfg)
}

// closePluginSidebar 收起插件侧栏并持久化（已在收起状态时为空操作）。
func (u *UI) closePluginSidebar() {
	if !u.pluginSidebar.visible {
		return
	}
	u.pluginSidebar.visible = false
	u.persistPluginSidebar()
}

// LayoutPluginSidebarWidth 返回展开时的侧栏宽度（物理像素），收起为 0。
// 由 LayoutRoot 调用并计入 FrameMetrics.Sidebar。
func (u *UI) LayoutPluginSidebarWidth(gtx layout.Context) int {
	if !u.pluginSidebar.visible {
		return 0
	}
	return gtx.Dp(unit.Dp(u.pluginSidebar.width))
}

// layoutPluginSidebar 渲染右侧插件侧栏（内容区水平 Flex 的 Rigid 部分）。
func (u *UI) layoutPluginSidebar(gtx layout.Context) layout.Dimensions {
	if !u.pluginSidebar.visible {
		return layout.Dimensions{} // 收起态零占位
	}

	// 侧栏内"打开设置"：收起侧栏后进入设置中心，避免两者同时出现
	if u.pluginSidebar.openSet.Clicked(gtx) {
		u.closePluginSidebar()
		u.b.OpenSettings()
	}

	w := gtx.Dp(unit.Dp(u.pluginSidebar.width))
	gtx.Constraints.Min.X = w
	gtx.Constraints.Max.X = w

	return u.cardBox(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top: unit.Dp(4), Bottom: unit.Dp(4),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return u.layoutPluginSidebarHeader(gtx)
				}),
				layout.Rigid(spacerVertical(8)),
				layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
					return u.layoutPluginSidebarBody(gtx)
				}),
			)
		})
	})
}

func (u *UI) layoutPluginSidebarHeader(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return glyph(gtx, iconExtensions, 14, CAccent)
		}),
		layout.Rigid(spacer(6)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Label(u.theme, unit.Sp(12), "插件")
			lbl.Color = CTabActiveText
			return lbl.Layout(gtx)
		}),
		layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Point{X: gtx.Constraints.Max.X, Y: 0}}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.IconButton(u.theme, &u.pluginSidebar.toggle, iconClose, "收起")
			btn.Background = CBtnBG
			btn.Color = CBtnFG
			btn.Size = unit.Dp(14)
			btn.Inset = layout.UniformInset(unit.Dp(5))
			return btn.Layout(gtx)
		}),
	)
}

func (u *UI) layoutPluginSidebarBody(gtx layout.Context) layout.Dimensions {
	exts := extension.GetGlobalManager().List()
	if len(exts) == 0 {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(u.theme, unit.Sp(11), "暂无插件")
				lbl.Color = CTabInactiveText
				return lbl.Layout(gtx)
			}),
			layout.Rigid(spacerVertical(6)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(u.theme, unit.Sp(10), "去 设置 → 扩展 中加载已解压扩展")
				lbl.Color = CEditorHint
				return lbl.Layout(gtx)
			}),
			layout.Rigid(spacerVertical(8)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(u.theme, &u.pluginSidebar.openSet, "打开设置")
				btn.Background = CBtnBG
				btn.Color = CBtnFG
				btn.TextSize = unit.Sp(11)
				btn.Inset = layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(12), Right: unit.Dp(12)}
				return btn.Layout(gtx)
			}),
		)
	}

	items := make([]layout.Widget, 0, len(exts))
	for _, e := range exts {
		ext := e
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return u.layoutPluginSidebarCard(gtx, ext)
		}, spacerVertical(6))
	}
	return u.pluginSidebar.list.Layout(gtx, len(items), func(gtx layout.Context, i int) layout.Dimensions {
		return items[i](gtx)
	})
}

func (u *UI) layoutPluginSidebarCard(gtx layout.Context, ext *extension.Extension) layout.Dimensions {
	ctl := u.pluginSidebar.cards[ext.ID]
	if ctl == nil {
		ctl = &widget.Clickable{}
		u.pluginSidebar.cards[ext.ID] = ctl
	}
	if ctl.Clicked(gtx) {
		_, _ = extension.GetGlobalManager().Toggle(ext.ID)
	}
	enabled := ext.Enabled
	label := "启用"
	bg := CAccent
	if enabled {
		label = "停用"
		bg = CBtnBG
	}
	return u.cardBox(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(u.theme, unit.Sp(11), ext.Manifest.Name)
						lbl.Color = CTabActiveText
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.badgePill(gtx, "v"+ext.Manifest.Version, CBtnBG, CTabInactiveText)
					}),
				)
			}),
			layout.Rigid(spacerVertical(6)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(u.theme, ctl, label)
				btn.Background = bg
				btn.Color = COnAccent
				btn.TextSize = unit.Sp(11)
				btn.Inset = layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(10), Right: unit.Dp(10)}
				return btn.Layout(gtx)
			}),
		)
	})
}