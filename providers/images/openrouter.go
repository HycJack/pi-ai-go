// Package images implements image generation providers.
package images

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	piai "pi-ai-go"
)

const defaultOpenRouterBaseURL = "https://openrouter.ai/api/v1"

// OpenRouterProvider implements image generation via OpenRouter.
type OpenRouterProvider struct{}

// NewOpenRouter creates a new OpenRouter images provider.
func NewOpenRouter() *OpenRouterProvider {
	return &OpenRouterProvider{}
}

func (p *OpenRouterProvider) GenerateImages(model piai.ImagesModel, c piai.Context, opts piai.ImageOptions) (*piai.AssistantImages, error) {
	apiKey := piai.ResolveAPIKey(model.Provider, opts.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("openrouter-images: no API key provided")
	}

	baseURL := piai.ResolveBaseURL(piai.Model{
		BaseURL: model.BaseURL,
	}, defaultOpenRouterBaseURL)

	// Build request
	messages := []map[string]any{}
	if c.SystemPrompt != "" {
		messages = append(messages, map[string]any{
			"role":    "system",
			"content": c.SystemPrompt,
		})
	}

	for _, msg := range c.Messages {
		if userMsg, ok := msg.(piai.UserMessage); ok {
			content := fmt.Sprintf("%v", userMsg.Content)
			messages = append(messages, map[string]any{
				"role":    "user",
				"content": content,
			})
		}
	}

	body := map[string]any{
		"model":      model.ID,
		"messages":   messages,
		"modalities": []string{"image"},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := baseURL + "/chat/completions"

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	for k, v := range model.Headers {
		req.Header.Set(k, v)
	}
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openrouter-images: API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Images []struct {
					URL string `json:"url"`
				} `json:"images"`
			} `json:"message"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	images := piai.AssistantImages{
		API:        "openrouter-images",
		Provider:   model.Provider,
		Model:      model.ID,
		StopReason: piai.StopStop,
		Timestamp:  time.Now(),
	}

	if len(result.Choices) > 0 {
		for _, img := range result.Choices[0].Message.Images {
			// Parse data: URLs
			data := img.URL
			mimeType := "image/png"

			if strings.HasPrefix(data, "data:") {
				parts := strings.SplitN(data, ",", 2)
				if len(parts) == 2 {
					headerParts := strings.SplitN(parts[0], ";", 2)
					if len(headerParts) > 0 {
						mimeType = strings.TrimPrefix(headerParts[0], "data:")
					}
					data = parts[1]
				}
			}

			images.Output = append(images.Output, piai.ImageData{
				Data:     data,
				MimeType: mimeType,
			})
		}
	}

	if result.Usage != nil {
		images.Usage = &piai.Usage{
			Input:       result.Usage.PromptTokens,
			Output:      result.Usage.CompletionTokens,
			TotalTokens: result.Usage.TotalTokens,
		}
	}

	return &images, nil
}
