package win32

import (
	"fmt"
	"log"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ---- 进程枚举与管理 ----

// ProcEntry 进程快照中的单条进程信息（不含内存，内存单独查询）。
type ProcEntry struct {
	PID     uint32
	PPID    uint32
	Name    string
	Threads uint32
}

// ProcMemory 单个进程的内存占用信息（字节）。
type ProcMemory struct {
	WorkingSet uint64
}

var (
	// K32GetProcessMemoryInfo：Win7+ 起真实导出位于 kernel32；
	// psapi.dll 仅为转发存根，直接调用会得到异常结果
	procGetProcessMemoryInfo = kernel32.NewProc("K32GetProcessMemoryInfo")
	procSetWindowPosTopmost  = user32.NewProc("SetWindowPos")
	// 注意：本工程验证过的 Windows 10 (19044) 上 kernel32 仅导出
	// SetProcessWorkingSetSize（无 K32/Ex 变体），psapi.dll 则连该名字都没有
	procSetProcessWorkingSetSize = kernel32.NewProc("SetProcessWorkingSetSize")
)

// processMemoryCounters 与 Win32 PROCESS_MEMORY_COUNTERS_EX 布局一致。
// 注意：部分 Windows 版本的 GetProcessMemoryInfo 校验 cb ≥ 72（EX 尺寸），
// 传标准 64 字节 counters 会被以 ERROR_INSUFFICIENT_BUFFER 拒绝，
// 故尾部额外保留 8 字节。
type processMemoryCounters struct {
	CB                      uint32
	PageFaultCount          uint32
	PeakWorkingSetSize      uint64
	WorkingSetSize          uint64
	QuotaPeakPagedPoolUsage uint64
	QuotaPagedPoolUsage     uint64
	QuotaNonPagedPoolUsage  uint64
	PagefileUsage           uint64
	PeakPagefileUsage       uint64
	Reserved                uint64
}

// ListProcesses 枚举当前系统全部进程（Toolhelp32 快照）。
func ListProcesses() []ProcEntry {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)

	var out []ProcEntry
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snap, &entry); err != nil {
		return nil
	}
	for {
		out = append(out, ProcEntry{
			PID:     entry.ProcessID,
			PPID:    entry.ParentProcessID,
			Name:    windows.UTF16ToString(entry.ExeFile[:]),
			Threads: entry.Threads,
		})
		if err := windows.Process32Next(snap, &entry); err != nil {
			break
		}
	}
	return out
}

// WorkingSetSize 查询进程当前工作集大小（字节）；无法访问的进程返回 0。
func WorkingSetSize(pid uint32) uint64 {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)

	var pmc processMemoryCounters
	pmc.CB = uint32(unsafe.Sizeof(pmc))
	r, _, _ := procGetProcessMemoryInfo.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&pmc)),
		uintptr(pmc.CB),
	)
	if r == 0 {
		return 0
	}
	return pmc.WorkingSetSize
}

// TerminateProcess 结束指定进程；返回便于上层呈现的错误信息。
func TerminateProcess(pid uint32) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return fmt.Errorf("打开进程 %d 失败（权限不足或已退出）: %w", pid, err)
	}
	defer windows.CloseHandle(h)

	if err := windows.TerminateProcess(h, 1); err != nil {
		return fmt.Errorf("结束进程 %d 失败: %w", pid, err)
	}
	return nil
}

// CurrentPID 返回当前进程 ID。
func CurrentPID() uint32 {
	return windows.GetCurrentProcessId()
}

// TrimWorkingSet 将指定进程的物理工作集尽量换出（min=max=-1 语义）。
// 被trim的页面进入系统内存压缩/页面文件；进程再次活跃时按需回迁。
func TrimWorkingSet(pid uint32) error {
	// PROCESS_SET_QUOTA 理论上足够，但实测对部分 Chromium 子进程需要
	// 更宽的访问掩码，统一用 ALL_ACCESS（与 PowerShell 验证路径一致）
	h, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, pid)
	if err != nil {
		return fmt.Errorf("打开进程 %d 失败: %w", pid, err)
	}
	defer windows.CloseHandle(h)

	const (
		lo = ^uintptr(0) // -1：清空工作集
		hi = ^uintptr(0)
	)
	r, _, callErr := procSetProcessWorkingSetSize.Call(uintptr(h), lo, hi)
	if r == 0 {
		return fmt.Errorf("调整进程 %d 工作集失败: %v", pid, callErr)
	}
	return nil
}

// TrimProcessTree 将 rootPID 及其全部后代进程的工作集逐一裁剪，
// exclude 中的 PID 跳过不处理（如宿主 UI 自身），返回成功裁剪的进程数。
// 用于浏览器进程树（宿主 + WebView2 全家桶）。
func TrimProcessTree(rootPID uint32, exclude map[uint32]bool) int {
	entries := ListProcesses()
	children := make(map[uint32][]uint32, len(entries))
	for _, e := range entries {
		children[e.PPID] = append(children[e.PPID], e.PID)
	}
	// BFS 收集子树（含根）；exclude 仅跳过裁剪动作，不阻断遍历
	queue := []uint32{rootPID}
	seen := map[uint32]bool{rootPID: true}
	var order []uint32
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		if !exclude[pid] {
			order = append(order, pid)
		}
		for _, c := range children[pid] {
			if !seen[c] {
				seen[c] = true
				queue = append(queue, c)
			}
		}
	}

	count := 0
	for _, pid := range order {
		if err := TrimWorkingSet(pid); err != nil {
			log.Printf("[内存] 裁剪进程 %d 失败: %v", pid, err)
			continue
		}
		count++
	}
	return count
}

// SetTopmost 将窗口置于最顶层（浮窗语义）；topmost=false 恢复常规层级。
func SetTopmost(hwnd uintptr, topmost bool) {
	if hwnd == 0 {
		return
	}
	const (
		HWND_TOPMOST   = ^uintptr(0) // -1
		HWND_NOTOPMOST = ^uintptr(1) // -2
		SWP_NOSIZE     = 0x0001
		SWP_NOMOVE     = 0x0002
		SWP_NOACTIVATE = 0x0010
		SWP_SHOWWINDOW = 0x0040
	)
	after := HWND_NOTOPMOST
	if topmost {
		after = HWND_TOPMOST
	}
	procSetWindowPosTopmost.Call(
		hwnd, after, 0, 0, 0, 0,
		SWP_NOSIZE|SWP_NOMOVE|SWP_NOACTIVATE|SWP_SHOWWINDOW,
	)
}
