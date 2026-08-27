package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"pi-ai-go/core"
	"pi-ai-go/llm"
)

// GetModels 获取指定 provider 的可用模型列表。
func (a *App) GetModels(params map[string]interface{}) ([]ModelInfo, error) {
	providerStr, _ := params["provider"].(string)
	baseURL, _ := params["baseUrl"].(string)
	apiKey, _ := params["apiKey"].(string)
	LogInfo("[getModels] provider=%s baseURL=%s", providerStr, baseURL)

	if providerStr == "" {
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
		return a.getCachedModels(core.ProviderOpenAI), nil
	}
	models, err := fetchModelList(providerStr, baseURL, apiKey)
	if err != nil {
		LogWarn("[getModels] fetch failed: %v, using cached models", err)
		return a.getCachedModels(providerToKnown(providerStr)), nil
	}
	return models, nil
}

func providerToKnown(provider string) core.KnownProvider {
	switch strings.ToLower(provider) {
	case "anthropic":
		return core.ProviderAnthropic
	case "google":
		return core.ProviderGoogle
	case "deepseek":
		return core.ProviderDeepSeek
	case "mistral":
		return core.ProviderMistral
	default:
		return core.ProviderOpenAI
	}
}

type providerHeaderConfig struct {
	defaultBaseURL string
	header         []struct{ key, valueTemplate string }
}

var providerModelListConfigs = map[string]providerHeaderConfig{
	"openai": {
		defaultBaseURL: "https://api.openai.com/v1",
		header: []struct{ key, valueTemplate string }{
			{"Authorization", "Bearer %s"},
		},
	},
	"openai-compatible": {
		defaultBaseURL: "https://api.openai.com/v1",
		header: []struct{ key, valueTemplate string }{
			{"Authorization", "Bearer %s"},
		},
	},
	"anthropic": {
		defaultBaseURL: "https://api.anthropic.com/v1",
		header: []struct{ key, valueTemplate string }{
			{"x-api-key", "%s"},
			{"anthropic-version", "2023-06-01"},
		},
	},
}

func fetchModelList(provider, baseURL, apiKey string) ([]ModelInfo, error) {
	cfg, ok := providerModelListConfigs[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	url := strings.TrimRight(baseURL, "/")
	if url == "" {
		url = cfg.defaultBaseURL
	}
	url += "/models"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	for _, h := range cfg.header {
		val := fmt.Sprintf(h.valueTemplate, apiKey)
		req.Header.Set(h.key, val)
	}
	req.Header.Set("Content-Type", "application/json")

	if apiKey == "" {
		var knownProvider core.KnownProvider
		switch provider {
		case "openai":
			knownProvider = core.ProviderOpenAI
		case "anthropic":
			knownProvider = core.ProviderAnthropic
		}
		if knownProvider != "" {
			envKey := core.ResolveAPIKey(knownProvider, "")
			if envKey != "" {
				for _, h := range cfg.header {
					req.Header.Set(h.key, fmt.Sprintf(h.valueTemplate, envKey))
				}
			}
		}
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	var result struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
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
		name := m.Name
		if name == "" {
			name = m.ID
		}
		result = append(result, ModelInfo{ID: m.ID, Name: name})
	}
	return result
}

// resolveModel 把 provider 字符串 + 模型 ID 映射为 core.Model。
func (a *App) resolveModel(providerStr, modelID, baseURL string) core.Model {
	providerStr = strings.ToLower(providerStr)
	var provider core.KnownProvider
	var api core.KnownAPI
	switch providerStr {
	case "anthropic":
		provider = core.ProviderAnthropic
		api = core.APIAnthropicMessages
	case "google":
		provider = core.ProviderGoogle
		api = core.APIGoogleGenerative
	case "deepseek":
		provider = core.ProviderDeepSeek
		api = core.APIOpenAICompletions
	case "mistral":
		provider = core.ProviderMistral
		api = core.APIMistralConversations
	default:
		provider = core.ProviderOpenAI
		api = core.APIOpenAICompletions
	}
	if modelID == "" {
		modelID = "auto"
	}
	model, err := llm.GetModel(provider, modelID)
	if err != nil {
		if provider != core.ProviderOpenAI && providerStr != "openai" {
			if m, err2 := llm.GetModel(core.ProviderOpenAI, modelID); err2 == nil {
				model = m
				err = nil
			}
		}
		if err != nil {
			LogWarn("[model] %v, using fallback model %s/%s", err, provider, modelID)
			model = core.Model{
				ID:            modelID,
				Provider:      provider,
				API:           api,
				ContextWindow: 8192,
			}
		}
	}
	if baseURL != "" {
		model.BaseURL = baseURL
	}
	LogDebug("[model] resolved: id=%s provider=%s api=%s baseURL=%s ctxWindow=%d",
		model.ID, model.Provider, model.API, model.BaseURL, model.ContextWindow)
	return model
}

// providerFromRequest 从请求字段解析 provider/apiKey/baseURL/model，缺失时回退到设置。
func (a *App) providerFromRequest(provider, apiKey, baseURL, modelID string) (core.Model, string, core.SimpleStreamOptions) {
	cp := a.settings.Current()
	if provider == "" && cp != nil {
		provider = cp.Type
	}
	if apiKey == "" && cp != nil {
		apiKey = cp.APIKey
	}
	if apiKey == "" {
		apiKey = core.ResolveAPIKey(core.ProviderOpenAI, "")
	}
	if baseURL == "" && cp != nil {
		baseURL = cp.BaseURL
	}
	if modelID == "" {
		modelID = a.settings.Model
	}

	model := a.resolveModel(provider, modelID, baseURL)
	opts := core.SimpleStreamOptions{
		StreamOptions: core.StreamOptions{
			APIKey: apiKey,
		},
	}
	return model, provider, opts
}
