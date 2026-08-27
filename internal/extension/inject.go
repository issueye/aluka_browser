package extension

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// chromeSandboxPolyfill 注入页面的 chrome.* 兼容沙箱工厂。
//
// 重要：沙箱对象在各扩展的 IIFE 内以局部变量 `chrome` 遮蔽页面原生
// window.chrome（WebView2 通道），绝不覆盖全局，避免破坏宿主通信。
// storage 基于 localStorage 的扩展级命名空间持久化，API 为 Promise 风格。
const chromeSandboxPolyfill = `
(function() {
	if (window.__GIO_CHROME_SANDBOX_INITIALIZED__) return;
	window.__GIO_CHROME_SANDBOX_INITIALIZED__ = true;

	window.__createChromeSandbox = function(meta) {
		const prefix = '__gio_ext_' + meta.id + '_';

		function storageGet(keys) {
			return new Promise(function(resolve) {
				const out = {};
				if (keys === null || keys === undefined) {
					for (let i = 0; i < localStorage.length; i++) {
						const k = localStorage.key(i);
						if (k && k.startsWith(prefix)) {
							try { out[k.substring(prefix.length)] = JSON.parse(localStorage.getItem(k)); } catch (e) {}
						}
					}
					resolve(out); return;
				}
				const list = typeof keys === 'string' ? [keys] : keys;
				list.forEach(function(k) {
					const v = localStorage.getItem(prefix + k);
					if (v !== null) { try { out[k] = JSON.parse(v); } catch (e) {} }
				});
				resolve(out);
			});
		}
		function storageSet(items) {
			return new Promise(function(resolve) {
				for (const k in items) localStorage.setItem(prefix + k, JSON.stringify(items[k]));
				resolve();
			});
		}
		function storageRemove(keys) {
			return new Promise(function(resolve) {
				(typeof keys === 'string' ? [keys] : keys).forEach(function(k) {
					localStorage.removeItem(prefix + k);
				});
				resolve();
			});
		}
		function storageClear() {
			return new Promise(function(resolve) {
				const kill = [];
				for (let i = 0; i < localStorage.length; i++) {
					const k = localStorage.key(i);
					if (k && k.startsWith(prefix)) kill.push(k);
				}
				kill.forEach(function(k) { localStorage.removeItem(k); });
				resolve();
			});
		}
		const storageArea = { get: storageGet, set: storageSet, remove: storageRemove, clear: storageClear };

		return {
			runtime: {
				id: meta.id,
				getManifest: function() { return meta.manifest; },
				sendMessage: function(msg, cb) {
					try {
						window.chrome.webview.postMessage(JSON.stringify({
							type: 'ext_message', extId: meta.id, extName: meta.name,
							url: window.location.href, payload: msg
						}));
					} catch (e) {}
					if (typeof cb === 'function') cb(undefined);
					return Promise.resolve(undefined);
				},
				onMessage: { addListener: function() {} },
				onInstalled: { addListener: function() {} },
				getURL: function(path) {
					console.warn('[gio-browser] chrome.runtime.getURL 暂不支持:', path);
					return '';
				}
			},
			storage: { local: storageArea, sync: storageArea },
			i18n: { getMessage: function(key) { return key; } }
		};
	};
})();
`

// BuildInjectionForURL 为目标 URL 生成全部命中扩展的注入代码（无命中返回空串）。
func (m *Manager) BuildInjectionForURL(targetURL string) string {
	matching := m.GetMatchingExtensions(targetURL)
	if len(matching) == 0 {
		return ""
	}
	return BuildInjectionBundle(matching)
}

// BuildInjectionBundle 构建可直接在 WebView2 中执行的一体化注入包：
// chrome 沙箱 polyfill + 每个扩展一段隔离 IIFE（局部遮蔽 chrome）。
func BuildInjectionBundle(exts []*Extension) string {
	if len(exts) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(chromeSandboxPolyfill)
	sb.WriteString("\n")

	for _, e := range exts {
		if !e.Enabled {
			continue
		}
		sb.WriteString("/* --- [Extension]: " + e.Manifest.Name + " (" + e.ID + ") --- */\n")
		sb.WriteString("(function(){\n")

		meta := map[string]any{
			"id":       e.ID,
			"name":     e.Manifest.Name,
			"version":  e.Manifest.Version,
			"manifest": e.Manifest,
		}
		metaJSON, _ := json.Marshal(meta)
		sb.WriteString(fmt.Sprintf("\tconst __extMeta = %s;\n", metaJSON))
		sb.WriteString("\tconst chrome = window.__createChromeSandbox(__extMeta);\n\n")

		// CSS 以 <style> 方式先行注入
		for _, cs := range e.Manifest.ContentScripts {
			for _, f := range cs.CSS {
				css, err := os.ReadFile(filepath.Join(e.Dir, filepath.FromSlash(f)))
				if err != nil {
					continue
				}
				cssJSON, _ := json.Marshal(string(css))
				sb.WriteString("\ttry{ (function(){ const s=document.createElement('style'); s.textContent=" + string(cssJSON) + "; (document.head||document.documentElement).appendChild(s); })(); }catch(err){ console.error('[Extension CSS " + e.Manifest.Name + "]', err); }\n")
			}
		}

		// JS 内容脚本按声明顺序执行，单个文件异常不影响其余
		for _, cs := range e.Manifest.ContentScripts {
			for _, f := range cs.JS {
				code, err := os.ReadFile(filepath.Join(e.Dir, filepath.FromSlash(f)))
				if err != nil {
					continue
				}
				sb.WriteString("\ttry{\n" + string(code) + "\n\t}catch(err){ console.error('[Extension JS " + e.Manifest.Name + "/" + f + "]', err); }\n")
			}
		}

		sb.WriteString("})();\n\n")
	}
	return sb.String()
}
