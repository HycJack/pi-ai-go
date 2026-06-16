// Package memory 提供 Agent 长期记忆的简单 KV 存储。
//
// 设计目标：
// 1. 跨会话保留：用户偏好、任务进度、关键事实
// 2. JSON 持久化：可读、可手工编辑
// 3. 简单 API：Get/Set/Delete/List，无需重型数据库
//
// 与 session 包的关系：
// - session 管理"会话级"对话历史（每条消息、会话分支）
// - memory 管理"跨会话"长期事实（用户偏好、已完成任务）
package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Memory 是长期记忆 KV 存储。
//
// 示例：
//
//	mem := memory.New("/path/to/memory.json")
//	mem.Set("user.name", "小明")
//	mem.Set("user.preferred_language", "zh-CN")
//	name, ok := mem.Get("user.name")
//	mem.Save() // 持久化到磁盘
type Memory struct {
	mu   sync.RWMutex
	path string
	data map[string]Entry
}

// Entry 是单条记忆条目。
type Entry struct {
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// Category 用于分组（如 "user", "task", "preference"）。
	Category string `json:"category,omitempty"`
}

// New 加载或创建 memory 文件。
// 如果文件不存在，自动创建。
func New(path string) (*Memory, error) {
	m := &Memory{
		path: path,
		data: make(map[string]Entry),
	}

	// 确保目录存在
	if dir := filepath.Dir(path); dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}

	// 尝试加载现有数据
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &m.data); err != nil {
			return nil, err
		}
	}

	return m, nil
}

// Get 获取键对应的值。
// 返回值和是否存在标志。
func (m *Memory) Get(key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.data[key]
	if !ok {
		return "", false
	}
	return e.Value, true
}

// Set 设置键值对。
// 自动更新 UpdatedAt 时间戳。
func (m *Memory) Set(key, value string) {
	m.SetWithCategory(key, value, "")
}

// SetWithCategory 设置带分类的键值对。
func (m *Memory) SetWithCategory(key, value, category string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	existing, exists := m.data[key]
	if exists {
		existing.Value = value
		existing.UpdatedAt = now
		if category != "" {
			existing.Category = category
		}
		m.data[key] = existing
	} else {
		m.data[key] = Entry{
			Value:     value,
			CreatedAt: now,
			UpdatedAt: now,
			Category:  category,
		}
	}
}

// Delete 删除键。
func (m *Memory) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

// Has 检查键是否存在。
func (m *Memory) Has(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.data[key]
	return ok
}

// Keys 返回所有键（按字母顺序排序）。
func (m *Memory) Keys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]string, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ListByCategory 返回指定分类下的所有条目。
func (m *Memory) ListByCategory(category string) []Item {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var items []Item
	for k, v := range m.data {
		if v.Category == category {
			items = append(items, Item{Key: k, Entry: v})
		}
	}
	// 按 key 排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})
	return items
}

// Item 是 ListByCategory 返回的条目。
type Item struct {
	Key   string
	Entry Entry
}

// Size 返回条目数。
func (m *Memory) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

// Hash 返回记忆内容的快速 hash。
// 用于判断记忆是否变化（决定是否重建 system prompt）。
// 简单做法：基于 UpdatedAt 时间戳拼接，相同说明没变。
func (m *Memory) Hash() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 使用所有 entry 的 UpdatedAt 时间戳拼接
	// 任何一条变化都会改变 hash
	if len(m.data) == 0 {
		return ""
	}
	var sb strings.Builder
	for k, v := range m.data {
		sb.WriteString(k)
		sb.WriteString("|")
		sb.WriteString(v.UpdatedAt.Format(time.RFC3339Nano))
		sb.WriteString(";")
	}
	return sb.String()
}

// Save 持久化到磁盘。
func (m *Memory) Save() error {
	m.mu.RLock()
	data, err := json.MarshalIndent(m.data, "", "  ")
	m.mu.RUnlock()

	if err != nil {
		return err
	}

	// 原子写入：先写临时文件，再 rename
	dir := filepath.Dir(m.path)
	tmp, err := os.CreateTemp(dir, ".memory-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, m.path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// Load 从磁盘重新加载（覆盖内存中的数据）。
func (m *Memory) Load() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return json.Unmarshal(data, &m.data)
}

// FormatForPrompt 把长期记忆格式化为可注入 system prompt 的字符串。
//
// 格式：
//
//	# Long-term Memory
//
//	user.name: 小明
//	user.language: zh-CN
//	task.current: 完善 demo 项目
func (m *Memory) FormatForPrompt() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.data) == 0 {
		return ""
	}

	// 按分类分组
	categories := make(map[string][]string)
	for k, v := range m.data {
		cat := v.Category
		if cat == "" {
			cat = "general"
		}
		categories[cat] = append(categories[cat], k+": "+v.Value)
	}

	// 稳定排序：按分类名
	catNames := make([]string, 0, len(categories))
	for c := range categories {
		catNames = append(catNames, c)
	}
	sort.Strings(catNames)

	result := "# Long-term Memory\n\n"
	for _, cat := range catNames {
		lines := categories[cat]
		sort.Strings(lines)
		for _, line := range lines {
			result += "- " + line + "\n"
		}
	}
	return result
}
