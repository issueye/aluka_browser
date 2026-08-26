package ui

import (
	"gioui.org/widget"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

// 图标资源（material design 矢量图标，编译期内嵌）。
var (
	iconBack, _    = widget.NewIcon(icons.NavigationArrowBack)
	iconForward, _ = widget.NewIcon(icons.NavigationArrowForward)
	iconRefresh, _ = widget.NewIcon(icons.NavigationRefresh)
	iconHome, _    = widget.NewIcon(icons.ActionHome)
	iconAdd, _         = widget.NewIcon(icons.ContentAdd)
	iconClose, _       = widget.NewIcon(icons.NavigationClose)
	iconSettings, _    = widget.NewIcon(icons.ActionSettings)
	iconDone, _        = widget.NewIcon(icons.ActionDone)
	iconTune, _        = widget.NewIcon(icons.ImageTune)
	iconInfo, _        = widget.NewIcon(icons.ActionInfo)
	iconSave, _        = widget.NewIcon(icons.ContentSave)
	iconDescription, _ = widget.NewIcon(icons.ActionDescription)
	iconFlashOn, _     = widget.NewIcon(icons.ImageFlashOn)
	iconBookmark, _    = widget.NewIcon(icons.ActionBookmark)

	// 地址栏前缀：https 显示锁，其余显示地球
	iconLock  = mustNewIcon(icons.ActionLock)
	iconGlobe = mustNewIcon(icons.ActionLanguage)
)

func mustNewIcon(data []byte) *widget.Icon {
	ic, err := widget.NewIcon(data)
	if err != nil {
		panic(err)
	}
	return ic
}
