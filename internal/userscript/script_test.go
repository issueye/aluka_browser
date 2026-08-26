package userscript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMetadata(t *testing.T) {
	code := `// ==UserScript==
// @name         GitHub 增强助手
// @namespace    https://github.com/issueye
// @version      2.1.0
// @description  优化 GitHub UI 体验
// @author       issueye
// @match        *://*.github.com/*
// @match        https://github.com/*
// @exclude      *://github.com/settings/*
// @run-at       document-start
// @grant        GM_addStyle
// @grant        GM_setValue
// ==/UserScript==

console.log("GitHub helper loaded");
`
	meta := ParseMetadata(code)

	if meta.Name != "GitHub 增强助手" {
		t.Errorf("Name = %q, want 'GitHub 增强助手'", meta.Name)
	}
	if meta.Version != "2.1.0" {
		t.Errorf("Version = %q, want '2.1.0'", meta.Version)
	}
	if meta.Author != "issueye" {
		t.Errorf("Author = %q, want 'issueye'", meta.Author)
	}
	if len(meta.Match) != 2 {
		t.Errorf("Match count = %d, want 2", len(meta.Match))
	}
	if len(meta.Exclude) != 1 || meta.Exclude[0] != "*://github.com/settings/*" {
		t.Errorf("Exclude = %v", meta.Exclude)
	}
	if meta.RunAt != "document-start" {
		t.Errorf("RunAt = %q, want 'document-start'", meta.RunAt)
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		url     string
		want    bool
	}{
		{"*://*/*", "https://github.com/issueye/demo", true},
		{"<all_urls>", "http://localhost:8080/app", true},
		{"https://*.github.com/*", "https://api.github.com/users", true},
		{"https://github.com/*", "https://github.com/login", true},
		{"https://github.com/*", "http://github.com/login", false}, // scheme 不匹配
		{"*://*.bilibili.com/*", "https://www.bilibili.com/video/BV123", true},
		{"*://*.bilibili.com/*", "https://bilibili.com/anime", true},
		{"*://*.bilibili.com/*", "https://youtube.com/watch", false},
	}

	for _, tt := range tests {
		got := MatchPattern(tt.pattern, tt.url)
		if got != tt.want {
			t.Errorf("MatchPattern(%q, %q) = %v, want %v", tt.pattern, tt.url, got, tt.want)
		}
	}
}

func TestScript_MatchesWithExclude(t *testing.T) {
	code := `// ==UserScript==
// @name         测试脚本
// @match        https://github.com/*
// @exclude      https://github.com/settings/*
// ==/UserScript==
`
	s := NewScript(code, true)

	if !s.Matches("https://github.com/issueye") {
		t.Errorf("应当匹配正常 github 页面")
	}
	if s.Matches("https://github.com/settings/profile") {
		t.Errorf("被 exclude 的页面不应当匹配")
	}

	s.Enabled = false
	if s.Matches("https://github.com/issueye") {
		t.Errorf("禁用状态下不应当匹配")
	}
}

func TestManager_CRUD(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_userscripts.json")

	mgr := NewManager(path)
	mgr.Load()

	// 初始应有预设脚本
	list := mgr.List()
	if len(list) == 0 {
		t.Fatalf("预设脚本不应为空")
	}

	// 添加新脚本
	custom := NewScript(`// ==UserScript==
// @name         自定义测试
// @match        *://test.local/*
// ==/UserScript==
console.log('test');`, true)
	if err := mgr.AddOrUpdate(custom); err != nil {
		t.Fatalf("AddOrUpdate 失败: %v", err)
	}

	// 查询新脚本
	got, ok := mgr.Get(custom.ID)
	if !ok || got.Meta.Name != "自定义测试" {
		t.Fatalf("未能按 ID 获取新脚本: %+v", got)
	}

	// 匹配测试
	matching := mgr.GetMatchingScripts("http://test.local/index.html")
	if len(matching) == 0 {
		t.Fatalf("应当命中 test.local 脚本")
	}

	// 切换启用
	enabled, err := mgr.Toggle(custom.ID)
	if err != nil || enabled != false {
		t.Fatalf("Toggle 失败: enabled=%v, err=%v", enabled, err)
	}

	// 删除
	if err := mgr.Delete(custom.ID); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if _, ok := mgr.Get(custom.ID); ok {
		t.Fatalf("删除后不应存在")
	}
}

func TestBuildInjectionBundle(t *testing.T) {
	s := NewScript(`// ==UserScript==
// @name         样式注入测试
// @match        *://*/*
// @grant        GM_addStyle
// ==/UserScript==
GM_addStyle("body { background: red; }");`, true)

	bundle := BuildInjectionBundle([]*Script{s})
	if !strings.Contains(bundle, "__createGMSandbox") {
		t.Errorf("缺少 GM 沙箱 polyfill")
	}
	if !strings.Contains(bundle, "GM_addStyle(\"body { background: red; }\")") {
		t.Errorf("缺少脚本执行体")
	}
}

// 保证引入 os 避免未引用警告
var _ = os.PathSeparator
