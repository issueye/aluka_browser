package userscript

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Script 代表一个独立的用户脚本对象。
type Script struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`

	// 缓存从 Code 解析出的元数据
	Meta Metadata `json:"meta"`
}

// NewScript 基于代码和启用状态创建一个新的脚本实例。
func NewScript(code string, enabled bool) *Script {
	meta := ParseMetadata(code)
	now := time.Now().Unix()
	id := fmt.Sprintf("us-%d", time.Now().UnixNano())

	return &Script{
		ID:        id,
		Code:      code,
		Enabled:   enabled,
		CreatedAt: now,
		UpdatedAt: now,
		Meta:      meta,
	}
}

// UpdateCode 更新脚本代码并重新解析元数据。
func (s *Script) UpdateCode(code string) {
	s.Code = code
	s.Meta = ParseMetadata(code)
	s.UpdatedAt = time.Now().Unix()
}

// Matches 判断脚本是否命中指定的网页 URL。
func (s *Script) Matches(targetURL string) bool {
	if !s.Enabled || strings.TrimSpace(targetURL) == "" {
		return false
	}

	// 1. 检查 Exclude 排除规则（优先级最高）
	for _, exc := range s.Meta.Exclude {
		if MatchPattern(exc, targetURL) {
			return false
		}
	}

	// 2. 检查 Match 规则
	for _, m := range s.Meta.Match {
		if MatchPattern(m, targetURL) {
			return true
		}
	}

	// 3. 检查 Include 规则
	for _, inc := range s.Meta.Include {
		if MatchPattern(inc, targetURL) {
			return true
		}
	}

	return false
}

// MatchPattern 判断 pattern 是否匹配 targetURL（遵循 Tampermonkey 规范）。
// 支持类似 "*://*.github.com/*" 或 "<all_urls>" 或 "https://*/*" 或直接包含正则/通配符。
func MatchPattern(pattern, targetURL string) bool {
	p := strings.TrimSpace(pattern)
	u := strings.TrimSpace(targetURL)

	if p == "" || u == "" {
		return false
	}
	if p == "<all_urls>" || p == "*://*/*" || p == "*" {
		return true
	}

	// 解析 URL
	parsedURL, err := url.Parse(u)
	if err != nil {
		return false
	}

	// 如果 pattern 包含 scheme 模式，如 "*://host/path"
	schemeSep := strings.Index(p, "://")
	if schemeSep != -1 {
		schemePattern := p[:schemeSep]
		restPattern := p[schemeSep+3:]

		// 匹配 Scheme
		if schemePattern != "*" && schemePattern != parsedURL.Scheme {
			return false
		}

		pathSep := strings.Index(restPattern, "/")
		hostPattern := restPattern
		urlPathPattern := "/*"
		if pathSep != -1 {
			hostPattern = restPattern[:pathSep]
			urlPathPattern = restPattern[pathSep:]
		}

		// 匹配 Host
		if !matchHost(hostPattern, parsedURL.Host) {
			return false
		}

		// 匹配 Path（含 Query）
		fullPath := parsedURL.EscapedPath()
		if fullPath == "" {
			fullPath = "/"
		}
		if parsedURL.RawQuery != "" {
			fullPath += "?" + parsedURL.RawQuery
		}
		return matchWildcard(urlPathPattern, fullPath)
	}

	// 纯通配符模式匹配
	return matchWildcard(p, u)
}

// matchHost 匹配 Host，支持 "*.example.com" 或 "example.com" 或 "*"
func matchHost(pattern, host string) bool {
	// 去除端口号
	if colon := strings.Index(host, ":"); colon != -1 {
		host = host[:colon]
	}

	p := strings.ToLower(pattern)
	h := strings.ToLower(host)

	if p == "*" || p == h {
		return true
	}

	if strings.HasPrefix(p, "*.") {
		domain := p[2:]
		return h == domain || strings.HasSuffix(h, "."+domain)
	}

	return matchWildcard(p, h)
}

// matchWildcard 通配符匹配算法（将 * 转为 .* 正则）
func matchWildcard(pattern, s string) bool {
	var reBuilder strings.Builder
	reBuilder.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			reBuilder.WriteString(".*")
		case '?', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '.', '\\':
			reBuilder.WriteString("\\" + string(c))
		default:
			reBuilder.WriteByte(c)
		}
	}
	reBuilder.WriteString("$")

	re, err := regexp.Compile(reBuilder.String())
	if err != nil {
		return false
	}
	return re.MatchString(s)
}
