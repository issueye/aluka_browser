// Package win32 封装本工程所需的 Win32 窗口操作，
// 使上层（webview/ui）不直接接触 syscall 细节。
package win32

import (
	"image"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 子窗口样式
const (
	WS_CHILD        uintptr = 0x40000000
	WS_VISIBLE      uintptr = 0x10000000
	WS_CLIPCHILDREN uintptr = 0x02000000
	WS_CLIPSIBLINGS         = 0x04000000

	SWP_NOZORDER   = 0x0004
	SWP_NOACTIVATE = 0x0010

	SW_HIDE = 0
	SW_SHOW = 5

	GWL_STYLE = ^uintptr(15) // -16
)

var (
	user32                     = windows.NewLazySystemDLL("user32.dll")
	kernel32                   = windows.NewLazySystemDLL("kernel32.dll")
	dwmapi                     = windows.NewLazySystemDLL("dwmapi.dll")
	procGetWindowThreadProcess = user32.NewProc("GetWindowThreadProcessId")
	procEnumWindows            = user32.NewProc("EnumWindows")
	procGetWindowTextW         = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW   = user32.NewProc("GetWindowTextLengthW")
	procGetWindowLongPtrW      = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW      = user32.NewProc("SetWindowLongPtrW")
	procGetCurrentThreadId     = kernel32.NewProc("GetCurrentThreadId")
	procCreateWindowExW        = user32.NewProc("CreateWindowExW")
	procDestroyWindow          = user32.NewProc("DestroyWindow")
	procSetWindowPos           = user32.NewProc("SetWindowPos")
	procShowWindow             = user32.NewProc("ShowWindow")
	procSetFocus               = user32.NewProc("SetFocus")
	procDwmSetWindowAttribute  = dwmapi.NewProc("DwmSetWindowAttribute")
)

type Rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

func (r Rect) Width() int32  { return r.Right - r.Left }
func (r Rect) Height() int32 { return r.Bottom - r.Top }

// FindWindowByTitle 在当前进程中按标题子串查找顶层窗口 HWND。
func FindWindowByTitle(titleSubstr string) uintptr {
	currentPID := windows.GetCurrentProcessId()
	var foundHWND uintptr

	cb := syscall.NewCallback(func(hwnd, lparam uintptr) uintptr {
		var pid uint32
		procGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid != currentPID {
			return 1
		}

		length, _, _ := procGetWindowTextLengthW.Call(hwnd)
		if length == 0 {
			return 1
		}

		buf := make([]uint16, length+1)
		procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), length+1)
		title := windows.UTF16ToString(buf)

		if strings.Contains(title, titleSubstr) {
			foundHWND = hwnd
			return 0
		}
		return 1
	})

	procEnumWindows.Call(cb, 0)
	return foundHWND
}

// AddWindowStyles 为指定窗口追加样式位。
func AddWindowStyles(hwnd uintptr, styles uintptr) {
	if hwnd == 0 || styles == 0 {
		return
	}
	style, _, _ := procGetWindowLongPtrW.Call(hwnd, GWL_STYLE)
	procSetWindowLongPtrW.Call(hwnd, GWL_STYLE, style|styles)
}

// EnableNativeRoundCorners 启用 Win11 DWM 原生圆角 (8px)。
func EnableNativeRoundCorners(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	const (
		DWMWA_WINDOW_CORNER_PREFERENCE = 33
		DWMWCP_ROUND                   = 2
	)
	preference := uint32(DWMWCP_ROUND)
	procDwmSetWindowAttribute.Call(hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, uintptr(unsafe.Pointer(&preference)), 4)
}

// CreateStaticChild 在父窗口客户区坐标处创建 STATIC 子容器窗口。
func CreateStaticChild(parentHWND uintptr, x, y, w, h int32) uintptr {
	staticClass, _ := windows.UTF16PtrFromString("STATIC")
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(staticClass)),
		0,
		WS_CHILD|WS_VISIBLE|WS_CLIPCHILDREN|WS_CLIPSIBLINGS,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		parentHWND,
		0, 0, 0,
	)
	return hwnd
}

// DestroyWindow 销毁窗口。
func DestroyWindow(hwnd uintptr) {
	if hwnd != 0 {
		procDestroyWindow.Call(hwnd)
	}
}

// PositionWindow 调整窗口位置尺寸（保持 Z 序、不激活）。
func PositionWindow(hwnd uintptr, x, y, w, h int32) {
	if hwnd == 0 {
		return
	}
	procSetWindowPos.Call(
		hwnd, 0,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		SWP_NOZORDER|SWP_NOACTIVATE,
	)
}

func Show(hwnd uintptr) {
	if hwnd != 0 {
		procShowWindow.Call(hwnd, SW_SHOW)
	}
}

func Hide(hwnd uintptr) {
	if hwnd != 0 {
		procShowWindow.Call(hwnd, SW_HIDE)
	}
}

// FocusKeyboard 将系统键盘焦点移至指定窗口。
func FocusKeyboard(hwnd uintptr) {
	if hwnd != 0 {
		procSetFocus.Call(hwnd)
	}
}

// CurrentThreadID 返回当前 OS 线程 ID。
func CurrentThreadID() uint32 {
	tid, _, _ := procGetCurrentThreadId.Call()
	return uint32(tid)
}

// ---- 窗口图标 ----

const (
	WM_SETICON = 0x0080
	ICON_SMALL = 0
	ICON_BIG   = 1
)

var (
	procSendMessageW       = user32.NewProc("SendMessageW")
	procCreateIconIndirect = user32.NewProc("CreateIconIndirect")
	gdi32                  = windows.NewLazySystemDLL("gdi32.dll")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32 // 负值表示 top-down 行序
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type iconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

// SetWindowIcons 将像素图像设为窗口图标（big → 任务栏大图标，small → 小图标），
// 使无边框窗口在任务栏与 Alt+Tab 中呈现应用标识而非默认图标。
func SetWindowIcons(hwnd uintptr, big, small *image.NRGBA) {
	if hwnd == 0 {
		return
	}
	if h := createHICON(big); h != 0 {
		procSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, h)
	}
	if h := createHICON(small); h != 0 {
		procSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, h)
	}
}

func createHICON(img *image.NRGBA) uintptr {
	if img == nil || img.Rect.Empty() {
		return 0
	}
	w, h := int32(img.Rect.Dx()), int32(img.Rect.Dy())

	dc, _, _ := procCreateCompatibleDC.Call(0)
	defer procDeleteDC.Call(dc)

	colorBits, hbmpColor := allocDIB(dc, w, h)
	maskBits, hbmpMask := allocDIB(dc, w, h) // 全零掩码：透明度由颜色位图的 alpha 决定
	if hbmpColor == 0 || hbmpMask == 0 {
		procDeleteObject.Call(hbmpColor)
		procDeleteObject.Call(hbmpMask)
		return 0
	}
	if colorBits == nil || maskBits == nil {
		procDeleteObject.Call(hbmpColor)
		procDeleteObject.Call(hbmpMask)
		return 0
	}

	row := w * 4
	dst := unsafe.Slice(colorBits, int(row*h))
	for y := int32(0); y < h; y++ {
		src := img.Pix[int(y)*img.Stride : int(y)*img.Stride+int(row)]
		copy(dst[y*row:(y+1)*row], src)
	}

	ii := iconInfo{FIcon: 1, HbmMask: hbmpMask, HbmColor: hbmpColor}
	hicon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))

	procDeleteObject.Call(hbmpColor)
	procDeleteObject.Call(hbmpMask)
	return hicon
}

// allocDIB 创建一块 32bpp top-down DIB，返回像素内存指针与句柄。
func allocDIB(hdc uintptr, w, h int32) (*byte, uintptr) {
	bi := bitmapInfoHeader{
		Size:     40,
		Width:    w,
		Height:   -h,
		Planes:   1,
		BitCount: 32,
	}
	var bits *byte
	hbmp, _, _ := procCreateDIBSection.Call(
		hdc,
		uintptr(unsafe.Pointer(&bi)),
		0,                              // DIB_RGB_COLORS
		uintptr(unsafe.Pointer(&bits)), // void** ppvBits
		0, 0,
	)
	return bits, hbmp
}
