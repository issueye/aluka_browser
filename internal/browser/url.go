package browser

import (
	"net/url"
	"strings"
)

// HomeURL 默认主页。
const HomeURL = "https://github.com"

// NormalizeInputURL 将地址栏输入归一化为可导航 URL：
//  1. 已带协议的直接返回；
//  2. 含 "." 且无空格的视作域名，补 https://；
//  3. host:port 形式（如 localhost:8080）视作网址；
//  4. 其余一律作为搜索关键词送搜索引擎（转义空格）。
func NormalizeInputURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s
	}
	if strings.Contains(s, ".") && !strings.Contains(s, " ") {
		return "https://" + s
	}
	if host, port, ok := strings.Cut(s, ":"); ok &&
		host != "" && !strings.Contains(host, " ") && isDigits(port) {
		return "https://" + s
	}
	return "https://duckduckgo.com/?q=" + url.QueryEscape(s)
}

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
