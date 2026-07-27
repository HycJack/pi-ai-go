package main

import (
	"context"
	"sync"
	"sync/atomic"

	"chat-app/contextmgr"
	"chat-app/memory"
)

// App is the main Wails application struct. It holds the runtime context,
// user settings, memory, token statistics, and the API key pool.
type App struct {
	ctx                    context.Context
	cancelFn               context.CancelFunc
	settings               AppSettings
	settingsMu             sync.RWMutex
	dataDir                string
	mem                    *memory.Memory
	tokenStats             map[string]*contextmgr.TokenStats // per-conversation token stats
	currentConvID          string                            // active conversation ID for token stats
	ctxSettings            contextmgr.Settings
	settingsPath           string
	conversationSettings   map[string]ConversationSettings
	conversationSettingsMu sync.RWMutex
	keyPool                atomic.Value // stores *keypool.Pool
}

// AppSettings holds all user-configurable settings, persisted to
// settings.json.
type AppSettings struct {
	Providers       []ProviderSetting `json:"providers"`
	CurrentProvider int               `json:"currentProviderIndex"`
	Model           string            `json:"model"`
	MaxTokens       int               `json:"maxTokens"`
	Temperature     float64           `json:"temperature"`
	Reasoning       string            `json:"reasoning"`
	AgentMode       bool              `json:"agentMode"`
	WorkingDir      string            `json:"workingDir"`
	TTSSettings
	AgentSettings
}

// ProviderSetting describes a single LLM provider configuration.
type ProviderSetting struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	APIKey  string   `json:"apiKey"`
	ApiKeys []string `json:"apiKeys"`
	BaseURL string   `json:"baseUrl"`
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

// TTSSettings holds text-to-speech preferences.
type TTSSettings struct {
	TTSEnabled bool   `json:"ttsEnabled"`
	TTSVoice   string `json:"ttsVoice"`
}

// AgentSettings holds agent-related preferences.
type AgentSettings struct {
	AutoLearn   bool   `json:"autoLearn"`
	AutoCompact bool   `json:"autoCompact"`
	SkillsDir   string `json:"skillsDir"`
}

// ConversationSettings holds per-conversation toggles.
type ConversationSettings struct {
	AutoLearn bool `json:"autoLearn"`
}

// AgentRequest is the JSON payload sent by the frontend when invoking
// AgentMessage.
type AgentRequest struct {
	Message        string                   `json:"message"`
	Messages       []map[string]interface{} `json:"messages"`
	ConversationID string                   `json:"conversationId"`
	Provider       string                   `json:"provider"`
	APIKey         string                   `json:"apiKey"`
	BaseURL        string                   `json:"baseUrl"`
	Model          string                   `json:"model"`
	MaxTokens      int                      `json:"maxTokens"`
	Temperature    float64                  `json:"temperature"`
	Reasoning      string                   `json:"reasoning"`
	Images         []ImageInput             `json:"images,omitempty"`
}

// ImageInput represents an image attachment for multi-modal input.
type ImageInput struct {
	Data     string `json:"data"`
	MimeType string `json:"mimeType,omitempty"`
}

// GetModelsRequest is the JSON payload for GetModels.
type GetModelsRequest struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	APIKey   string `json:"apiKey"`
}

// ModelInfo describes a model returned by GetModels.
type ModelInfo struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Reasoning        bool              `json:"reasoning,omitempty"`
	ThinkingLevelMap map[string]string `json:"thinkingLevelMap,omitempty"`
}
