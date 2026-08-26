package ui

import "testing"

func TestHostOf(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://github.com", "github.com"},
		{"https://www.google.com/search?q=x", "google.com"},
		{"http://localhost:8080/app", "localhost"},
		{"bilibili.com", "bilibili.com"},
		{"", ""},
	}
	for _, c := range cases {
		if got := hostOf(c.in); got != c.want {
			t.Errorf("hostOf(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSiteBadgeFor(t *testing.T) {
	if _, ok := SiteBadgeFor(""); ok {
		t.Error("空 URL 不应产生头像")
	}

	b1, ok := SiteBadgeFor("https://github.com/gioui")
	if !ok {
		t.Fatal("合法 URL 应产生头像")
	}
	if b1.Initial != "G" {
		t.Errorf("首字母 = %q, want G", b1.Initial)
	}

	b2, _ := SiteBadgeFor("https://www.github.com/issues")
	if b2.BG != b1.BG {
		t.Errorf("同一站点（忽略 www 与路径）应取相同配色: %+v vs %+v", b1.BG, b2.BG)
	}
}
