package procs

import (
	"strings"
	"testing"
)

func entry(pid, ppid uint32, name string, mem uint64) ProcInfo {
	return ProcInfo{PID: pid, PPID: ppid, Name: name, WorkingSet: mem}
}

func TestBuildTree(t *testing.T) {
	// 结构：system(100)
	//        └ browser(200)
	//            ├ webview-browser(300)
	//            │   ├ renderer-tab1(301)
	//            │   └ gpu(302)
	//            └ crashpad(310)
	//        other(400)
	self := uint32(200)
	input := []ProcInfo{
		entry(100, 0, "system.exe", 100),
		entry(200, 100, "gio-browser.exe", 5000),
		entry(300, 200, "msedgewebview2.exe", 3000),
		entry(301, 300, "msedgewebview2.exe", 8000),
		entry(302, 300, "msedgewebview2.exe", 1000),
		entry(310, 200, "msedgewebview2.exe", 200), // crashpad 语义，命名简化
		entry(400, 0, "other.exe", 50),
	}

	roots := BuildTree(input, self)
	if len(roots) != 2 {
		t.Fatalf("根数量 = %d, want 2", len(roots))
	}

	// 浏览器(200) 父进程存在，挂在 system(100) 之下而非根
	var browser *Node
	for _, n := range Flatten(roots) {
		if n.Info.PID == self {
			browser = n
		}
	}
	if browser == nil {
		t.Fatal("未找到浏览器节点")
	}
	if browser.Depth != 1 {
		t.Fatalf("浏览器深度 = %d, want 1（system 之下）", browser.Depth)
	}
	if !browser.InBrowserSubtree {
		t.Fatal("浏览器根应标记子树内")
	}
	if len(browser.Children) != 2 {
		t.Fatalf("浏览器直接子进程数 = %d, want 2", len(browser.Children))
	}
	// 同层按内存降序：webview-browser(3000) 在 crashpad(200) 前
	if browser.Children[0].Info.PID != 300 {
		t.Fatalf("子层排序错误: 首个应为 PID 300, got %d", browser.Children[0].Info.PID)
	}
	wv := browser.Children[0]
	if wv.Children[0].Info.PID != 301 { // 8000 > 1000
		t.Fatalf("孙层排序错误: %d", wv.Children[0].Info.PID)
	}

	// 全部后代均标记子树内
	for _, n := range Flatten([]*Node{browser}) {
		if !n.InBrowserSubtree {
			t.Fatalf("PID %d 应在浏览器子树内", n.Info.PID)
		}
	}
	// 其他进程不受标记
	for _, n := range Flatten(roots) {
		if n.Info.PID == 400 && n.InBrowserSubtree {
			t.Fatal("无关进程不应被标记")
		}
	}

	// 深度正确（system(0) → browser(1) → webview(2) → renderer(3)）
	if wv.Depth != 2 || wv.Children[0].Depth != 3 {
		t.Fatalf("深度错误: %d/%d", wv.Depth, wv.Children[0].Depth)
	}

	// 展平后总量守恒
	if got := len(Flatten(roots)); got != len(input) {
		t.Fatalf("展平数量 = %d, want %d", got, len(input))
	}
}

func TestBuildTreeCyclePromotedToRoot(t *testing.T) {
	// 环：1 -> 2 -> 1（父互指），且父均在快照中 → 不得死循环、不得丢节点
	input := []ProcInfo{
		entry(1, 2, "a.exe", 10),
		entry(2, 1, "b.exe", 20),
	}
	roots := BuildTree(input, 999)
	if got := len(Flatten(roots)); got != 2 {
		t.Fatalf("环中节点应全部保留, got %d", got)
	}
}

func TestFilterTreeKeepsAncestors(t *testing.T) {
	input := []ProcInfo{
		entry(100, 0, "system.exe", 100),
		entry(200, 100, "gio-browser.exe", 500),
		entry(301, 200, "msedgewebview2.exe", 800),
	}
	roots := BuildTree(input, 200)

	filtered := FilterTree(roots, "msedgewebview2")
	rows := Flatten(filtered)
	if len(rows) != 3 {
		t.Fatalf("过滤后应保留完整祖先链 3 行, got %d", len(rows))
	}
	joined := rows[0].Info.Name + "," + rows[1].Info.Name + "," + rows[2].Info.Name
	if !strings.Contains(joined, "gio-browser") || !strings.Contains(joined, "msedgewebview2") {
		t.Fatalf("祖先链不完整: %s", joined)
	}

	if got := Flatten(FilterTree(roots, "不存在的名字")); len(got) != 0 {
		t.Fatalf("无匹配应过滤为空, got %d", len(got))
	}

	// 大小写不敏感
	if got := Flatten(FilterTree(roots, "GIO-BROWSER")); len(got) != 2 {
		t.Fatalf("过滤应大小写不敏感, got %d", len(got))
	}

	// 空过滤返回原树
	if got := Flatten(FilterTree(roots, "  ")); len(got) != 3 {
		t.Fatalf("空过滤应返回全量, got %d", len(got))
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   uint64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{5 * 1024 * 1024, "5.0 MB"},
		{3 * 1024 * 1024 * 1024, "3.0 GB"},
	}
	for _, tc := range cases {
		if got := FormatBytes(tc.in); got != tc.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
