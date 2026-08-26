package ui

import "image/color"

// 深色主题调色板。集中定义，避免色值散落在布局代码里。
var (
	// 窗口各区域背景
	CWindowBG    = color.NRGBA{R: 16, G: 19, B: 26, A: 255}  // 标签栏 / 状态栏底色
	CToolbarBG   = color.NRGBA{R: 28, G: 32, B: 42, A: 255}  // 工具栏 / 活跃标签页底色
	CQuickBG     = color.NRGBA{R: 22, G: 25, B: 34, A: 255}  // 书签栏底色
	CContentBG   = color.NRGBA{R: 15, G: 17, B: 23, A: 255}  // 页面内容区兜底底色
	CStatusBG    = color.NRGBA{R: 16, G: 18, B: 24, A: 255}  // 底部状态栏底色
	CBorder      = color.NRGBA{R: 45, G: 52, B: 68, A: 255}  // 描边 / 分隔线
	CInputBorder = color.NRGBA{R: 70, G: 80, B: 105, A: 255} // 地址栏边框

	// 标签页
	CTabInactiveBG   = color.NRGBA{R: 20, G: 23, B: 30, A: 255}
	CTabInactiveText = color.NRGBA{R: 160, G: 165, B: 180, A: 255}
	CTabActiveText   = color.NRGBA{R: 245, G: 245, B: 250, A: 255}
	CTabHoverBG      = color.NRGBA{R: 26, G: 30, B: 40, A: 255}   // 悬停态胶囊底色
	CTabActiveBG     = color.NRGBA{R: 41, G: 49, B: 66, A: 255}   // 活跃态胶囊底色
	CTabIdleClose    = color.NRGBA{R: 160, G: 165, B: 180, A: 70} // 非悬停时淡化的 ×
	CUnderline       = color.NRGBA{R: 96, G: 146, B: 244, A: 255} // 活跃标签底部亮条

	// 站点头像占位色板 (背景, 文字) —— 与深色 UI 协调的中饱和色对，按站点哈希取色
	SiteBadgePalette = [8][2]color.NRGBA{
		{{R: 47, G: 92, B: 178, A: 255}, {R: 214, G: 228, B: 255, A: 255}},  // 蓝
		{{R: 44, G: 118, B: 92, A: 255}, {R: 206, G: 242, B: 223, A: 255}},  // 绿
		{{R: 158, G: 92, B: 34, A: 255}, {R: 255, G: 228, B: 198, A: 255}},  // 橙
		{{R: 128, G: 58, B: 136, A: 255}, {R: 242, G: 217, B: 250, A: 255}}, // 紫
		{{R: 168, G: 62, B: 68, A: 255}, {R: 255, G: 218, B: 218, A: 255}},  // 红
		{{R: 42, G: 108, B: 148, A: 255}, {R: 205, G: 232, B: 252, A: 255}}, // 青
		{{R: 120, G: 104, B: 36, A: 255}, {R: 246, G: 235, B: 192, A: 255}}, // 金
		{{R: 82, G: 84, B: 118, A: 255}, {R: 220, G: 222, B: 246, A: 255}},  // 灰紫
	}

	// 窗口控制红绿灯
	CCloseFill     = color.NRGBA{R: 255, G: 95, B: 86, A: 255}
	CCloseHover    = color.NRGBA{R: 224, G: 68, B: 62, A: 255}
	CMinimizeFill  = color.NRGBA{R: 255, G: 189, B: 46, A: 255}
	CMinimizeHover = color.NRGBA{R: 222, G: 161, B: 35, A: 255}
	CMaxFill       = color.NRGBA{R: 39, G: 201, B: 63, A: 255}
	CMaxHover      = color.NRGBA{R: 26, G: 171, B: 41, A: 255}

	// 控件
	CBtnBG       = color.NRGBA{R: 37, G: 43, B: 58, A: 255}
	CBtnFG       = color.NRGBA{R: 220, G: 228, B: 245, A: 255}
	CSubBtnBG    = color.NRGBA{R: 35, G: 40, B: 55, A: 255}
	CSubBtnFG    = color.NRGBA{R: 200, G: 210, B: 235, A: 255}
	CAccent      = color.NRGBA{R: 56, G: 114, B: 224, A: 255}
	COnAccent    = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	CEditorFG    = color.NRGBA{R: 245, G: 245, B: 250, A: 255}
	CEditorHint  = color.NRGBA{R: 140, G: 145, B: 160, A: 255}
	CStatusText  = color.NRGBA{R: 170, G: 175, B: 190, A: 255}
	CAccentGreen = color.NRGBA{R: 90, G: 200, B: 130, A: 255}
)
