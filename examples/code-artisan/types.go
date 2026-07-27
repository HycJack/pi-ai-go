package main

import (
	"context"
	"sync"

	"code-artisan/internal/env"
)

// App is the main Wails application struct.
type App struct {
	ctx              context.Context
	cancelFn         context.CancelFunc
	settings         AppSettings
	settingsMu       sync.RWMutex
	convMu           sync.Mutex
	dataDir          string
	settingsPath     string
	envManager       *env.Manager
	pythonRunner     *Runner
	artifactProgress chan env.Progress
}

// AppSettings holds all user-configurable settings, persisted to settings.json.
type AppSettings struct {
	Providers       []ProviderSetting `json:"providers"`
	CurrentProvider int               `json:"currentProviderIndex"`
	Model           string            `json:"model"`
	MaxTokens       int               `json:"maxTokens"`
	Temperature     float64           `json:"temperature"`
}

// ProviderSetting describes a single LLM provider configuration.
type ProviderSetting struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseUrl"`
}

// Current returns the currently selected provider, or nil if none.
func (s *AppSettings) Current() *ProviderSetting {
	if len(s.Providers) == 0 {
		return nil
	}
	idx := s.CurrentProvider
	if idx < 0 || idx >= len(s.Providers) {
		idx = 0
	}
	return &s.Providers[idx]
}

// ─── Conversation / Message Types ───

// Conversation stores a complete conversation (prompt + code + messages).
type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Prompt    string    `json:"prompt"`
	Code      string    `json:"code"`
	Timestamp string    `json:"timestamp"`
	Messages  []Message `json:"messages,omitempty"`
}

// Message represents a single turn in a conversation.
type Message struct {
	Role    string `json:"role"`    // "user" | "assistant"
	Content string `json:"content"` // text content
}

// ConversationSummary is a lightweight view for the sidebar list.
type ConversationSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Timestamp string `json:"timestamp"`
}

// ─── Code Generation Types ───

// CodeGenRequest is the JSON payload for code generation.
type CodeGenRequest struct {
	Prompt      string    `json:"prompt"`
	Provider    string    `json:"provider"`
	APIKey      string    `json:"apiKey"`
	BaseURL     string    `json:"baseUrl"`
	Model       string    `json:"model"`
	MaxTokens   int       `json:"maxTokens"`
	Temperature float64   `json:"temperature"`
	CurrentCode string    `json:"currentCode,omitempty"`
	ConvID      string    `json:"convId,omitempty"`
	Messages    []Message `json:"messages,omitempty"`
}

// CodeGenResponse is sent back to frontend after generation.
type CodeGenResponse struct {
	Code  string `json:"code"`
	Error string `json:"error,omitempty"`
}

// RunScriptRequest is the payload to run a Python script.
type RunScriptRequest struct {
	Code string `json:"code"`
}

// RunScriptResponse is sent back after script execution.
type RunScriptResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
