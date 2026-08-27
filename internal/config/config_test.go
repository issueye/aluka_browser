package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func TestLoadSaveRoundtrip(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	cfg := Load()
	if cfg.HomePage != DefaultHomePage {
		t.Errorf("首次加载主页 = %q, want %q", cfg.HomePage, DefaultHomePage)
	}
	if len(cfg.QuickLinks) != len(DefaultQuickLinks()) {
		t.Fatalf("首次加载快捷访问数 = %d, want %d", len(cfg.QuickLinks), len(DefaultQuickLinks()))
	}

	cfg.HomePage = "https://example.com"
	cfg.QuickLinks = append(cfg.QuickLinks, QuickLink{Name: "测试", URL: "https://t.io"})
	if err := Save(cfg); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	again := Load()
	if again.HomePage != "https://example.com" {
		t.Errorf("持久化主页丢失: %q", again.HomePage)
	}
	if len(again.QuickLinks) != len(cfg.QuickLinks) {
		t.Errorf("持久化快捷访问数不一致: %d vs %d", len(again.QuickLinks), len(cfg.QuickLinks))
	}
	last := again.QuickLinks[len(again.QuickLinks)-1]
	if last.Name != "测试" || last.URL != "https://t.io" {
		t.Errorf("新增条目未持久化: %+v", last)
	}
}

func TestLoadInvalidJSONFallsBackToDefaults(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	if err := writeFile(ConfigFilePath(), []byte("{ 不是 JSON")); err != nil {
		t.Fatal(err)
	}

	cfg := Load()
	if cfg.HomePage != DefaultHomePage || len(cfg.QuickLinks) != len(DefaultQuickLinks()) {
		t.Errorf("损坏配置应回退默认值, got %+v", cfg)
	}
}

func TestLoadLegacyConfigIgnoresUnknownFields(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	// 历史版本配置：含 proxy_* 字段，应忽略未知字段并补默认主页
	legacy := `{"proxy_enabled":true,"proxy_server":"127.0.0.1:7897"}`
	if err := writeFile(ConfigFilePath(), []byte(legacy)); err != nil {
		t.Fatal(err)
	}

	cfg := Load()
	if cfg.HomePage != DefaultHomePage {
		t.Errorf("旧配置应补默认主页, got %q", cfg.HomePage)
	}
	if len(cfg.QuickLinks) == 0 {
		t.Error("旧配置应补默认快捷访问")
	}
}
