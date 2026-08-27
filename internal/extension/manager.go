package extension

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Manager 管理已加载扩展的注册表持久化、查询、启停与注入。
// 设计与 userscript.Manager 保持同构。
type Manager struct {
	mu      sync.RWMutex
	exts    map[string]*Extension
	order   []string // 保持展示顺序
	baseDir string   // 注册表文件所在目录
}

var (
	globalManager *Manager
	once          sync.Once
)

// registryFileName 扩展注册表文件名（位于 baseDir 下）。
const registryFileName = "extensions.json"

// GetGlobalManager 获取扩展全局单例管理器（存储于 %APPDATA%\gio-browser\extensions）。
func GetGlobalManager() *Manager {
	once.Do(func() {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = os.TempDir()
		}
		base := filepath.Join(appData, "gio-browser", "extensions")
		globalManager = NewManager(base)
		globalManager.Load()
	})
	return globalManager
}

// NewManager 基于指定基础目录创建管理器（测试可注入临时目录）。
func NewManager(baseDir string) *Manager {
	return &Manager{
		exts:    make(map[string]*Extension),
		baseDir: baseDir,
	}
}

// registryPath 返回注册表文件完整路径。
func (m *Manager) registryPath() string {
	return filepath.Join(m.baseDir, registryFileName)
}

// Load 从磁盘读取注册表；每个条目重新校验目录与清单（目录被删则标记禁用错误）。
func (m *Manager) Load() {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.registryPath())
	if err != nil {
		return // 首次使用，空注册表
	}
	var saved []*Extension
	if err := json.Unmarshal(data, &saved); err != nil {
		log.Printf("[Extension] 解析扩展注册表失败: %v", err)
		return
	}

	m.exts = make(map[string]*Extension, len(saved))
	m.order = nil
	for _, e := range saved {
		// 重新加载清单，跟踪源目录的更新
		if manifest, err := loadManifestFrom(e.Dir); err == nil {
			e.Manifest = *manifest
		} else {
			e.Enabled = false
			e.Manifest.Name = e.Manifest.Name + "（源目录缺失）"
			log.Printf("[Extension] 扩展源目录不可用: %s (%v)", e.Dir, err)
		}
		m.exts[e.ID] = e
		m.order = append(m.order, e.ID)
	}
}

// saveLocked 持久化注册表（必须在持有 mu 写锁下调用）。
func (m *Manager) saveLocked() error {
	if err := os.MkdirAll(m.baseDir, 0o755); err != nil {
		return err
	}
	list := make([]*Extension, 0, len(m.order))
	for _, id := range m.order {
		if e, ok := m.exts[id]; ok {
			list = append(list, e)
		}
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.registryPath(), data, 0o644)
}

// List 返回全部扩展快照（按安装顺序）。
func (m *Manager) List() []*Extension {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Extension, 0, len(m.order))
	for _, id := range m.order {
		if e, ok := m.exts[id]; ok {
			cp := *e
			out = append(out, &cp)
		}
	}
	return out
}

// Get 按 ID 查找扩展副本。
func (m *Manager) Get(id string) (*Extension, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.exts[id]
	if !ok {
		return nil, false
	}
	cp := *e
	return &cp, true
}

// LoadUnpacked 加载（或更新）一个已解压扩展目录。
func (m *Manager) LoadUnpacked(dir string) (*Extension, error) {
	ext, err := LoadFromDir(dir)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.exts[ext.ID]; !exists {
		m.order = append(m.order, ext.ID)
	}
	m.exts[ext.ID] = ext
	if err := m.saveLocked(); err != nil {
		return nil, err
	}
	return ext, nil
}

// Toggle 切换扩展启停状态。
func (m *Manager) Toggle(id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.exts[id]
	if !ok {
		return false, fmt.Errorf("扩展不存在: %s", id)
	}
	e.Enabled = !e.Enabled
	err := m.saveLocked()
	return e.Enabled, err
}

// Delete 从注册表移除扩展（不删除源目录）。
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.exts[id]; !ok {
		return fmt.Errorf("扩展不存在: %s", id)
	}
	delete(m.exts, id)
	for i, oid := range m.order {
		if oid == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	return m.saveLocked()
}

// GetMatchingExtensions 返回命中目标 URL 的全部已启用扩展。
func (m *Manager) GetMatchingExtensions(targetURL string) []*Extension {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*Extension
	for _, id := range m.order {
		if e := m.exts[id]; e != nil && e.MatchesURL(targetURL) {
			cp := *e
			out = append(out, &cp)
		}
	}
	return out
}
