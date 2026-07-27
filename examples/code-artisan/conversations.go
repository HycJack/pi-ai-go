package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// conversationsDir returns the path to the per-conversation files directory.
func (a *App) conversationsDir() string {
	return filepath.Join(a.dataDir, "conversations")
}

// GetConversations reads all conversation files from conversations/ directory.
// Returns a JSON array of ConversationSummary (lightweight, no messages).
func (a *App) GetConversations() (string, error) {
	dir := a.conversationsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
			LogError("[conversations] mkdir error: %v", mkErr)
		}
		return "[]", nil
	}

	summaries := make([]ConversationSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			LogError("[conversations] failed to read %s: %v", entry.Name(), err)
			continue
		}
		var conv Conversation
		if err := json.Unmarshal(data, &conv); err != nil {
			LogError("[conversations] failed to unmarshal %s: %v", entry.Name(), err)
			continue
		}
		summaries = append(summaries, ConversationSummary{
			ID:        conv.ID,
			Title:     conv.Title,
			Timestamp: conv.Timestamp,
		})
	}

	result, err := json.Marshal(summaries)
	if err != nil {
		LogError("[conversations] marshal error: %v", err)
		return "[]", nil
	}
	return string(result), nil
}

// GetConversation returns a single conversation by ID.
func (a *App) GetConversation(id string) (string, error) {
	dir := a.conversationsDir()
	path := filepath.Join(dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveConversation saves a single conversation to its own file.
func (a *App) SaveConversation(id string, jsonStr string) error {
	dir := a.conversationsDir()
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, id+".json")
	return os.WriteFile(path, []byte(jsonStr), 0644)
}

// DeleteConversation deletes a single conversation file.
func (a *App) DeleteConversation(id string) error {
	dir := a.conversationsDir()
	path := filepath.Join(dir, id+".json")
	return os.Remove(path)
}
