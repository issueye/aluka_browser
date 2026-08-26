// Package config 提供浏览器用户配置的本地持久化与网络代理环境变量管理。
package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Config 浏览器全局配置。
type Config struct {
	ProxyEnabled bool   `json:"proxy_enabled"` // 是否启用网络代理
	ProxyType    string `json:"proxy_type"`    // 代理类型："http" 或 "socks5"
	ProxyServer  string `json:"proxy_server"`  // 代理服务器地址，如 "127.0.0.1:7890"
	ProxyBypass  string `json:"proxy_bypass"`  // 不走代理的白名单列表，如 "<local>;localhost;127.0.0.1"
}

var (
	mu        sync.RWMutex
	globalCfg Config
)

// DefaultConfig 返回默认初始化配置。
func DefaultConfig() Config {
	return Config{
		ProxyEnabled: false,
		ProxyType:    "http",
		ProxyServer:  "127.0.0.1:7890",
		ProxyBypass:  "<local>;localhost;127.0.0.1",
	}
}

// ConfigFilePath 获取配置文件的存储绝对路径。
func ConfigFilePath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.TempDir()
	}
	return filepath.Join(appData, "gio-browser", "config.json")
}

// Load 从磁盘读取配置，若文件不存在则使用默认配置并持久化，同时应用代理环境变量。
func Load() Config {
	mu.Lock()
	defer mu.Unlock()

	cfgPath := ConfigFilePath()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		globalCfg = DefaultConfig()
		_ = saveLocked(globalCfg)
		applyProxyEnvLocked(globalCfg)
		return globalCfg
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("[配置] 解析配置文件失败，采用默认值: %v", err)
		globalCfg = DefaultConfig()
	} else {
		// 校验默认缺省字段
		if cfg.ProxyType == "" {
			cfg.ProxyType = "http"
		}
		if cfg.ProxyBypass == "" {
			cfg.ProxyBypass = "<local>;localhost;127.0.0.1"
		}
		globalCfg = cfg
	}

	applyProxyEnvLocked(globalCfg)
	return globalCfg
}

// Current 获取当前内存中的配置副本。
func Current() Config {
	mu.RLock()
	defer mu.RUnlock()
	return globalCfg
}

// Save 保存新配置到磁盘，并更新内存和代理环境变量。
func Save(cfg Config) error {
	mu.Lock()
	defer mu.Unlock()

	globalCfg = cfg
	if err := saveLocked(cfg); err != nil {
		return err
	}
	applyProxyEnvLocked(cfg)
	return nil
}

// saveLocked 将配置写入 JSON 文件（必须在持有 mu 锁下调用）。
func saveLocked(cfg Config) error {
	cfgPath := ConfigFilePath()
	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cfgPath, data, 0644)
}

// BuildProxyArgs 根据配置生成 WebView2/Chromium 命令行代理参数。
func BuildProxyArgs(cfg Config) string {
	if !cfg.ProxyEnabled || strings.TrimSpace(cfg.ProxyServer) == "" {
		return ""
	}

	server := strings.TrimSpace(cfg.ProxyServer)
	pType := strings.ToLower(strings.TrimSpace(cfg.ProxyType))
	if pType == "socks5" {
		if !strings.HasPrefix(server, "socks5://") {
			server = "socks5://" + server
		}
	} else {
		if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
			server = "http://" + server
		}
	}

	args := "--proxy-server=" + server
	bypass := strings.TrimSpace(cfg.ProxyBypass)
	if bypass != "" {
		args += " --proxy-bypass-list=" + bypass
	}
	return args
}

// applyProxyEnvLocked 注入或清除 WebView2 相关的代理环境变量。
func applyProxyEnvLocked(cfg Config) {
	args := BuildProxyArgs(cfg)
	if args != "" {
		_ = os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", args)
		log.Printf("[代理] 已启用网络代理参数: %s", args)
	} else {
		_ = os.Unsetenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS")
		log.Printf("[代理] 已关闭网络代理（直连模式）")
	}
}
