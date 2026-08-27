package main

import (
	"context"
	"sync"

	"pi-ai-go/core"
)

// App 是 Wails 主应用结构体，持有运行时上下文、用户设置与渲染状态。
type App struct {
	ctx      context.Context
	cancelFn context.CancelFunc

	settings   AppSettings
	settingsMu sync.RWMutex
	dataDir    string

	// 字体资产目录（内置 TTF）
	fontAssetsDir string
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

// HandwritingParams 是前端调用 Generate 时发送的渲染参数。
type HandwritingParams struct {
	// 文本
	Text string `json:"text"`

	// 字体
	FontSize   int    `json:"fontSize"`
	LineSpacing int   `json:"lineSpacing"`
	WordSpacing int    `json:"wordSpacing"`
	Fill        string `json:"fill"` // "r,g,b" 或 "r,g,b,a"

	// 画布
	Width  int `json:"width"`
	Height int `json:"height"`

	// 边距
	MarginTop    int `json:"marginTop"`
	MarginBottom int `json:"marginBottom"`
	MarginLeft   int `json:"marginLeft"`
	MarginRight  int `json:"marginRight"`

	// 随机扰动（手写感）
	LineSpacingSigma   float64 `json:"lineSpacingSigma"`
	FontSizeSigma      float64 `json:"fontSizeSigma"`
	WordSpacingSigma   float64 `json:"wordSpacingSigma"`
	PerturbXSigma      float64 `json:"perturbXSigma"`
	PerturbYSigma      float64 `json:"perturbYSigma"`
	PerturbThetaSigma  float64 `json:"perturbThetaSigma"`
	InkDepthSigma      float64 `json:"inkDepthSigma"`

	// 删除线（涂改痕迹）
	StrikethroughProbability float64 `json:"strikethroughProbability"`
	StrikethroughLengthSigma  float64 `json:"strikethroughLengthSigma"`
	StrikethroughWidthSigma   float64 `json:"strikethroughWidthSigma"`
	StrikethroughAngleSigma   float64 `json:"strikethroughAngleSigma"`
	StrikethroughWidth        float64 `json:"strikethroughWidth"`

	// 选项
	IsUnderlined        bool `json:"isUnderlined"`
	EnableEnglishSpacing bool `json:"enableEnglishSpacing"`
	Preview             bool `json:"preview"`
	FullPreview         bool `json:"fullPreview"`
	ExportPDF           bool `json:"exportPDF"`

	// 字体来源：fontOption（内置字体名）或 fontFileBase64（用户上传）
	FontOption     string `json:"fontOption"`
	FontFileBase64 string `json:"fontFileBase64"`

	// 背景图：backgroundImageBase64（用户上传）或 width/height 自动生成
	BackgroundImageBase64 string `json:"backgroundImageBase64"`
}

// FontInfo 描述一个可用字体。
type FontInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// GenerateResult 是生成完成后的返回。
type GenerateResult struct {
	// 预览模式：返回 base64 图片数组
	Images []string `json:"images,omitempty"`
	// 完整模式：返回 zip 文件路径（前端触发下载）
	ZipPath string `json:"zipPath,omitempty"`
	// PDF 模式：返回 pdf 文件路径
	PDFPath string `json:"pdfPath,omitempty"`
	// 错误信息
	Error string `json:"error,omitempty"`
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

// ModelInfo 描述 GetModels 返回的模型条目。
type ModelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// 确保编译期类型存在
var _ core.KnownProvider
