package webview

import (
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	"gio-browser/internal/win32"
)

// WM_APP_ACTION 是宿主 STA 线程私有消息，用于唤醒任务队列。
const WM_APP_ACTION = 0x8000 + 1

// WM_QUIT 请求 GetMessageW 消息循环退出。
const WM_QUIT = 0x0012

var (
	procPostThreadMessageW = windows.NewLazySystemDLL("user32.dll").NewProc("PostThreadMessageW")
	procGetMessageW        = windows.NewLazySystemDLL("user32.dll").NewProc("GetMessageW")
	procTranslateMessage   = windows.NewLazySystemDLL("user32.dll").NewProc("TranslateMessage")
	procDispatchMessageW   = windows.NewLazySystemDLL("user32.dll").NewProc("DispatchMessageW")
)

// msgStruct 与 Win32 MSG 结构体内存布局一致。
type msgStruct struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ X, Y int32 }
}

func postThreadMessage(threadID uint32, msg uint32) {
	procPostThreadMessageW.Call(uintptr(threadID), uintptr(msg), 0, 0)
}

// Start 启动 WebView2 宿主的专有 STA 消息循环线程。
//
// 所有针对 WebView2 的操作（创建标签页、导航、调整 bounds…）都会通过
// dispatch 投递到该线程执行；回调 onState / onOpenTab 同样在该线程触发，
// 上层须自行保证线程安全（browser.Browser 内部有锁）。
func (m *Manager) Start(
	parentHWND uintptr,
	onState StateFunc,
	onOpenTab OpenTabFunc,
) {
	m.mu.Lock()
	m.parentHWND = parentHWND
	m.onState = onState
	m.onOpenTab = onOpenTab
	m.mu.Unlock()

	readyChan := make(chan struct{})
	stoppedChan := make(chan struct{})
	var stopOnce sync.Once

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		_ = windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)

		m.mu.Lock()
		m.threadID = win32.CurrentThreadID()
		m.mu.Unlock()

		// 宿主窗口需要 WS_CLIPCHILDREN 才能避免子窗口区域闪烁
		win32.AddWindowStyles(parentHWND, win32.WS_CLIPCHILDREN|win32.WS_CLIPSIBLINGS)

		close(readyChan)

		// 启动后台标签挂起清理协程（阈值见 suspendAfter）
		m.startSuspendJanitor()

		// 持续 Win32 消息分发循环
		var msg msgStruct
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if r == 0 || int32(r) == -1 {
				break
			}
			if msg.message == WM_APP_ACTION {
				m.runTasks()
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}

		// 消息循环结束（收到 WM_QUIT），通知等待方
		stopOnce.Do(func() { close(stoppedChan) })
	}()

	<-readyChan
	m.mu.Lock()
	m.stopped = stoppedChan
	m.mu.Unlock()
}

// dispatch 向宿主 STA 线程派发一个任务，立即返回（异步）。
func (m *Manager) dispatch(fn func()) {
	m.mu.Lock()
	m.tasks = append(m.tasks, fn)
	tid := m.threadID
	m.mu.Unlock()

	if tid != 0 {
		postThreadMessage(tid, WM_APP_ACTION)
	}
}

func (m *Manager) runTasks() {
	m.mu.Lock()
	tasks := m.tasks
	m.tasks = nil
	m.mu.Unlock()

	for _, t := range tasks {
		t()
	}
}
