// Package config 提供浏览器用户配置的本地持久化（主页、快捷访问等）。
package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// QuickLink 快捷访问条目（快捷访问栏与设置中心「快捷访问」共用）。
type QuickLink struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Config 浏览器全局配置。
type Config struct {
	HomePage   string      `json:"home_page"`  // 主页地址（新标签/回主页/最后标签回落）
	QuickLinks []QuickLink `json:"quick_links"` // 快捷访问列表

	// 插件侧栏
	PluginSidebarVisible *bool `json:"plugin_sidebar_visible,omitempty"` // nil 表示缺省（收起）
	PluginSidebarWidth   int   `json:"plugin_sidebar_width,omitempty"`   // dp，默认 300
}

// PluginSidebarDefaults 返回插件侧栏的默认尺寸与可见性。
func PluginSidebarDefaults() (visible bool, width int) {
	return false, 300
}

// DefaultHomePage 默认主页地址。
const DefaultHomePage = "https://github.com"

// DefaultQuickLinks 默认快捷访问列表（首次运行写入）。
func DefaultQuickLinks() []QuickLink {
	return []QuickLink{
		{Name: "GitHub", URL: "https://github.com"},
		{Name: "Google", URL: "https://www.google.com"},
		{Name: "Bilibili", URL: "https://www.bilibili.com"},
		{Name: "MDN Web", URL: "https://developer.mozilla.org"},
		{Name: "HackerNews", URL: "https://news.ycombinator.com"},
	}
}

// DefaultConfig 返回默认初始化配置。
func DefaultConfig() Config {
	return Config{
		HomePage:   DefaultHomePage,
		QuickLinks: DefaultQuickLinks(),
	}
}

var (
	mu        sync.RWMutex
	globalCfg Config
)

// ConfigFilePath 获取配置文件的存储绝对路径。
func ConfigFilePath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.TempDir()
	}
	return filepath.Join(appData, "gio-browser", "config.json")
}

// Load 从磁盘读取配置，若文件不存在则使用默认配置并持久化。
// 未知字段（如历史版本的 proxy_*）会被 json 忽略。
func Load() Config {
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(ConfigFilePath())
	if err != nil {
		globalCfg = DefaultConfig()
		_ = saveLocked(globalCfg)
		return globalCfg
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("[配置] 解析配置文件失败，采用默认值: %v", err)
		cfg = DefaultConfig()
	}
	if cfg.HomePage == "" {
		cfg.HomePage = DefaultHomePage
	}
	if len(cfg.QuickLinks) == 0 {
		cfg.QuickLinks = DefaultQuickLinks()
	}
	if cfg.PluginSidebarWidth == 0 {
		_, defW := PluginSidebarDefaults()
		cfg.PluginSidebarWidth = defW
	}
	globalCfg = cfg
	return globalCfg
}

// Current 获取当前内存中的配置副本。
func Current() Config {
	mu.RLock()
	defer mu.RUnlock()
	return globalCfg
}

// Save 保存新配置到磁盘并更新内存。
func Save(cfg Config) error {
	mu.Lock()
	defer mu.Unlock()

	globalCfg = cfg
	return saveLocked(cfg)
}

// saveLocked 将配置写入 JSON 文件（必须在持有 mu 锁下调用）。
func saveLocked(cfg Config) error {
	cfgPath := ConfigFilePath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cfgPath, data, 0644)
}
