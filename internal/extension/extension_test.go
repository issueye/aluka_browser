package extension

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeExt 依据传入 manifest 内容在临时目录中创建一个最小扩展目录。
func writeExt(t *testing.T, manifest string, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const validManifest = `{
	"manifest_version": 3,
	"name": "测试扩展",
	"version": "1.2.0",
	"description": "用于单测的扩展",
	"permissions": ["storage"],
	"content_scripts": [{
		"matches": ["*://*.example.com/*"],
		"exclude_matches": ["*://example.com/settings/*"],
		"js": ["content.js"],
		"css": ["style.css"],
		"run_at": "document_idle"
	}],
	"action": { "default_popup": "popup.html", "default_title": "点我" }
}`

func TestParseManifest(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr string
		check   func(t *testing.T, m *Manifest)
	}{
		{
			name:  "MV3 完整清单",
			input: validManifest,
			check: func(t *testing.T, m *Manifest) {
				if m.Name != "测试扩展" || m.Version != "1.2.0" || m.ManifestVersion != 3 {
					t.Fatalf("基础字段解析错误: %+v", m)
				}
				if len(m.ContentScripts) != 1 || len(m.ContentScripts[0].JS) != 1 {
					t.Fatalf("content_scripts 解析错误: %+v", m.ContentScripts)
				}
				if a := m.ToolbarAction(); a == nil || a.DefaultPopup != "popup.html" {
					t.Fatalf("action 解析错误: %+v", m.Action)
				}
			},
		},
		{
			name: "MV2 browser_action 等价生效",
			input: `{"manifest_version":2,"name":"旧版","version":"0.1",
				"browser_action":{"default_popup":"p.html"}}`,
			check: func(t *testing.T, m *Manifest) {
				if a := m.ToolbarAction(); a == nil || a.DefaultPopup != "p.html" {
					t.Fatal("browser_action 应等价生效")
				}
			},
		},
		{
			name:    "缺少 name",
			input:   `{"manifest_version":3,"version":"1.0"}`,
			wantErr: "name",
		},
		{
			name:    "非法 manifest_version",
			input:   `{"manifest_version":4,"name":"x","version":"1.0"}`,
			wantErr: "manifest_version",
		},
		{
			name:    "content_scripts 缺 matches",
			input:   `{"manifest_version":3,"name":"x","version":"1.0","content_scripts":[{"js":["a.js"]}]}`,
			wantErr: "matches",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := ParseManifest([]byte(tc.input))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("错误 = %v, want 含 %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("意外错误: %v", err)
			}
			tc.check(t, m)
		})
	}
}

func TestLoadFromDirAndMatches(t *testing.T) {
	dir := writeExt(t, validManifest, map[string]string{
		"content.js": "console.log('hello from extension');",
		"style.css":  "body{color:red}",
		"popup.html": "<html></html>",
	})

	e, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir 失败: %v", err)
	}
	if e.ID == "" || len(e.ID) != 8 {
		t.Fatalf("ID 派生异常: %q", e.ID)
	}
	if !e.MatchesURL("https://www.example.com/page") {
		t.Fatal("应命中 example.com")
	}
	if e.MatchesURL("https://example.com/settings/profile") {
		t.Fatal("exclude_matches 应排除 settings 路径")
	}
	if e.MatchesURL("https://other.com/") {
		t.Fatal("不应命中无关站点")
	}

	// 同一目录重复加载 ID 稳定（更新语义）
	e2, err := LoadFromDir(dir)
	if err != nil || e2.ID != e.ID {
		t.Fatalf("同目录 ID 应稳定: %s vs %s (%v)", e.ID, e2.ID, err)
	}

	// 引用文件缺失应报错
	badDir := writeExt(t, validManifest, nil)
	if _, err := LoadFromDir(badDir); err == nil {
		t.Fatal("引用文件缺失时应报错")
	}

	// popup URL 生成（已映射为虚拟域名 https 方案）
	if got := e.PopupURL(); !strings.HasPrefix(got, "https://") || !strings.HasSuffix(got, "popup.html") {
		t.Fatalf("popup URL 异常: %s", got)
	}
	if url, host, dir := e.PopupVirtualURL(); host == "" || dir == "" || !strings.HasPrefix(url, "https://"+host+"/") {
		t.Fatalf("虚拟域名 URL 异常: %s host=%s", url, host)
	}
}

func TestManagerLifecycle(t *testing.T) {
	base := t.TempDir()
	extDir := writeExt(t, validManifest, map[string]string{
		"content.js": "chrome.storage.local.set({n:1}); console.log('injected');",
		"style.css":  "body{color:blue}",
		"popup.html": "<html></html>",
	})

	m := NewManager(base)
	if _, err := m.LoadUnpacked(extDir); err != nil {
		t.Fatalf("LoadUnpacked 失败: %v", err)
	}

	// 同目录重复加载应去重（更新）
	if _, err := m.LoadUnpacked(extDir); err != nil {
		t.Fatalf("重复加载失败: %v", err)
	}
	if got := len(m.List()); got != 1 {
		t.Fatalf("同目录应去重, got %d 条", got)
	}

	// 持久化：新管理器实例从注册表恢复
	m2 := NewManager(base)
	m2.Load()
	if got := len(m2.List()); got != 1 {
		t.Fatalf("注册表恢复失败, got %d 条", got)
	}

	e := m2.List()[0]
	id := e.ID

	// 启停
	enabled, err := m2.Toggle(id)
	if err != nil || enabled {
		t.Fatalf("Toggle 应变为停用: %v %v", enabled, err)
	}

	// 停用后不参与注入
	if code := m2.BuildInjectionForURL("https://www.example.com/"); code != "" {
		t.Fatal("停用后不应生成注入代码")
	}

	// 重新启用后注入包含 polyfill、内容脚本与 CSS
	if _, err := m2.Toggle(id); err != nil {
		t.Fatal(err)
	}
	code := m2.BuildInjectionForURL("https://www.example.com/")
	for _, want := range []string{"__createChromeSandbox", "console.log('injected')", "chrome.storage.local.set", "color:blue", "const chrome ="} {
		if !strings.Contains(code, want) {
			t.Fatalf("注入代码缺少 %q", want)
		}
	}

	// 非匹配 URL 不注入
	if code := m2.BuildInjectionForURL("https://unrelated.org/"); code != "" {
		t.Fatal("非匹配 URL 不应生成注入代码")
	}

	// 删除
	if err := m2.Delete(id); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if _, err := m2.Toggle(id); err == nil {
		t.Fatal("删除后 Toggle 应报错")
	}
}

func TestDeriveIDStableAcrossCase(t *testing.T) {
	// Windows 路径大小写不敏感，ID 派生应归一化
	a := deriveID(`E:\Ext\Demo`)
	b := deriveID(`e:\ext\demo`)
	if a != b {
		t.Fatalf("大小写不同导致 ID 不稳定: %s vs %s", a, b)
	}
}
