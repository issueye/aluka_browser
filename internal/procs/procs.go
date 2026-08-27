// Package procs 提供进程快照的树状组织与过滤能力：
// 将系统进程按父子关系组织为进程树，标记浏览器自身子树，
// 供进程管理浮窗呈现。树构建为纯逻辑，与 Win32 采集解耦、可独立测试。
package procs

import (
	"fmt"
	"sort"
	"strings"

	"gio-browser/internal/win32"
)

// ProcInfo 快照中的单个进程（已含内存信息）。
type ProcInfo struct {
	PID        uint32
	PPID       uint32
	Name       string
	Threads    uint32
	WorkingSet uint64 // 字节；不可访问的进程为 0
}

// Node 进程树节点（Children 已按内存降序排列）。
type Node struct {
	Info     ProcInfo
	Depth    int
	Children []*Node

	// InBrowserSubtree 是否位于浏览器自身进程子树内
	// （根为 gio-browser.exe，含各标签的 msedgewebview2 进程族）
	InBrowserSubtree bool
}

// Snapshot 采集一次系统进程快照并装配内存信息。
func Snapshot() []ProcInfo {
	entries := win32.ListProcesses()
	if entries == nil {
		return nil
	}
	out := make([]ProcInfo, 0, len(entries))
	for _, e := range entries {
		out = append(out, ProcInfo{
			PID:     e.PID,
			PPID:    e.PPID,
			Name:    e.Name,
			Threads: e.Threads,
		})
	}
	// 内存查询按 PID 逐个进行（OpenProcess+GetProcessMemoryInfo）
	for i := range out {
		out[i].WorkingSet = win32.WorkingSetSize(out[i].PID)
	}
	return out
}

// BuildTree 将进程列表组织为进程树，返回根节点切片。
//
// 规则：
//   - 父进程不在快照中的进程作为根（系统进程树天然多根：Idle/System/...）；
//   - ppid 环（异常快照）中的进程提升为根，保证不丢失、不死循环；
//   - 同层按内存降序、内存相同按名称升序，保证展示稳定。
//
// selfPID 用于标记浏览器自身子树（该进程及其全部后代）。
func BuildTree(procs []ProcInfo, selfPID uint32) []*Node {
	nodes := make(map[uint32]*Node, len(procs))
	for _, p := range procs {
		nodes[p.PID] = &Node{Info: p}
	}

	// 挂接父子关系
	var roots []*Node
	for _, n := range nodes {
		parent, ok := nodes[n.Info.PPID]
		if !ok || n.Info.PPID == n.Info.PID {
			roots = append(roots, n)
			continue
		}
		parent.Children = append(parent.Children, n)
	}

	// 环检测：从根可达的节点全部标记；不可达者（处于环中）提升为根。
	// 标记过程中跳过已标记的子节点——即切断回边，保证 Children 最终无环，
	// 否则后续排序/展平的递归会陷入死循环。
	marked := make(map[uint32]bool, len(nodes))
	var mark func(n *Node, depth int)
	mark = func(n *Node, depth int) {
		if marked[n.Info.PID] {
			return
		}
		marked[n.Info.PID] = true
		n.Depth = depth
		safe := make([]*Node, 0, len(n.Children))
		for _, c := range n.Children {
			if !marked[c.Info.PID] {
				safe = append(safe, c)
			}
		}
		n.Children = safe
		for _, c := range n.Children {
			mark(c, depth+1)
		}
	}
	for _, r := range roots {
		mark(r, 0)
	}
	for _, n := range nodes {
		if !marked[n.Info.PID] {
			n.Depth = 0
			roots = append(roots, n)
			mark(n, 0)
		}
	}

	// 标记浏览器子树
	if self, ok := nodes[selfPID]; ok {
		var markSub func(n *Node)
		markSub = func(n *Node) {
			n.InBrowserSubtree = true
			for _, c := range n.Children {
				markSub(c)
			}
		}
		markSub(self)
	}

	// 同层排序（内存降序 → 名称升序）
	sortNodes(roots)
	return roots
}

func sortNodes(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		a, b := nodes[i], nodes[j]
		if a.Info.WorkingSet != b.Info.WorkingSet {
			return a.Info.WorkingSet > b.Info.WorkingSet
		}
		return a.Info.Name < b.Info.Name
	})
	for _, n := range nodes {
		sortNodes(n.Children)
	}
}

// Flatten 将树展开为深度优先行序列，供线性列表渲染。
func Flatten(roots []*Node) []*Node {
	var out []*Node
	var walk func(n *Node)
	walk = func(n *Node) {
		out = append(out, n)
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, r := range roots {
		walk(r)
	}
	return out
}

// FilterTree 按名称子串（大小写不敏感）过滤树：
// 命中节点的全部祖先保留；返回新根切片，不影响原树。
func FilterTree(roots []*Node, substr string) []*Node {
	substr = strings.ToLower(strings.TrimSpace(substr))
	if substr == "" {
		return roots
	}
	var filter func(n *Node) *Node
	filter = func(n *Node) *Node {
		kept := make([]*Node, 0, len(n.Children))
		for _, c := range n.Children {
			if fc := filter(c); fc != nil {
				kept = append(kept, fc)
			}
		}
		selfHit := strings.Contains(strings.ToLower(n.Info.Name), substr)
		if !selfHit && len(kept) == 0 {
			return nil
		}
		cp := *n
		cp.Children = kept
		return &cp
	}

	var out []*Node
	for _, r := range roots {
		if fr := filter(r); fr != nil {
			out = append(out, fr)
		}
	}
	return out
}

// FormatBytes 将字节数格式化为人类可读串（B/KB/MB/GB）。
func FormatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit && exp < 3; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGT"[exp])
}
