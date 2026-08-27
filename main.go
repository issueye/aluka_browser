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
	"os"

	"gio-browser/internal/app"
)

func main() {
	go func() {
		if err := app.Run(); err != nil {
			log.Printf("应用异常退出: %v", err)
			os.Exit(1)
		}
		// gioapp.Main 在 Windows 上不随最后一个窗口关闭而返回；
		// 此时引擎与窗口均已清理完毕，直接结束进程
		os.Exit(0)
	}()

	app.Main()
}
