// gio-browser：基于 Gio + WebView2 的无边框多标签浏览器。
//
// 工程结构：
//
//	main.go            入口
//	internal/app       应用装配与事件循环
//	internal/browser   浏览器领域模型（标签页状态机）
//	internal/ui        Gio 界面组件
//	internal/webview   WebView2 多标签引擎（STA 宿主线程）
//	internal/win32     Win32 系统调用封装
package main

import (
	"log"

	"gio-browser/internal/app"
)

func main() {
	go func() {
		if err := app.Run(); err != nil {
			log.Fatal(err)
		}
	}()

	app.Main()
}
