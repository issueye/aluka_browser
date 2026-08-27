package extension

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gio-browser/internal/userscript"
)

// Extension 一个已加载的"已解压扩展"。
//
// 与 Chrome 开发者模式一致，本系统直接指向源目录而不复制文件；
// ID 由目录绝对路径派生，同一目录重复加载视为更新。
type Extension struct {
	ID          string   `json:"id"`
	Dir         string   `json:"dir"` // 扩展源目录绝对路径
	Enabled     bool     `json:"enabled"`
	InstalledAt int64    `json:"installed_at"`
	Manifest    Manifest `json:"manifest"`
}

// LoadFromDir 从磁盘加载一个扩展目录（读取并校验 manifest 与引用文件）。
func LoadFromDir(dir string) (*Extension, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("解析扩展路径失败: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("扩展目录不存在: %s", abs)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("扩展路径不是目录: %s", abs)
	}

	manifest, err := loadManifestFrom(abs)
	if err != nil {
		return nil, err
	}

	// 校验 content_scripts / popup 引用的文件真实存在，尽早暴露路径错误
	for _, cs := range manifest.ContentScripts {
		for _, f := range append(append([]string{}, cs.JS...), cs.CSS...) {
			if _, err := os.Stat(filepath.Join(abs, filepath.FromSlash(f))); err != nil {
				return nil, fmt.Errorf("扩展 %q 引用的文件缺失: %s", manifest.Name, f)
			}
		}
	}
	if a := manifest.ToolbarAction(); a != nil && a.DefaultPopup != "" {
		if _, err := os.Stat(filepath.Join(abs, filepath.FromSlash(a.DefaultPopup))); err != nil {
			return nil, fmt.Errorf("扩展 %q 的 popup 页面缺失: %s", manifest.Name, a.DefaultPopup)
		}
	}

	return &Extension{
		ID:          deriveID(abs),
		Dir:         abs,
		Enabled:     true,
		InstalledAt: time.Now().Unix(),
		Manifest:    *manifest,
	}, nil
}

// deriveID 由目录绝对路径派生 8 位十六进制 ID（同一目录恒定，仿 Chrome 语义）。
func deriveID(absDir string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.ToLower(absDir)))
	return fmt.Sprintf("%016x", h.Sum64())[:8]
}

// MatchesURL 判断扩展的任一 content_scripts 组是否命中目标 URL
// （排除规则优先，匹配算法与用户脚本共用 userscript.MatchPattern）。
func (e *Extension) MatchesURL(targetURL string) bool {
	if !e.Enabled || strings.TrimSpace(targetURL) == "" {
		return false
	}
	for _, cs := range e.Manifest.ContentScripts {
		excluded := false
		for _, exc := range cs.ExcludeMatches {
			if userscript.MatchPattern(exc, targetURL) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		for _, m := range cs.Matches {
			if userscript.MatchPattern(m, targetURL) {
				return true
			}
		}
	}
	return false
}

// PopupURL 返回工具按钮 popup 页面的 file:// URL（未配置则返回空串）。
func (e *Extension) PopupURL() string {
	a := e.Manifest.ToolbarAction()
	if a == nil || a.DefaultPopup == "" {
		return ""
	}
	p := filepath.ToSlash(filepath.Join(e.Dir, filepath.FromSlash(a.DefaultPopup)))
	return "file:///" + strings.TrimPrefix(p, "/")
}
