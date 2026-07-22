package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"pi-ai-go/agent"
	"pi-ai-go/agent/session"
	agenttools "pi-ai-go/agent/tools"
	"pi-ai-go/core"
	"pi-ai-go/llm"
	"pi-ai-go/providers"

	"chat-app/autolearn"
	"chat-app/contextmgr"
	"chat-app/keypool"
	"chat-app/memory"

	"github.com/kbinani/screenshot"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx                    context.Context
	cancelFn               context.CancelFunc
	settings               AppSettings
	settingsMu             sync.RWMutex
	dataDir                string
	mem                    *memory.Memory
	tokenStats             *contextmgr.TokenStats
	ctxSettings            contextmgr.Settings
	settingsPath           string
	conversationSettings   map[string]ConversationSettings
	conversationSettingsMu sync.RWMutex
	keyPool                *keypool.Pool
}

type AppSettings struct {
	Providers       []ProviderSetting `json:"providers"`
	CurrentProvider int               `json:"currentProviderIndex"`
	Model           string            `json:"model"`
	MaxTokens       int               `json:"maxTokens"`
	Temperature     float64           `json:"temperature"`
	Reasoning       string            `json:"reasoning"`
	AgentMode       bool              `json:"agentMode"`
	TTSSettings
	AgentSettings
}

type ProviderSetting struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	APIKey  string   `json:"apiKey"`
	ApiKeys []string `json:"apiKeys"`
	BaseURL string   `json:"baseUrl"`
}

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

type TTSSettings struct {
	TTSEnabled bool   `json:"ttsEnabled"`
	TTSVoice   string `json:"ttsVoice"`
}

type AgentSettings struct {
	AutoLearn   bool   `json:"autoLearn"`
	AutoCompact bool   `json:"autoCompact"`
	SkillsDir   string `json:"skillsDir"`
}

type ConversationSettings struct {
	AutoLearn bool `json:"autoLearn"`
}

func NewApp() *App {
	dataDir := getDataDir()
	_ = os.MkdirAll(dataDir, 0755)
	return &App{
		dataDir:              dataDir,
		settingsPath:         filepath.Join(dataDir, "settings.json"),
		conversationSettings: make(map[string]ConversationSettings),
		settings: AppSettings{
			Providers: []ProviderSetting{
				{
					Name:    "OpenAI",
					Type:    "openai",
					APIKey:  "",
					ApiKeys: []string{},
					BaseURL: "https://api.openai.com/v1",
				},
			},
			CurrentProvider: 0,
			Model:           "gpt-4o-mini",
			MaxTokens:       4096,
			Temperature:     1.0,
			Reasoning:       "medium",
			AgentSettings: AgentSettings{
				AutoLearn:   false,
				AutoCompact: true,
				SkillsDir:   filepath.Join(dataDir, "skills"),
			},
		},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	providers.RegisterBuiltInProviders()
	a.loadSettings()
	a.initKeyPool()
	mem, err := memory.New(filepath.Join(a.dataDir, "memory.json"))
	if err != nil {
		fmt.Printf("[memory] init error: %v\n", err)
	} else {
		a.mem = mem
		fmt.Printf("[memory] loaded %d entries\n", mem.Size())
	}
	skillsDir := a.settings.SkillsDir
	if skillsDir == "" {
		skillsDir = filepath.Join(a.dataDir, "skills")
	}
	_ = os.MkdirAll(skillsDir, 0755)
	fmt.Printf("[app] initialized, data dir: %s\n", a.dataDir)
}

func (a *App) initKeyPool() {
	cp := a.settings.Current()
	if cp == nil {
		a.keyPool = keypool.New(nil, keypool.DefaultSettings())
		return
	}
	keys := cp.ApiKeys
	if len(keys) == 0 && cp.APIKey != "" {
		keys = []string{cp.APIKey}
	}
	a.keyPool = keypool.New(keys, keypool.DefaultSettings())
}

// selectAPIKey 选择一个可用的 API key。优先使用 keypool 轮询，pool 为空时使用单 key。
func (a *App) selectAPIKey() string {
	key, err := a.keyPool.Next()
	if err != nil {
		return ""
	}
	return key
}

// markKeySuccess 标记当前 key 调用成功。
func (a *App) markKeySuccess() {
	a.keyPool.MarkSuccess()
}

// markKeyFailed 标记当前 key 调用失败。
func (a *App) markKeyFailed(err error) {
	a.keyPool.MarkFailed(keypool.CategorizeError(err))
}

func (a *App) loadSettings() {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	data, err := os.ReadFile(a.settingsPath)
	if err != nil {
		return
	}
	var s AppSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return
	}
	a.settings = s
}

func (a *App) saveSettings() {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	data, err := json.MarshalIndent(a.settings, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(a.settingsPath, data, 0644)
}

// ─── 后端 API ───

func (a *App) GetSettings() (string, error) {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	data, err := json.Marshal(a.settings)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) SaveSettings(str string) error {
	var s AppSettings
	if err := json.Unmarshal([]byte(str), &s); err != nil {
		return err
	}
	a.settingsMu.Lock()
	oldModel := a.settings.Model
	newModel := s.Model
	a.settings = s
	a.settingsMu.Unlock()
	a.initKeyPool()
	if a.settings.SkillsDir != "" {
		_ = os.MkdirAll(a.settings.SkillsDir, 0755)
	}
	if oldModel != newModel || newModel == "" {
		modelID := newModel
		if modelID == "" {
			modelID = "gpt-4o-mini"
		}
		a.ctxSettings = contextmgr.DefaultSettings(modelID)
		a.tokenStats = contextmgr.NewTokenStats(a.ctxSettings)
	}
	a.saveSettings()
	return nil
}

func (a *App) GetConversations() (string, error) {
	path := filepath.Join(a.dataDir, "conversations.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "[]", nil
	}
	return string(data), nil
}

func (a *App) SaveConversations(str string) error {
	path := filepath.Join(a.dataDir, "conversations.json")
	return os.WriteFile(path, []byte(str), 0644)
}

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

func (a *App) SetMemoryEntry(key, value, category string) error {
	if a.mem == nil {
		return fmt.Errorf("memory not initialized")
	}
	a.mem.SetWithCategory(key, value, category)
	return a.mem.Save()
}

func (a *App) DeleteMemoryEntry(key string) error {
	if a.mem == nil {
		return fmt.Errorf("memory not initialized")
	}
	a.mem.Delete(key)
	return a.mem.Save()
}

func (a *App) GetContextStats() string {
	if a.tokenStats == nil {
		modelID := a.settings.Model
		if modelID == "" {
			modelID = "gpt-4o-mini"
		}
		a.ctxSettings = contextmgr.DefaultSettings(modelID)
		a.tokenStats = contextmgr.NewTokenStats(a.ctxSettings)
	}
	stats := a.tokenStats.Get()
	return contextmgr.FormatStats(stats)
}

func (a *App) CancelStream() {
	if a.cancelFn != nil {
		a.cancelFn()
		a.cancelFn = nil
	}
}

func (a *App) SetAutoLearnEnabled(enabled bool) error {
	a.settingsMu.Lock()
	a.settings.AutoLearn = enabled
	a.settingsMu.Unlock()
	a.saveSettings()
	return nil
}

func (a *App) GetCompactionStatus() string {
	if a.tokenStats == nil {
		return "Not initialized"
	}
	s := a.tokenStats.Get()
	return contextmgr.FormatStats(s)
}

// CaptureScreen captures the given display (default 0 = primary screen) and
// returns the PNG image as a base64 data URL (e.g. "data:image/png;base64,...").
// The frontend can use this directly in an <img src> or send it to a
// multi-modal model as an ImageContent block.
func (a *App) CaptureScreen(displayIndex int) (string, error) {
	if displayIndex < 0 {
		displayIndex = 0
	}
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return "", fmt.Errorf("no active display available")
	}
	if displayIndex >= n {
		displayIndex = 0
	}
	bounds := screenshot.GetDisplayBounds(displayIndex)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return "", fmt.Errorf("capture failed: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("png encode failed: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	return "data:image/png;base64," + encoded, nil
}

// NumDisplays returns the number of active displays.
func (a *App) NumDisplays() int {
	return screenshot.NumActiveDisplays()
}

// CaptureRegion captures a rectangular region of the given display and returns
// the PNG image as a base64 data URL. Coordinates (x, y) are relative to the
// display's origin; width/height define the region size. If width or height
// is <= 0, the entire display is captured.
func (a *App) CaptureRegion(displayIndex, x, y, width, height int) (string, error) {
	if displayIndex < 0 {
		displayIndex = 0
	}
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return "", fmt.Errorf("no active display available")
	}
	if displayIndex >= n {
		displayIndex = 0
	}
	bounds := screenshot.GetDisplayBounds(displayIndex)
	// If no region specified, capture the whole display
	if width <= 0 || height <= 0 {
		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			return "", fmt.Errorf("capture failed: %w", err)
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return "", fmt.Errorf("png encode failed: %w", err)
		}
		encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
		return "data:image/png;base64," + encoded, nil
	}
	// Clamp region to display bounds
	maxW := bounds.Dx() - x
	maxH := bounds.Dy() - y
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if width > maxW {
		width = maxW
	}
	if height > maxH {
		height = maxH
	}
	if width <= 0 || height <= 0 {
		return "", fmt.Errorf("invalid region after clamping")
	}
	rect := image.Rect(bounds.Min.X+x, bounds.Min.Y+y, bounds.Min.X+x+width, bounds.Min.Y+y+height)
	img, err := screenshot.CaptureRect(rect)
	if err != nil {
		return "", fmt.Errorf("capture region failed: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("png encode failed: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	return "data:image/png;base64," + encoded, nil
}

func (a *App) GetModels(params map[string]interface{}) ([]ModelInfo, error) {
	providerStr, _ := params["provider"].(string)
	baseURL, _ := params["baseUrl"].(string)
	apiKey, _ := params["apiKey"].(string)
	if providerStr == "" {
		// 回退到当前 provider
		cp := a.settings.Current()
		if cp != nil {
			providerStr = cp.Type
			if baseURL == "" {
				baseURL = cp.BaseURL
			}
			if apiKey == "" {
				apiKey = cp.APIKey
			}
		}
	}
	if providerStr == "" {
		return nil, fmt.Errorf("no provider configured")
	}
	if providerStr == "anthropic" {
		return a.getAnthropicModels(baseURL, apiKey)
	}
	return a.getOpenAIModels(baseURL, apiKey)
}

type GetModelsRequest struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	APIKey   string `json:"apiKey"`
}

type ModelInfo struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Reasoning        bool              `json:"reasoning,omitempty"`
	ThinkingLevelMap map[string]string `json:"thinkingLevelMap,omitempty"`
}

func (a *App) getOpenAIModels(baseURL, apiKey string) ([]ModelInfo, error) {
	url := baseURL
	if url == "" {
		url = "https://api.openai.com/v1"
	}
	url += "/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return a.getCachedModels(core.ProviderOpenAI), nil
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	} else {
		envKey := core.ResolveAPIKey(core.ProviderOpenAI, "")
		if envKey != "" {
			req.Header.Set("Authorization", "Bearer "+envKey)
		}
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return a.getCachedModels(core.ProviderOpenAI), nil
	}
	defer resp.Body.Close()
	var result struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return a.getCachedModels(core.ProviderOpenAI), nil
	}
	var models []ModelInfo
	for _, m := range result.Data {
		models = append(models, ModelInfo{ID: m.ID, Name: m.Name})
	}
	return models, nil
}

func (a *App) getAnthropicModels(baseURL, apiKey string) ([]ModelInfo, error) {
	url := baseURL
	if url == "" {
		url = "https://api.anthropic.com/v1"
	}
	url += "/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return a.getCachedModels(core.ProviderAnthropic), nil
	}
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	} else {
		envKey := core.ResolveAPIKey(core.ProviderAnthropic, "")
		if envKey != "" {
			req.Header.Set("x-api-key", envKey)
		}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return a.getCachedModels(core.ProviderAnthropic), nil
	}
	defer resp.Body.Close()
	var result struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return a.getCachedModels(core.ProviderAnthropic), nil
	}
	var models []ModelInfo
	for _, m := range result.Data {
		models = append(models, ModelInfo{ID: m.ID, Name: m.Name})
	}
	return models, nil
}

func (a *App) getCachedModels(provider core.KnownProvider) []ModelInfo {
	models := llm.GetModels(provider)
	var result []ModelInfo
	for _, m := range models {
		result = append(result, ModelInfo{ID: m.ID, Name: m.ID})
	}
	return result
}

// ─── 普通流式对话 ───

func (a *App) StreamMessage(params map[string]interface{}) error {
	message, _ := params["message"].(string)
	providerStr, _ := params["provider"].(string)
	apiKey, _ := params["apiKey"].(string)
	baseURL, _ := params["baseUrl"].(string)
	modelID, _ := params["model"].(string)

	// 使用当前 provider 作为回退
	cp := a.settings.Current()
	if providerStr == "" && cp != nil {
		providerStr = cp.Type
	}
	if apiKey == "" && cp != nil {
		apiKey = cp.APIKey
	}
	if baseURL == "" && cp != nil {
		baseURL = cp.BaseURL
	}
	if modelID == "" {
		modelID = a.settings.Model
	}

	model := a.resolveModel(providerStr, modelID, baseURL)
	if apiKey == "" {
		apiKey = a.selectAPIKey()
	}
	if apiKey == "" {
		apiKey = core.ResolveAPIKey(model.Provider, "")
	}

	// 构建消息历史
	messages := buildMessages(params, message)
	streamCtx, cancelFn := context.WithCancel(a.ctx)
	a.cancelFn = cancelFn

	if a.tokenStats == nil {
		settings := contextmgr.DefaultSettings(modelID)
		a.ctxSettings = settings
		a.tokenStats = contextmgr.NewTokenStats(settings)
	}
	if a.ctxSettings.MaxContextTokens == 0 {
		a.ctxSettings = contextmgr.DefaultSettings(modelID)
	}
	// 统计整组消息的 token 数，而非逐条累加
	a.tokenStats.Recompute(messages)
	totalTokens := a.tokenStats.Tokens()
	hardLimit := a.ctxSettings.HardLimit()
	if totalTokens > hardLimit && len(messages) > 5 {
		messages = contextmgr.Truncate(messages, int(math.Max(float64(len(messages))*0.5, 5)))
		a.tokenStats.Recompute(messages)
	}

	maxTokens, _ := params["maxTokens"].(float64)
	temperature, _ := params["temperature"].(float64)
	reasoning, _ := params["reasoning"].(string)

	opts := core.SimpleStreamOptions{
		StreamOptions: core.StreamOptions{
			APIKey: apiKey,
		},
	}
	if maxTokens > 0 {
		t := int(maxTokens)
		opts.MaxTokens = &t
	}
	if temperature > 0 {
		t := temperature
		opts.Temperature = &t
	}
	if reasoning != "" {
		opts.Reasoning = core.ThinkingLevel(reasoning)
	}

	stream, err := llm.StreamSimple(streamCtx, model, messages, opts)
	if err != nil {
		runtime.EventsEmit(a.ctx, "stream-error", fmt.Sprintf("Error: %v", err))
		return err
	}

	go func() {
		_, err := stream.ForEach(streamCtx, func(event core.AssistantMessageEvent) error {
			switch e := event.(type) {
			case core.EventThinkingDelta:
				runtime.EventsEmit(a.ctx, "stream-thinking-delta", e.Delta)
			case core.EventToolCallStart:
				data, _ := json.Marshal(map[string]interface{}{"id": e.ID, "name": e.Name})
				runtime.EventsEmit(a.ctx, "stream-tool-call-start", string(data))
			case core.EventToolCallDelta:
				runtime.EventsEmit(a.ctx, "stream-tool-call-delta", e.ArgumentsDelta)
			case core.EventToolCallEnd:
				// Arguments 已经是 JSON 格式，用安全的编码方式传递
				argsStr := string(e.Arguments)
				safeArgs, _ := json.Marshal(argsStr)
				runtime.EventsEmit(a.ctx, "stream-tool-call-end", string(safeArgs))
			case core.EventTextDelta:
				runtime.EventsEmit(a.ctx, "stream-text-delta", e.Delta)
			case core.EventDone:
				a.markKeySuccess()
				runtime.EventsEmit(a.ctx, "stream-done", "")
				return nil
			}
			return nil
		})
		if err != nil {
			a.markKeyFailed(err)
			runtime.EventsEmit(a.ctx, "stream-error", fmt.Sprintf("Error: %v", err))
		}
	}()
	return nil
}

// ─── Agent 流式对话 ───

type AgentRequest struct {
	Message     string                   `json:"message"`
	Messages    []map[string]interface{} `json:"messages"`
	Provider    string                   `json:"provider"`
	APIKey      string                   `json:"apiKey"`
	BaseURL     string                   `json:"baseUrl"`
	Model       string                   `json:"model"`
	MaxTokens   int                      `json:"maxTokens"`
	Temperature float64                  `json:"temperature"`
	Reasoning   string                   `json:"reasoning"`
	Images      []ImageInput             `json:"images,omitempty"`
}

// ImageInput represents an image attachment for multi-modal input.
type ImageInput struct {
	Data     string `json:"data"`
	MimeType string `json:"mimeType,omitempty"`
}

func (a *App) AgentMessage(jsonStr string) error {
	var req AgentRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		runtime.EventsEmit(a.ctx, "agent-error", fmt.Sprintf("parse error: %v", err))
		return err
	}

	model := a.resolveModel(req.Provider, req.Model, req.BaseURL)
	apiKey := req.APIKey
	if apiKey == "" {
		apiKey = a.selectAPIKey()
	}
	if apiKey == "" {
		apiKey = core.ResolveAPIKey(model.Provider, "")
	}

	// 构建 system prompt
	systemPrompt := a.buildSystemPrompt()

	// 加载 skills：从两个位置递归加载 SKILL.md
	// 1) ~/.agent/skills/（home 目录下的通用 skills）
	// 2) 设置的 SkillsDir（项目级 skills，优先级更高，同名覆盖）
	var skills []session.Skill
	var skillDirs []string
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		agentSkillsDir := filepath.Join(homeDir, ".agent", "skills")
		skillDirs = append(skillDirs, agentSkillsDir)
	}
	skillsDir := a.settings.SkillsDir
	if skillsDir == "" {
		skillsDir = filepath.Join(a.dataDir, "skills")
	}
	if skillsDir != "" {
		// 确保项目级目录在最后，这样同名 skill 会覆盖 home 目录的
		skillDirs = append(skillDirs, skillsDir)
	}
	for _, dir := range skillDirs {
		if loaded, diags := session.LoadSkills(dir); len(loaded) > 0 {
			skills = append(skills, loaded...)
			if len(diags) > 0 {
				for _, d := range diags {
					fmt.Printf("[skill] %s: %s\n", d.Path, d.Message)
				}
			}
			fmt.Printf("[skills] loaded %d skill(s) from %s\n", len(loaded), dir)
		}
	}

	// Agent 配置
	config := agent.AgentLoopConfig{
		Model:        model,
		SystemPrompt: systemPrompt,
		Tools:        agenttools.All(),
		Skills:       skills,
		SimpleStreamOptions: core.SimpleStreamOptions{
			StreamOptions: core.StreamOptions{
				APIKey: apiKey,
			},
		},
	}
	if req.MaxTokens > 0 {
		t := req.MaxTokens
		config.SimpleStreamOptions.MaxTokens = &t
	}
	if req.Temperature > 0 {
		config.SimpleStreamOptions.Temperature = &req.Temperature
	}
	if req.Reasoning != "" {
		config.SimpleStreamOptions.Reasoning = core.ThinkingLevel(req.Reasoning)
	}

	messages := a.buildAgentMessages(req)

	// autolearn — 仅在用户启用时处理输入提取记忆
	learned := 0
	if a.mem != nil && a.settings.AutoLearn {
		learned = a.autoLearn().ProcessUserInput(req.Message)
		if learned > 0 {
			config.SystemPrompt = a.buildSystemPrompt()
			_ = a.mem.Save()
		}
	}

	streamCtx, cancelFn := context.WithCancel(a.ctx)
	a.cancelFn = cancelFn

	if a.tokenStats == nil {
		settings := contextmgr.DefaultSettings(req.Model)
		a.ctxSettings = settings
		a.tokenStats = contextmgr.NewTokenStats(settings)
	}

	// 使用 AgentLoopDetailed 同时获取事件流和最终结果
	eventStream, detailed := agent.AgentLoopDetailed(streamCtx, messages, config)

	go func() {
		// 读事件流
		eventStream.ForEach(streamCtx, func(evt agent.AgentEvent) error {
			switch e := evt.(type) {
			case agent.EventMessageUpdate:
				if e.AssistantEvent != nil {
					switch ae := e.AssistantEvent.(type) {
					case core.EventTextDelta:
						runtime.EventsEmit(a.ctx, "agent-text-delta", ae.Delta)
					case core.EventThinkingDelta:
						runtime.EventsEmit(a.ctx, "agent-thinking-delta", ae.Delta)
					case core.EventToolCallStart:
						data, _ := json.Marshal(map[string]interface{}{"id": ae.ID, "name": ae.Name, "arguments": ""})
						runtime.EventsEmit(a.ctx, "agent-tool-call-start", string(data))
					case core.EventToolCallDelta:
						runtime.EventsEmit(a.ctx, "agent-tool-call-delta", ae.ArgumentsDelta)
					case core.EventToolCallEnd:
						// Arguments 是原始 JSON 字节，先转为字符串再安全编码传递
						argsStr := string(ae.Arguments)
						safeArgs, _ := json.Marshal(argsStr)
						runtime.EventsEmit(a.ctx, "agent-tool-call-end", string(safeArgs))
					}
				}
			case agent.EventTurnEnd:
				runtime.EventsEmit(a.ctx, "agent-turn-end", "")
			case agent.EventAgentEnd:
				// Agent 完成
			case agent.EventMessageEnd:
				runtime.EventsEmit(a.ctx, "agent-text-end", "")
			case agent.EventMessageStart:
				runtime.EventsEmit(a.ctx, "agent-text-start", "")
			case agent.EventToolExecStart:
				runtime.EventsEmit(a.ctx, "agent-tool-exec-start", "")
			case agent.EventToolExecEnd:
				result, _ := json.Marshal(map[string]interface{}{"success": !e.IsError})
				if !e.IsError && len(e.Result) > 0 {
					// 解析工具执行结果，提取文本内容供前端显示
					var toolResult core.AgentToolResult
					if err := json.Unmarshal(e.Result, &toolResult); err == nil {
						var textParts []string
						for _, block := range toolResult.Content {
							if tc, ok := block.(core.TextContent); ok {
								textParts = append(textParts, tc.Text)
							}
						}
						if len(textParts) > 0 {
							runtime.EventsEmit(a.ctx, "agent-tool-result", strings.Join(textParts, "\n"))
						} else {
							runtime.EventsEmit(a.ctx, "agent-tool-result", string(e.Result))
						}
					} else {
						runtime.EventsEmit(a.ctx, "agent-tool-result", string(e.Result))
					}
				}
				runtime.EventsEmit(a.ctx, "agent-tool-exec-end", string(result))
			}
			return nil
		})

		// 获取最终结果并提取记忆
		result, err := detailed()
		if err != nil {
			runtime.EventsEmit(a.ctx, "agent-error", fmt.Sprintf("agent error: %v", err))
			runtime.EventsEmit(a.ctx, "agent-done", "")
			return
		}

		// 提取记忆（仅当 AutoLearn 启用时）
		if a.mem != nil && a.settings.AutoLearn && len(result.Messages) > 0 {
			_ = a.autoLearn().MaybeExtract(streamCtx, result.Messages, a.newLLMExtractor(model, apiKey))
			_ = a.mem.Save()
			a.tokenStats.Recompute(result.Messages)
		}

		// 上下文压缩（仅当 AutoCompact 启用且 token 超过软限制时）
		if a.settings.AutoCompact && a.tokenStats != nil && a.tokenStats.ShouldCompact() {
			compactCtx, compactCancel := context.WithTimeout(context.Background(), 30*time.Second)
			streamOpts := []core.SimpleStreamOptions{
				{
					StreamOptions: core.StreamOptions{
						APIKey: apiKey,
					},
				},
			}
			if cr, err := contextmgr.Compact(compactCtx, model, result.Messages, a.ctxSettings, streamOpts...); err == nil {
				fmt.Printf("[compact] saved %d tokens in %v\n", cr.TokensSaved, cr.Duration)
				for _, msg := range cr.NewMessages {
					result.Messages = append(result.Messages, msg)
				}
				a.tokenStats.Recompute(result.Messages)
			} else {
				fmt.Printf("[compact] skipped: %v\n", err)
			}
			compactCancel()
		}

		runtime.EventsEmit(a.ctx, "agent-done", "")
	}()

	return nil
}

func (a *App) resolveModel(providerStr, modelID, baseURL string) core.Model {
	var provider core.KnownProvider
	var api core.KnownAPI
	if providerStr == "anthropic" {
		provider = core.ProviderAnthropic
		api = core.APIAnthropicMessages
	} else {
		provider = core.ProviderOpenAI
		api = core.APIOpenAICompletions
	}
	if modelID == "" {
		modelID = "gpt-4o-mini"
		if providerStr == "anthropic" {
			modelID = "claude-3-5-haiku-20241022"
		}
	}
	model, err := llm.GetModel(provider, modelID)
	if err != nil {
		model = core.Model{
			ID:            modelID,
			Provider:      provider,
			API:           api,
			ContextWindow: 8192,
		}
	}
	if baseURL != "" {
		model.BaseURL = baseURL
	}
	return model
}

func (a *App) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("你是一个有帮助的 AI 助手，可以访问文件系统工具。\n")
	sb.WriteString("你可以读取文件、写入文件、编辑文件、列出目录、执行命令和搜索内容。\n")
	sb.WriteString("当用户提出涉及文件操作的任务时，请使用可用的工具。\n")
	sb.WriteString("在执行工具之前，请清楚地解释你的操作。\n")
	sb.WriteString("\n## 输出规范\n")
	sb.WriteString("- 尽量不要在回复中使用 emoji（表情符号）\n")
	sb.WriteString("- 使用简洁、专业的语言回复\n")
	sb.WriteString("- 代码块使用正确的 markdown 格式\n")
	if a.mem != nil && a.mem.Size() > 0 {
		memText := a.mem.FormatForPrompt()
		if memText != "" {
			sb.WriteString("\n---\n")
			sb.WriteString(memText)
			sb.WriteString("\n---\n")
		}
	}
	return sb.String()
}

func (a *App) autoLearn() *autolearn.AutoLearner {
	settings := a.settings
	skillsDir := settings.SkillsDir
	if skillsDir == "" {
		skillsDir = filepath.Join(a.dataDir, "skills")
	}
	al := autolearn.New(a.mem, autolearn.Settings{
		AutoLearn:     settings.AutoLearn,
		ExtractEveryN: 3,
		MinConfidence: 0.5,
	})
	al.WorkflowDir = filepath.Join(skillsDir, "auto-extracted")
	// 设置 LLM 提取器，用于直接从用户输入提取记忆
	al.LLMExtract = a.newInputLLMExtractor
	return al
}

func (a *App) newLLMExtractor(model core.Model, apiKey string) *autolearn.LLMSimpleExtractor {
	return &autolearn.LLMSimpleExtractor{
		SummarizeFunc: func(ctx context.Context, prompt string) (string, error) {
			msg, err := llm.CompleteSimple(ctx, model, []core.Message{
				core.UserMessage{Content: prompt},
			}, core.SimpleStreamOptions{
				StreamOptions: core.StreamOptions{
					APIKey: apiKey,
				},
			})
			if err != nil {
				return "", err
			}
			var text strings.Builder
			for _, b := range msg.Content {
				if c, ok := b.(core.TextContent); ok {
					text.WriteString(c.Text)
				}
			}
			return text.String(), nil
		},
	}
}

// newInputLLMExtractor 创建一个用于从单条用户输入中提取记忆的 LLM 函数。
// 直接调用 LLM 解析文本，返回 KEY=VALUE 映射。
func (a *App) newInputLLMExtractor(ctx context.Context, text string) (map[string]string, error) {
	if text == "" {
		return nil, nil
	}
	settings := a.settings
	modelID := settings.Model
	if modelID == "" {
		modelID = "gpt-4o-mini"
	}
	cp := settings.Current()
	var apiKey string
	var providerType string
	var baseURL string
	if cp != nil {
		apiKey = cp.APIKey
		providerType = cp.Type
		baseURL = cp.BaseURL
	}
	if apiKey == "" {
		apiKey = core.ResolveAPIKey(core.ProviderOpenAI, "")
	}
	model := a.resolveModel(providerType, modelID, baseURL)

	prompt := fmt.Sprintf(`你是记忆提取助手。从用户的输入中提取需要**长期记住**的事实。
【规则】
- 只提取明确的、可验证的事实（名字、偏好、身份信息、项目名等）
- 每条输出一行 KEY=VALUE
- 允许的 KEY 前缀: user. , project. , fact.
- 没有值得记的，只输出 NONE
- 不要编造，不确定就不提取

用户说: %s`, text)

	msg, err := llm.CompleteSimple(ctx, model, []core.Message{
		core.UserMessage{Content: prompt},
	}, core.SimpleStreamOptions{
		StreamOptions: core.StreamOptions{
			APIKey: apiKey,
		},
	})
	if err != nil {
		return nil, err
	}
	var response strings.Builder
	for _, b := range msg.Content {
		if c, ok := b.(core.TextContent); ok {
			response.WriteString(c.Text)
		}
	}

	result := make(map[string]string)
	for _, line := range strings.Split(response.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "NONE" {
			continue
		}
		// 按 = 或 : 分割
		key, value, found := splitKV(line)
		if found && key != "" && value != "" {
			result[key] = value
		}
	}
	return result, nil
}

// splitKV 辅助函数，将 line 按第一个 = 或 : 分割
func splitKV(line string) (key, value string, found bool) {
	for _, sep := range []string{"=", ":", "："} {
		idx := strings.Index(line, sep)
		if idx > 0 {
			k := strings.TrimSpace(line[:idx])
			v := strings.Trim(strings.TrimSpace(line[idx+len(sep):]), "\"'「」『』")
			if k != "" && v != "" {
				return k, v, true
			}
		}
	}
	return "", "", false
}

// ─── 消息历史构建 ───

// buildMessages 从 params（前端 StreamMessage 请求）中提取消息历史，并追加最新用户消息。
func buildMessages(params map[string]interface{}, currentMessage string) []core.Message {
	raw, _ := params["messages"].([]interface{})
	msgs := parseMessageHistory(raw)
	msgs = append(msgs, buildCurrentUserMessage(currentMessage, extractImagesFromParams(params)))
	return msgs
}

// buildAgentMessages 从 AgentRequest 中构建消息历史，包括现有的 history 和最新的用户消息。
func (a *App) buildAgentMessages(req AgentRequest) []core.Message {
	msgs := parseMessageHistory(toInterfaceSlice(req.Messages))
	msgs = append(msgs, buildCurrentUserMessage(req.Message, req.Images))
	return msgs
}

// extractImagesFromParams pulls the "images" array from a StreamMessage params map.
func extractImagesFromParams(params map[string]interface{}) []ImageInput {
	raw, ok := params["images"].([]interface{})
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make([]ImageInput, 0, len(raw))
	for _, item := range raw {
		if img, ok := item.(map[string]interface{}); ok {
			data, _ := img["data"].(string)
			mime, _ := img["mimeType"].(string)
			if data != "" {
				out = append(out, ImageInput{Data: data, MimeType: mime})
			}
		}
	}
	return out
}

// buildCurrentUserMessage builds a UserMessage that may contain text + image blocks.
func buildCurrentUserMessage(text string, images []ImageInput) core.UserMessage {
	if len(images) == 0 {
		return core.UserMessage{Content: text}
	}
	var blocks []core.ContentBlock
	if text != "" {
		blocks = append(blocks, core.TextContent{Type: "text", Text: text})
	}
	for _, img := range images {
		mime := img.MimeType
		if mime == "" {
			mime = "image/png"
		}
		data := img.Data
		// Strip optional "data:<mime>;base64," prefix
		if idx := strings.Index(data, "base64,"); idx >= 0 {
			data = data[idx+len("base64,"):]
		}
		blocks = append(blocks, core.ImageContent{
			Type:     "image",
			Data:     data,
			MimeType: mime,
		})
	}
	return core.UserMessage{Content: blocks}
}

func toInterfaceSlice(src []map[string]interface{}) []interface{} {
	result := make([]interface{}, len(src))
	for i, m := range src {
		result[i] = m
	}
	return result
}

func parseMessageHistory(raw []interface{}) []core.Message {
	if len(raw) == 0 {
		return nil
	}
	var msgs []core.Message
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		if role == "" {
			continue
		}
		content, _ := m["content"].(string)
		// images: optional array of { data, mimeType } for multi-modal input
		var imageBlocks []core.ContentBlock
		if rawImgs, ok := m["images"].([]interface{}); ok {
			for _, rawImg := range rawImgs {
				if img, ok := rawImg.(map[string]interface{}); ok {
					data, _ := img["data"].(string)
					mime, _ := img["mimeType"].(string)
					if mime == "" {
						mime = "image/png"
					}
					if data != "" {
						// Strip optional "data:<mime>;base64," prefix
						if idx := strings.Index(data, "base64,"); idx >= 0 {
							data = data[idx+len("base64,"):]
						}
						imageBlocks = append(imageBlocks, core.ImageContent{
							Type:     "image",
							Data:     data,
							MimeType: mime,
						})
					}
				}
			}
		}
		if role == "user" {
			if len(imageBlocks) > 0 {
				// Multi-modal: [text?, image...]
				var blocks []core.ContentBlock
				if content != "" {
					blocks = append(blocks, core.TextContent{Type: "text", Text: content})
				}
				blocks = append(blocks, imageBlocks...)
				msgs = append(msgs, core.UserMessage{Content: blocks})
			} else {
				msgs = append(msgs, core.UserMessage{Content: content})
			}
		} else if role == "assistant" {
			msg := core.AssistantMessage{
				Role: "assistant",
			}
			// 尝试保留 tool_calls 信息
			if rawTCs, ok := m["tool_calls"].([]interface{}); ok && len(rawTCs) > 0 {
				var toolCalls []core.ContentBlock
				for _, rawTC := range rawTCs {
					if tc, ok := rawTC.(map[string]interface{}); ok {
						tcID, _ := tc["id"].(string)
						tcName, _ := tc["name"].(string)
						tcArgs, _ := tc["arguments"].(string)
						toolCalls = append(toolCalls, core.ToolCall{
							Type:      "toolCall",
							ID:        tcID,
							Name:      tcName,
							Arguments: json.RawMessage(tcArgs),
						})
					}
				}
				if content != "" {
					msg.Content = append([]core.ContentBlock{core.TextContent{Type: "text", Text: content}}, toolCalls...)
				} else {
					msg.Content = toolCalls
				}
			} else if content != "" {
				msg.Content = []core.ContentBlock{core.TextContent{Type: "text", Text: content}}
			}
			msgs = append(msgs, msg)
		} else if role == "tool" {
			toolCallID, _ := m["tool_call_id"].(string)
			toolName, _ := m["tool_name"].(string)
			if toolCallID == "" {
				toolCallID, _ = m["toolCallID"].(string)
			}
			msgs = append(msgs, core.ToolResultMessage{
				Role:       "tool",
				ToolCallID: toolCallID,
				ToolName:   toolName,
				Content:    []core.ContentBlock{core.TextContent{Type: "text", Text: content}},
			})
		}
	}
	return msgs
}

func getDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".pi-chat-app"
	}
	return filepath.Join(home, ".pi-chat-app")
}
