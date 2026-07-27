package main

import (
	"encoding/json"
	"fmt"
)

// GetMemory returns all memory entries as a JSON array.
func (a *App) GetMemory() (string, error) {
	if a.mem == nil {
		return "[]", nil
	}
	keys := a.mem.Keys()
	type memEntry struct {
		Key      string `json:"key"`
		Value    string `json:"value"`
		Category string `json:"category,omitempty"`
	}
	entries := make([]memEntry, 0, len(keys))
	for _, k := range keys {
		v, ok := a.mem.Get(k)
		if ok {
			entries = append(entries, memEntry{Key: k, Value: v})
		}
	}
	data, _ := json.Marshal(entries)
	return string(data), nil
}

// SetMemoryEntry sets a memory entry and persists the store.
func (a *App) SetMemoryEntry(key, value, category string) error {
	if a.mem == nil {
		return fmt.Errorf("memory not initialized")
	}
	a.mem.SetWithCategory(key, value, category)
	return a.mem.Save()
}

// DeleteMemoryEntry removes a memory entry and persists the store.
func (a *App) DeleteMemoryEntry(key string) error {
	if a.mem == nil {
		return fmt.Errorf("memory not initialized")
	}
	a.mem.Delete(key)
	return a.mem.Save()
}
