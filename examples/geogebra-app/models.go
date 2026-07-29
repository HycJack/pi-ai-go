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

// GetModels fetches the list of available models from the provider's API.
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
		// Return built-in models
		return a.getCachedModels(core.ProviderOpenAI), nil
	}
	models, err := fetchModelList(providerStr, baseURL, apiKey)
	if err != nil {
		// Fallback to built-in
		LogWarn("[getModels] fetch failed: %v, using cached models", err)
		known := providerToKnown(providerStr)
		return a.getCachedModels(known), nil
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
