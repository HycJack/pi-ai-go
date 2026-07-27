package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// FrontendMessage mirrors the frontend Message interface.
type FrontendMessage struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Conversation mirrors the frontend Conversation interface.
type Conversation struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Messages []FrontendMessage `json:"messages"`
	Result   *GeogebraResult   `json:"result,omitempty"`
	Prompt   string            `json:"prompt,omitempty"`
	Timestamp string           `json:"timestamp"`
}

// conversationsDir returns the path to the per-conversation files directory.
func (a *App) conversationsDir() string {
	return filepath.Join(a.dataDir, "conversations")
}

// GetConversations reads all conversation files from conversations/ directory.
func (a *App) GetConversations() (string, error) {
	dir := a.conversationsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
			LogError("[conversations] mkdir error: %v", mkErr)
		}
		return "[]", nil
	}

	var convs []json.RawMessage
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			LogError("[conversations] failed to read %s: %v", entry.Name(), err)
			continue
		}
		convs = append(convs, json.RawMessage(data))
	}
	if convs == nil {
		return "[]", nil
	}
	result, err := json.Marshal(convs)
	if err != nil {
		LogError("[conversations] marshal error: %v", err)
		return "[]", nil
	}
	return string(result), nil
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
