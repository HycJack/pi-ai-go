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

type Memory struct {
	mu   sync.RWMutex
	path string
	data map[string]Entry
}

type Entry struct {
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Category  string    `json:"category,omitempty"`
}

func New(path string) (*Memory, error) {
	m := &Memory{
		path: path,
		data: make(map[string]Entry),
	}
	if dir := filepath.Dir(path); dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &m.data); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (m *Memory) Get(key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.data[key]
	if !ok {
		return "", false
	}
	return e.Value, true
}

func (m *Memory) Set(key, value string) {
	m.SetWithCategory(key, value, "")
}

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

func (m *Memory) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

func (m *Memory) Has(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.data[key]
	return ok
}

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

type Item struct {
	Key   string
	Entry Entry
}

func (m *Memory) ListByCategory(category string) []Item {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var items []Item
	for k, v := range m.data {
		if v.Category == category {
			items = append(items, Item{Key: k, Entry: v})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})
	return items
}

func (m *Memory) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

func (m *Memory) Hash() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
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

func (m *Memory) Save() error {
	m.mu.RLock()
	data, err := json.MarshalIndent(m.data, "", "  ")
	m.mu.RUnlock()
	if err != nil {
		return err
	}
	dir := filepath.Dir(m.path)
	tmpPath := filepath.Join(dir, ".memory-tmp")
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, m.path)
}

func (m *Memory) FormatForPrompt() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.data) == 0 {
		return ""
	}
	categories := make(map[string][]string)
	for k, v := range m.data {
		cat := v.Category
		if cat == "" {
			cat = "general"
		}
		categories[cat] = append(categories[cat], k+": "+v.Value)
	}
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
