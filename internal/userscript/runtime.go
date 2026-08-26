package userscript

import (
	"encoding/json"
	"fmt"
	"strings"
)

// gmPolyfillScript 是注入到网页中的标准 GM_* API 沙箱环境。
const gmPolyfillScript = `
(function() {
	if (window.__GIO_GM_SANDBOX_INITIALIZED__) return;
	window.__GIO_GM_SANDBOX_INITIALIZED__ = true;

	// 创建带命名空间隔离的持久化存储引擎（基于 localStorage）
	function getStorageKey(scriptId, key) {
		return '__gio_gm_val_' + scriptId + '_' + key;
	}

	window.__createGMSandbox = function(meta, scriptId) {
		const GM_info = {
			script: {
				name: meta.name || 'UserScript',
				version: meta.version || '1.0.0',
				namespace: meta.namespace || '',
				description: meta.description || '',
				author: meta.author || '',
				matches: meta.match || []
			},
			scriptHandler: 'Gio Browser TamperEngine',
			version: '1.0.0'
		};

		function GM_addStyle(css) {
			const head = document.head || document.getElementsByTagName('head')[0] || document.documentElement;
			const style = document.createElement('style');
			style.type = 'text/css';
			style.appendChild(document.createTextNode(css));
			head.appendChild(style);
			return style;
		}

		function GM_setValue(key, value) {
			try {
				localStorage.setItem(getStorageKey(scriptId, key), JSON.stringify(value));
			} catch (e) {
				console.warn('[Userscript GM_setValue Error]', e);
			}
		}

		function GM_getValue(key, defaultValue) {
			try {
				const val = localStorage.getItem(getStorageKey(scriptId, key));
				if (val === null) return defaultValue;
				return JSON.parse(val);
			} catch (e) {
				return defaultValue;
			}
		}

		function GM_deleteValue(key) {
			try {
				localStorage.removeItem(getStorageKey(scriptId, key));
			} catch (e) {}
		}

		function GM_listValues() {
			const prefix = '__gio_gm_val_' + scriptId + '_';
			const res = [];
			try {
				for (let i = 0; i < localStorage.length; i++) {
					const k = localStorage.key(i);
					if (k && k.startsWith(prefix)) {
						res.push(k.substring(prefix.length));
					}
				}
			} catch (e) {}
			return res;
		}

		function GM_log(...args) {
			console.log('%c[' + (meta.name || 'UserScript') + ']', 'color: #3b82f6; font-weight: bold;', ...args);
		}

		function GM_setClipboard(text) {
			if (navigator.clipboard && navigator.clipboard.writeText) {
				navigator.clipboard.writeText(text);
			}
		}

		function GM_registerMenuCommand(caption, onClick) {
			// 预留菜单注册
			console.log('[Userscript Menu Registered]', caption);
		}

		return {
			GM_info,
			GM_addStyle,
			GM_setValue,
			GM_getValue,
			GM_deleteValue,
			GM_listValues,
			GM_log,
			GM_setClipboard,
			GM_registerMenuCommand,
			unsafeWindow: window
		};
	};
})();
`

// BuildInjectionBundle 构建用于直接在 WebView2 中执行的一体化脚本包。
func BuildInjectionBundle(scripts []*Script) string {
	if len(scripts) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(gmPolyfillScript)
	sb.WriteString("\n\n")

	for _, s := range scripts {
		if !s.Enabled {
			continue
		}

		metaJSON, _ := json.Marshal(s.Meta)
		sb.WriteString("/* --- [Userscript]: " + s.Meta.Name + " (" + s.ID + ") --- */\n")
		sb.WriteString("(function() {\n")
		sb.WriteString(fmt.Sprintf("\tconst __meta = %s;\n", string(metaJSON)))
		sb.WriteString(fmt.Sprintf("\tconst __sandbox = window.__createGMSandbox(__meta, %q);\n", s.ID))
		sb.WriteString("\tconst { GM_info, GM_addStyle, GM_setValue, GM_getValue, GM_deleteValue, GM_listValues, GM_log, GM_setClipboard, GM_registerMenuCommand, unsafeWindow } = __sandbox;\n\n")
		sb.WriteString("\ttry {\n")
		// 写入用户脚本代码
		sb.WriteString(s.Code)
		sb.WriteString("\n\t} catch (err) {\n")
		sb.WriteString(fmt.Sprintf("\t\tconsole.error('[Userscript Exec Error in %s]:', err);\n", s.Meta.Name))
		sb.WriteString("\t}\n")
		sb.WriteString("})();\n\n")
	}

	return sb.String()
}
