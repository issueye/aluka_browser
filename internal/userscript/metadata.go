// Package userscript 提供类似篡改猴（Tampermonkey）的用户脚本解析、匹配、存储与注入运行时。
package userscript

import (
	"bufio"
	"strings"
)

// Metadata 用户脚本的标准元数据块（从 // ==UserScript== 解析而来）。
type Metadata struct {
	Name        string   `json:"name"`
	Namespace   string   `json:"namespace"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Match       []string `json:"match"`
	Include     []string `json:"include"`
	Exclude     []string `json:"exclude"`
	RunAt       string   `json:"run_at"` // document-start, document-end, document-idle
	Grant       []string `json:"grant"`
}

// DefaultMetadata 返回默认初始化的元数据。
func DefaultMetadata() Metadata {
	return Metadata{
		Name:        "新用户脚本",
		Namespace:   "https://gio-browser.local/",
		Version:     "1.0.0",
		Description: "自定义用户增强脚本",
		Author:      "User",
		Match:       []string{"*://*/*"},
		RunAt:       "document-end",
		Grant:       []string{"GM_addStyle", "GM_log", "GM_setValue", "GM_getValue"},
	}
}

// ParseMetadata 从脚本源码中解析 // ==UserScript== ... // ==/UserScript== 元数据块。
func ParseMetadata(code string) Metadata {
	meta := DefaultMetadata()
	meta.Match = nil
	meta.Include = nil
	meta.Exclude = nil
	meta.Grant = nil

	scanner := bufio.NewScanner(strings.NewReader(code))
	inMetaBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "// ==UserScript==") {
			inMetaBlock = true
			continue
		}
		if strings.HasPrefix(line, "// ==/UserScript==") {
			inMetaBlock = false
			break
		}
		if !inMetaBlock {
			continue
		}

		if !strings.HasPrefix(line, "//") {
			continue
		}

		// 去掉 // 前缀
		content := strings.TrimSpace(strings.TrimPrefix(line, "//"))
		if !strings.HasPrefix(content, "@") {
			continue
		}

		parts := strings.Fields(content)
		if len(parts) < 1 {
			continue
		}

		key := parts[0]
		val := ""
		if len(parts) > 1 {
			val = strings.TrimSpace(content[len(key):])
		}

		switch key {
		case "@name":
			meta.Name = val
		case "@namespace":
			meta.Namespace = val
		case "@version":
			meta.Version = val
		case "@description":
			meta.Description = val
		case "@author":
			meta.Author = val
		case "@match":
			if val != "" {
				meta.Match = append(meta.Match, val)
			}
		case "@include":
			if val != "" {
				meta.Include = append(meta.Include, val)
			}
		case "@exclude":
			if val != "" {
				meta.Exclude = append(meta.Exclude, val)
			}
		case "@run-at":
			if val != "" {
				meta.RunAt = val
			}
		case "@grant":
			if val != "" {
				meta.Grant = append(meta.Grant, val)
			}
		}
	}

	if meta.Name == "" {
		meta.Name = "未命名脚本"
	}
	if len(meta.Match) == 0 && len(meta.Include) == 0 {
		meta.Match = []string{"*://*/*"}
	}
	if meta.RunAt == "" {
		meta.RunAt = "document-end"
	}

	return meta
}

// GenerateMetadataHeader 根据元数据生成标准的 // ==UserScript== 头部注释块。
func GenerateMetadataHeader(m Metadata) string {
	var sb strings.Builder
	sb.WriteString("// ==UserScript==\n")
	sb.WriteString("// @name         " + m.Name + "\n")
	if m.Namespace != "" {
		sb.WriteString("// @namespace    " + m.Namespace + "\n")
	}
	if m.Version != "" {
		sb.WriteString("// @version      " + m.Version + "\n")
	}
	if m.Description != "" {
		sb.WriteString("// @description  " + m.Description + "\n")
	}
	if m.Author != "" {
		sb.WriteString("// @author       " + m.Author + "\n")
	}
	for _, match := range m.Match {
		sb.WriteString("// @match        " + match + "\n")
	}
	for _, inc := range m.Include {
		sb.WriteString("// @include      " + inc + "\n")
	}
	for _, exc := range m.Exclude {
		sb.WriteString("// @exclude      " + exc + "\n")
	}
	if m.RunAt != "" {
		sb.WriteString("// @run-at       " + m.RunAt + "\n")
	}
	for _, g := range m.Grant {
		sb.WriteString("// @grant        " + g + "\n")
	}
	sb.WriteString("// ==/UserScript==\n")
	return sb.String()
}
