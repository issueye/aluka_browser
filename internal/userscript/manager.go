package userscript

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Manager 管理用户脚本集合的本地存储、查询、启停与注入。
type Manager struct {
	mu      sync.RWMutex
	scripts map[string]*Script
	order   []string // 保持展示顺序
	path    string
}

var (
	globalManager *Manager
	once          sync.Once
)

// DefaultScriptTemplate 新建脚本的标准模板代码。
const DefaultScriptTemplate = `// ==UserScript==
// @name         我的自定义脚本
// @namespace    https://gio-browser.local/
// @version      1.0.0
// @description  在匹配的网页中执行自定义逻辑
// @author       You
// @match        *://*/*
// @run-at       document-end
// @grant        GM_addStyle
// @grant        GM_log
// @grant        GM_setValue
// @grant        GM_getValue
// ==/UserScript==

(function() {
    'use strict';
    GM_log('用户脚本已在页面加载:', window.location.href);

    // 示例：动态添加自定义样式
    // GM_addStyle('body { font-family: "Segoe UI", sans-serif !important; }');
})();
`

// 预设推荐实用脚本定义
var presetScripts = []struct {
	id      string
	enabled bool
	code    string
}{
	{
		id:      "preset-dark-mode",
		enabled: false,
		code: `// ==UserScript==
// @name         暗黑夜间护眼模式
// @namespace    https://gio-browser.local/presets
// @version      1.0.0
// @description  为所有网页提供平滑舒适的深色护眼反色滤镜
// @author       Gio Browser
// @match        *://*/*
// @run-at       document-start
// @grant        GM_addStyle
// @grant        GM_log
// ==/UserScript==

(function() {
    'use strict';
    GM_addStyle('html { filter: invert(88%) hue-rotate(180deg) !important; background: #1a1a1a !important; } img, video, canvas, svg, [style*="background-image"] { filter: invert(100%) hue-rotate(180deg) !important; }');
    GM_log('暗黑夜间模式已注入');
})();
`,
	},
	{
		id:      "preset-unlock-copy",
		enabled: true,
		code: `// ==UserScript==
// @name         解除网页复制与右键限制
// @namespace    https://gio-browser.local/presets
// @version      1.0.0
// @description  自动解除网页中禁止选中文本、禁止右键菜单与禁止复制的限制
// @author       Gio Browser
// @match        *://*/*
// @run-at       document-start
// @grant        GM_addStyle
// @grant        GM_log
// ==/UserScript==

(function() {
    'use strict';
    GM_addStyle('* { -webkit-user-select: auto !important; user-select: auto !important; }');
    const events = ['copy', 'cut', 'contextmenu', 'selectstart', 'mousedown'];
    events.forEach(function(ev) {
        document.addEventListener(ev, function(e) {
            e.stopPropagation();
        }, true);
    });
    GM_log('网页复制与右键选中限制已解除');
})();
`,
	},
	{
		id:      "preset-quick-tools",
		enabled: true,
		code: `// ==UserScript==
// @name         页面快捷置顶悬浮球
// @namespace    https://gio-browser.local/presets
// @version      1.0.0
// @description  在网页右下角添加一个精致的一键平滑回到顶部悬浮按钮
// @author       Gio Browser
// @match        *://*/*
// @run-at       document-end
// @grant        GM_addStyle
// ==/UserScript==

(function() {
    'use strict';
    GM_addStyle('.gio-top-btn { position: fixed; right: 20px; bottom: 20px; width: 38px; height: 38px; background: rgba(30, 41, 59, 0.85); color: #fff; border-radius: 50%; display: flex; align-items: center; justify-content: center; font-size: 18px; cursor: pointer; z-index: 999999; box-shadow: 0 4px 12px rgba(0,0,0,0.3); border: 1px solid rgba(255,255,255,0.15); transition: all 0.2s ease; user-select: none; } .gio-top-btn:hover { background: #3b82f6; transform: scale(1.1); }');

    const btn = document.createElement('div');
    btn.className = 'gio-top-btn';
    btn.innerHTML = '▲';
    btn.title = '回到顶部';
    btn.onclick = function() {
        window.scrollTo({ top: 0, behavior: 'smooth' });
    };
    document.body.appendChild(btn);
})();
`,
	},
}

// GetGlobalManager 获取用户脚本全局单例管理器。
func GetGlobalManager() *Manager {
	once.Do(func() {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = os.TempDir()
		}
		path := filepath.Join(appData, "gio-browser", "userscripts.json")
		globalManager = NewManager(path)
		globalManager.Load()
	})
	return globalManager
}

// NewManager 基于指定路径创建一个管理器实例。
func NewManager(storagePath string) *Manager {
	return &Manager{
		scripts: make(map[string]*Script),
		path:    storagePath,
	}
}

// Load 从磁盘读取所有用户脚本，若不存在则注入内置预设。
func (m *Manager) Load() {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		// 初始化预设脚本
		m.loadPresetsLocked()
		_ = m.saveLocked()
		return
	}

	var saved []*Script
	if err := json.Unmarshal(data, &saved); err != nil {
		log.Printf("[Userscript] 读取配置文件失败: %v", err)
		m.loadPresetsLocked()
		return
	}

	m.scripts = make(map[string]*Script, len(saved))
	m.order = make([]string, 0, len(saved))
	for _, s := range saved {
		// 重新解析元数据
		s.Meta = ParseMetadata(s.Code)
		m.scripts[s.ID] = s
		m.order = append(m.order, s.ID)
	}

	if len(m.scripts) == 0 {
		m.loadPresetsLocked()
		_ = m.saveLocked()
	}
}

func (m *Manager) loadPresetsLocked() {
	m.scripts = make(map[string]*Script)
	m.order = nil
	for _, p := range presetScripts {
		s := NewScript(p.code, p.enabled)
		s.ID = p.id
		m.scripts[s.ID] = s
		m.order = append(m.order, s.ID)
	}
}

// Save 将当前所有脚本保存到磁盘。
func (m *Manager) saveLocked() error {
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	list := make([]*Script, 0, len(m.order))
	for _, id := range m.order {
		if s, ok := m.scripts[id]; ok {
			list = append(list, s)
		}
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.path, data, 0644)
}

// List 返回所有用户脚本列表副本。
func (m *Manager) List() []*Script {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*Script, 0, len(m.order))
	for _, id := range m.order {
		if s, ok := m.scripts[id]; ok {
			// 浅拷贝
			cp := *s
			out = append(out, &cp)
		}
	}
	return out
}

// Get 根据 ID 查找脚本。
func (m *Manager) Get(id string) (*Script, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.scripts[id]
	if !ok {
		return nil, false
	}
	cp := *s
	return &cp, true
}

// AddOrUpdate 添加或更新一个用户脚本。
func (m *Manager) AddOrUpdate(script *Script) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if script.ID == "" {
		script = NewScript(script.Code, script.Enabled)
	} else {
		script.UpdateCode(script.Code)
	}

	if _, exists := m.scripts[script.ID]; !exists {
		m.order = append(m.order, script.ID)
	}
	m.scripts[script.ID] = script

	return m.saveLocked()
}

// Toggle 切换脚本的启用/禁用状态。
func (m *Manager) Toggle(id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.scripts[id]
	if !ok {
		return false, fmt.Errorf("脚本不存在: %s", id)
	}

	s.Enabled = !s.Enabled
	err := m.saveLocked()
	return s.Enabled, err
}

// Delete 删除指定 ID 的脚本。
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.scripts, id)
	for i, oid := range m.order {
		if oid == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	return m.saveLocked()
}

// GetMatchingScripts 返回与 targetURL 匹配的所有已启用脚本。
func (m *Manager) GetMatchingScripts(targetURL string) []*Script {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matching []*Script
	for _, id := range m.order {
		s := m.scripts[id]
		if s != nil && s.Matches(targetURL) {
			matching = append(matching, s)
		}
	}
	return matching
}

// BuildInjectionForURL 为指定 URL 生成可注入的完整代码。
func (m *Manager) BuildInjectionForURL(targetURL string) string {
	matching := m.GetMatchingScripts(targetURL)
	if len(matching) == 0 {
		return ""
	}
	return BuildInjectionBundle(matching)
}
