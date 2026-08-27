// Package extension 实现参考 Chrome/Edge 的"加载已解压扩展"式插件系统：
// 解析 manifest.json、按 content_scripts 匹配规则向网页注入脚本与样式，
// 并为内容脚本提供 chrome.* API 的最小兼容沙箱。
package extension

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// RunAt 内容脚本的注入时机声明。
// 注意：当前注入固定发生在页面导航完成时，该字段仅解析记录、未严格生效
// （与用户脚本 @run-at 的现状一致）。
type RunAt string

const (
	RunAtDocumentStart RunAt = "document_start"
	RunAtDocumentEnd   RunAt = "document_end"
	RunAtDocumentIdle  RunAt = "document_idle"
)

// Manifest 扩展清单（manifest.json）中本系统支持的子集。
type Manifest struct {
	ManifestVersion   int               `json:"manifest_version"`          // 2 或 3
	Name              string            `json:"name"`                      // 扩展名称（必填）
	Version           string            `json:"version"`                   // 版本（必填）
	Description       string            `json:"description,omitempty"`     // 描述
	Icons             map[string]string `json:"icons,omitempty"`           // 尺寸 → 相对路径（当前未渲染）
	Permissions       []string          `json:"permissions,omitempty"`     // 声明的权限（v1 仅展示不做隔离）
	ContentScripts    []ContentScript   `json:"content_scripts,omitempty"` // 内容脚本定义
	Action            *Action           `json:"action,omitempty"`          // MV3 工具按钮
	BrowserAction     *Action           `json:"browser_action,omitempty"`  // MV2 工具按钮（等价处理）
	MinimumChromeVer  string            `json:"minimum_chrome_version,omitempty"`
	Author            string            `json:"author,omitempty"`
	HomepageURL       string            `json:"homepage_url,omitempty"`
}

// ContentScript 单组内容脚本声明。
type ContentScript struct {
	Matches       []string `json:"matches"`                  // 匹配模式（必填组）
	ExcludeMatches []string `json:"exclude_matches,omitempty"` // 排除模式（优先级最高）
	JS            []string `json:"js,omitempty"`             // 注入的 JS 文件（相对扩展目录）
	CSS           []string `json:"css,omitempty"`            // 注入的 CSS 文件
	RunAt         RunAt    `json:"run_at,omitempty"`         // 注入时机（当前未严格生效）
	AllFrames     bool     `json:"all_frames,omitempty"`     // v1 忽略，恒为顶层框架
}

// Action 工具栏按钮定义（MV2 browser_action 与 MV3 action 同构处理）。
type Action struct {
	DefaultTitle string            `json:"default_title,omitempty"`
	DefaultPopup string            `json:"default_popup,omitempty"` // 点击后打开的页面（v1 以新标签页呈现）
	DefaultIcon  map[string]string `json:"default_icon,omitempty"`
}

// ToolbarAction 返回生效的工具按钮定义（MV3 action 优先于 MV2 browser_action）。
func (m *Manifest) ToolbarAction() *Action {
	if m.Action != nil {
		return m.Action
	}
	return m.BrowserAction
}

// ParseManifest 从字节流解析并校验扩展清单。
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("解析 manifest.json 失败: %w", err)
	}
	if m.Name == "" {
		return nil, fmt.Errorf("manifest.json 缺少必填字段 name")
	}
	if m.Version == "" {
		return nil, fmt.Errorf("manifest.json 缺少必填字段 version")
	}
	if m.ManifestVersion != 2 && m.ManifestVersion != 3 {
		return nil, fmt.Errorf("不支持的 manifest_version: %d（仅支持 2/3）", m.ManifestVersion)
	}
	for _, cs := range m.ContentScripts {
		if len(cs.Matches) == 0 {
			return nil, fmt.Errorf("扩展 %q 的 content_scripts 缺少 matches 声明", m.Name)
		}
	}
	return &m, nil
}

// loadManifestFrom 从扩展目录读取清单文件。
func loadManifestFrom(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("读取 manifest.json 失败: %w", err)
	}
	return ParseManifest(data)
}
