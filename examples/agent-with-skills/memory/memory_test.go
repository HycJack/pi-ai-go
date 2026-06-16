package memory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMemoryGetSet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")

	m, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	// Set & Get
	m.Set("user.name", "小明")
	v, ok := m.Get("user.name")
	if !ok || v != "小明" {
		t.Errorf("Get returned %q, %v; want \"小明\", true", v, ok)
	}

	// Overwrite
	m.Set("user.name", "小红")
	v, _ = m.Get("user.name")
	if v != "小红" {
		t.Errorf("Get after overwrite = %q, want \"小红\"", v)
	}
}

func TestMemoryPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")

	// Write
	m1, _ := New(path)
	m1.Set("user.name", "小明")
	m1.SetWithCategory("user.lang", "zh-CN", "user")
	if err := m1.Save(); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}

	// Reload
	m2, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	if v, _ := m2.Get("user.name"); v != "小明" {
		t.Errorf("After reload, name = %q, want \"小明\"", v)
	}
	if v, _ := m2.Get("user.lang"); v != "zh-CN" {
		t.Errorf("After reload, lang = %q, want \"zh-CN\"", v)
	}
}

func TestMemoryDelete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	m, _ := New(path)

	m.Set("a", "1")
	m.Set("b", "2")
	if m.Size() != 2 {
		t.Errorf("Size = %d, want 2", m.Size())
	}

	m.Delete("a")
	if m.Size() != 1 {
		t.Errorf("After delete Size = %d, want 1", m.Size())
	}
	if _, ok := m.Get("a"); ok {
		t.Errorf("After delete, key 'a' should not exist")
	}
}

func TestMemoryKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	m, _ := New(path)

	m.Set("b", "1")
	m.Set("a", "2")
	m.Set("c", "3")

	keys := m.Keys()
	if len(keys) != 3 {
		t.Errorf("len(keys) = %d, want 3", len(keys))
	}
	// Should be sorted
	if keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Errorf("Keys not sorted: %v", keys)
	}
}

func TestMemoryFormatForPrompt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	m, _ := New(path)

	m.SetWithCategory("user.name", "小明", "user")
	m.SetWithCategory("user.lang", "zh-CN", "user")
	m.SetWithCategory("task.current", "完善 demo", "task")

	prompt := m.FormatForPrompt()
	if prompt == "" {
		t.Fatal("FormatForPrompt returned empty")
	}
	if !contains(prompt, "user.name: 小明") {
		t.Errorf("FormatForPrompt missing user.name: 小明\n%s", prompt)
	}
	if !contains(prompt, "task.current: 完善 demo") {
		t.Errorf("FormatForPrompt missing task.current")
	}
	if !contains(prompt, "# Long-term Memory") {
		t.Errorf("FormatForPrompt missing header")
	}
}

func TestMemoryEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	m, _ := New(path)

	if m.Size() != 0 {
		t.Errorf("Empty size = %d, want 0", m.Size())
	}
	if prompt := m.FormatForPrompt(); prompt != "" {
		t.Errorf("Empty FormatForPrompt = %q, want empty", prompt)
	}
}

func TestMemoryHas(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	m, _ := New(path)

	if m.Has("nonexistent") {
		t.Error("Has(nonexistent) = true, want false")
	}
	m.Set("exists", "yes")
	if !m.Has("exists") {
		t.Error("Has(exists) = false, want true")
	}
}

func TestMemoryListByCategory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	m, _ := New(path)

	m.SetWithCategory("user.name", "小明", "user")
	m.SetWithCategory("user.lang", "zh-CN", "user")
	m.SetWithCategory("task.current", "task1", "task")

	users := m.ListByCategory("user")
	if len(users) != 2 {
		t.Errorf("ListByCategory(user) returned %d items, want 2", len(users))
	}
	tasks := m.ListByCategory("task")
	if len(tasks) != 1 {
		t.Errorf("ListByCategory(task) returned %d items, want 1", len(tasks))
	}
	none := m.ListByCategory("nonexistent")
	if len(none) != 0 {
		t.Errorf("ListByCategory(nonexistent) returned %d items, want 0", len(none))
	}
}

func TestMemoryHashChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	m, _ := New(path)

	// 初始为空
	h1 := m.Hash()
	if h1 != "" {
		t.Errorf("empty hash = %q, want \"\"", h1)
	}

	// 添加一条
	m.Set("k1", "v1")
	h2 := m.Hash()
	if h2 == h1 {
		t.Error("hash should change after Set")
	}

	// 等待至少 1 纳秒（UpdatedAt 变化）
	time.Sleep(time.Millisecond)

	// 修改同一条
	m.Set("k1", "v1-updated")
	h3 := m.Hash()
	if h3 == h2 {
		t.Error("hash should change after Set (UpdatedAt updated)")
	}

	// 删除
	m.Delete("k1")
	h4 := m.Hash()
	if h4 != "" {
		t.Errorf("hash after delete = %q, want \"\"", h4)
	}
}

func TestMemoryLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")

	// Initial state
	m1, _ := New(path)
	m1.Set("k1", "v1")
	m1.Save()

	// New instance
	m2, _ := New(path)
	if v, _ := m2.Get("k1"); v != "v1" {
		t.Errorf("Get(k1) = %q, want v1", v)
	}

	// Externally modify
	m1.Set("k2", "v2")
	m1.Save()

	// Reload
	if err := m2.Load(); err != nil {
		t.Fatal(err)
	}
	if v, _ := m2.Get("k2"); v != "v2" {
		t.Errorf("After Load, Get(k2) = %q, want v2", v)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
