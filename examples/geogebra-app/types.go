package main

import (
	"context"
	"sync"

	"pi-ai-go/agent/session"
)

// App is the main Wails application struct.
type App struct {
	ctx          context.Context
	cancelFn     context.CancelFunc
	settings     AppSettings
	settingsMu   sync.RWMutex
	dataDir      string
	skillsOnce   sync.Once
	cachedSkills []session.Skill
}

// AppSettings persists user configuration.
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

// GeogebraRequest is the frontend request payload.
type GeogebraRequest struct {
	Message         string           `json:"message"`
	HistoryMessages []HistoryMessage `json:"historyMessages"`
	Provider        string           `json:"provider"`
	APIKey          string           `json:"apiKey"`
	BaseURL         string           `json:"baseUrl"`
	Model           string           `json:"model"`
	MaxTokens       int              `json:"maxTokens"`
	Temperature     float64          `json:"temperature"`
}

// HistoryMessage mirrors the frontend message for multi-turn context.
type HistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// GeogebraRegenRequest is the payload for validation + regeneration.
type GeogebraRegenRequest struct {
	GeogebraRequest
	OriginalMessage  string `json:"originalMessage"`
	GgbCode          string `json:"ggbCode"`
	ValidationErrors string `json:"validationErrors"`
}

// ModelInfo describes a model returned by GetModels.
type ModelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
