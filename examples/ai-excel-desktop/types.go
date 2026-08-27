package main

import (
	"context"
	"sync"

	"pi-ai-go/core"
)

// App 是 Wails 主应用结构体，持有运行时上下文、用户设置与已加载的数据。
type App struct {
	ctx        context.Context
	cancelFn   context.CancelFunc
	settings   AppSettings
	settingsMu sync.RWMutex
	dataDir    string

	// 已加载的 Excel/CSV 数据
	fileInfo  *FileInfo
	sheetData *SheetData
	dataMu    sync.RWMutex
}

// AppSettings 持久化用户配置。
type AppSettings struct {
	Providers       []ProviderSetting `json:"providers"`
	CurrentProvider int               `json:"currentProviderIndex"`
	Model           string            `json:"model"`
	MaxTokens       int               `json:"maxTokens"`
	Temperature     float64           `json:"temperature"`
}

// ProviderSetting 描述单个 LLM 提供商配置。
type ProviderSetting struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseUrl"`
}

// Current 返回当前选中的 provider，没有则返回 nil。
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

// AgentRequest 是前端调用 AgentMessage 时发送的 JSON 负载。
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
}

// HistoryMessage 镜像前端消息结构，用于多轮上下文。
type HistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ModelInfo 描述 GetModels 返回的模型条目。
type ModelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// 确保编译期类型存在
var _ core.KnownProvider
