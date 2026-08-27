package webview

import (
	"sync"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2/pkg/edge"
)

// 后台标签挂起：基于官方 ICoreWebView2_3::TrySuspendAsync / Resume 实现。
//
// 槽位推导（已与 webview2-sys 官方绑定逐方法核对，与本项目依赖的
// go-webview2 fork 的 vtbl 定义一致）：
//
//	IUnknown               3 个槽
//	ICoreWebView2         58 个槽（含 go_back/go_forward 等）
//	ICoreWebView2_2        7 个槽
//	→ TrySuspendAsync      = 3 + 58 + 7 + 0 = 68
//	→ Resume               = 69
//	→ GetIsSuspended       = 70
//
// 与 manager.go 中 PutBounds 一样属于受控的 vtable 直调例外；
// 上层请使用本文件封装的 suspendView / resumeView，勿自行计算槽位。
const (
	vtblSlotTrySuspendAsync = 68
	vtblSlotResume          = 69
	vtblSlotGetIsSuspended  = 70
)

// suspendHandler 是 TrySuspendAsync 所需的完成通知回调对象
// （ICoreWebView2TrySuspendCompletedHandler 的最小 COM 实现）。
// 全局共享一个实例：Invoke 仅吞掉完成码，无需逐次分配。
// vtable 存为值数组（非 uintptr），在调用点即时转换，避免持久化 Go 指针。
var (
	suspendHandlerOnce sync.Once
	suspendHandlerVT   [4]uintptr
)

func initSuspendHandler() {
	// QI 返回 E_NOINTERFACE：运行时对完成回调仅使用 AddRef/Release/Invoke
	// 三个槽位，不会查询其他接口；如此可避免 uintptr→Pointer 回写（vet 不洁）。
	// 若运行时行为变化导致挂起失败，日志会有对应错误输出。
	qi := syscall.NewCallback(func(this, riid, out uintptr) uintptr {
		return 1 // E_NOINTERFACE
	})
	addRef := syscall.NewCallback(func(this uintptr) uintptr { return 1 })
	release := syscall.NewCallback(func(this uintptr) uintptr { return 1 })
	invoke := syscall.NewCallback(func(this, errorCode uintptr) uintptr { return 0 })

	suspendHandlerVT[0], suspendHandlerVT[1] = qi, addRef
	suspendHandlerVT[2], suspendHandlerVT[3] = release, invoke
}

// vtblProc 读取接口对象 vtbl 中第 slot 个方法指针。
func vtblProc(comObj unsafe.Pointer, slot int) uintptr {
	vtbl := *(*unsafe.Pointer)(comObj)
	return *(*uintptr)(unsafe.Add(vtbl, 8*slot))
}

// suspendView 请求挂起指定 WebView（异步）。
// 调用前须保证 controller 不可见，否则运行时会拒绝挂起。
func suspendView(i3 *edge.ICoreWebView2_3) error {
	suspendHandlerOnce.Do(initSuspendHandler)
	proc := vtblProc(unsafe.Pointer(i3), vtblSlotTrySuspendAsync)
	hr, _, _ := syscall.SyscallN(proc,
		uintptr(unsafe.Pointer(i3)),
		uintptr(unsafe.Pointer(&suspendHandlerVT[0])),
	)
	if hr != 0 {
		return syscall.Errno(hr)
	}
	return nil
}

// resumeView 恢复已挂起的 WebView（同步；对未挂起对象为安全空操作）。
func resumeView(i3 *edge.ICoreWebView2_3) error {
	proc := vtblProc(unsafe.Pointer(i3), vtblSlotResume)
	hr, _, _ := syscall.SyscallN(proc, uintptr(unsafe.Pointer(i3)))
	if hr != 0 {
		return syscall.Errno(hr)
	}
	return nil
}

// isViewSuspended 查询真实挂起状态（用于校验，非必须路径）。
func isViewSuspended(i3 *edge.ICoreWebView2_3) bool {
	proc := vtblProc(unsafe.Pointer(i3), vtblSlotGetIsSuspended)
	var suspended int32
	syscall.SyscallN(proc, uintptr(unsafe.Pointer(i3)), uintptr(unsafe.Pointer(&suspended)))
	return suspended != 0
}
