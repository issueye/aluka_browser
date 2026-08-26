package ui

import (
	"hash/fnv"
	"image/color"
	"strings"
)

// SiteBadge 是无法获取真实 favicon 时的站点占位头像：
// 从 URL 提取主机名首字符，按哈希从固定色板取一对配色，
// 保证同一站点的头像颜色稳定。
type SiteBadge struct {
	Initial string
	BG      [2]color.NRGBA // 背景 / 文字
}

// hostOf 从 URL 中提取主机名（去协议、路径、端口、www 前缀）。
func hostOf(rawURL string) string {
	s := rawURL
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, ":"); i >= 0 && isDigits(s[i+1:]) {
		s = s[:i]
	}
	s = strings.TrimPrefix(s, "www.")
	return s
}

// isDigits 判断字符串是否全为数字（用于识别 host:port 中的端口）。
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// SiteBadgeFor 计算 URL 对应的占位头像。空 URL 返回 ok=false。
func SiteBadgeFor(url string) (SiteBadge, bool) {
	host := hostOf(url)
	if host == "" {
		return SiteBadge{}, false
	}
	for _, r := range host {
		initial := strings.ToUpper(string(r))
		h := fnv.New32a()
		_, _ = h.Write([]byte(host))
		pair := SiteBadgePalette[int(h.Sum32())%len(SiteBadgePalette)]
		return SiteBadge{Initial: initial, BG: pair}, true
	}
	return SiteBadge{}, false
}
