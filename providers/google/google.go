package google

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	piai "pi-ai-go"
)

// Options holds Google-specific options.
type Options struct {
	ToolChoice any `json:"toolChoice,omitempty"`
	Thinking   *ThinkingConfig `json:"thinking,omitempty"`
}

// ThinkingConfig configures thinking/reasoning.
type ThinkingConfig struct {
	Enabled     bool   `json:"enabled"`
	BudgetTokens int   `json:"budgetTokens,omitempty"`
	Level       string `json:"level,omitempty"` // MINIMAL, LOW, MEDIUM, HIGH
}

// Provider implements the Google Generative AI API.
type Provider struct{}

// New creates a new Google provider.
func New() *Provider {
	return &Provider{}
}

func (p *Provider) Stream(model piai.Model, ctx piai.Context, opts piai.StreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	return streamGoogle(model, ctx, opts, Options{})
}

func (p *Provider) StreamSimple(model piai.Model, ctx piai.Context, opts piai.SimpleStreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	googleOpts := Options{}
	if opts.Reasoning != "" {
		googleOpts.Thinking = &ThinkingConfig{
			Enabled: true,
			Level:   mapThinkingLevel(opts.Reasoning),
		}
		if opts.ThinkingBudgets != nil {
			if budget, ok := opts.ThinkingBudgets[string(opts.Reasoning)]; ok {
				googleOpts.Thinking.BudgetTokens = budget
			}
		}
	}
	return streamGoogle(model, ctx, opts.StreamOptions, googleOpts)
}

func mapThinkingLevel(level piai.ThinkingLevel) string {
	switch level {
	case piai.ThinkingMinimal:
		return "MINIMAL"
	case piai.ThinkingLow:
		return "LOW"
	case piai.ThinkingMedium:
		return "MEDIUM"
	case piai.ThinkingHigh, piai.ThinkingXHigh:
		return "HIGH"
	default:
		return "MEDIUM"
	}
}

func streamGoogle(model piai.Model, c piai.Context, opts piai.StreamOptions, googleOpts Options) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	apiKey := piai.ResolveAPIKey(model.Provider, opts.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("google: no API key provided")
	}

	baseURL := piai.ResolveBaseURL(model, defaultBaseURL)

	body, err := buildGoogleBody(model, c, opts, googleOpts)
	if err != nil {
		return nil, fmt.Errorf("google: failed to build request: %w", err)
	}

	if opts.OnPayload != nil {
		opts.OnPayload(body)
	}

	stream := piai.NewEventStream[piai.AssistantMessageEvent, piai.AssistantMessage]()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				stream.Error(fmt.Errorf("google: panic: %v", r))
			}
		}()

		msg, err := doGoogleStream(baseURL, apiKey, model, body, stream, opts)
		if err != nil {
			stream.Error(err)
			return
		}
		stream.End(msg)
	}()

	return stream, nil
}

func buildGoogleBody(model piai.Model, c piai.Context, opts piai.StreamOptions, googleOpts Options) (map[string]any, error) {
	body := map[string]any{}

	// Contents (messages)
	contents, err := ConvertMessages(c.Messages)
	if err != nil {
		return nil, err
	}
	body["contents"] = contents

	// System instruction
	if c.SystemPrompt != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []any{
				map[string]any{"text": c.SystemPrompt},
			},
		}
	}

	// Generation config
	genConfig := map[string]any{}
	if opts.MaxTokens != nil && *opts.MaxTokens > 0 {
		genConfig["maxOutputTokens"] = *opts.MaxTokens
	} else if model.MaxTokens > 0 {
		genConfig["maxOutputTokens"] = model.MaxTokens
	}
	if opts.Temperature != nil {
		genConfig["temperature"] = *opts.Temperature
	}
	if len(genConfig) > 0 {
		body["generationConfig"] = genConfig
	}

	// Tools
	if len(c.Tools) > 0 {
		body["tools"] = ConvertTools(c.Tools)
	}

	// Tool config
	if googleOpts.ToolChoice != nil {
		body["toolConfig"] = map[string]any{
			"functionCallingConfig": googleOpts.ToolChoice,
		}
	}

	// Thinking config
	if googleOpts.Thinking != nil {
		thinkingConfig := map[string]any{
			"includeThoughts": googleOpts.Thinking.Enabled,
		}
		if googleOpts.Thinking.BudgetTokens > 0 {
			thinkingConfig["thinkingBudget"] = googleOpts.Thinking.BudgetTokens
		} else if googleOpts.Thinking.Level != "" {
			thinkingConfig["thinkingLevel"] = googleOpts.Thinking.Level
		}
		body["thinkingConfig"] = thinkingConfig
	}

	return body, nil
}

func doGoogleStream(baseURL, apiKey string, model piai.Model, body map[string]any, stream *piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], opts piai.StreamOptions) (piai.AssistantMessage, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return piai.AssistantMessage{}, err
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", baseURL, model.ID, apiKey)

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return piai.AssistantMessage{}, err
	}

	req.Header.Set("Content-Type", "application/json")

	for k, v := range model.Headers {
		req.Header.Set(k, v)
	}
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return piai.AssistantMessage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return piai.AssistantMessage{}, fmt.Errorf("google: API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return processGoogleSSE(resp.Body, stream, model, opts)
}

func processGoogleSSE(body io.Reader, stream *piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], model piai.Model, opts piai.StreamOptions) (piai.AssistantMessage, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var (
		msg       piai.AssistantMessage
		textBuf   strings.Builder
		thinkBuf  strings.Builder
		toolCalls []piai.ToolCall
	)

	msg.API = model.API
	msg.Provider = model.Provider
	msg.Model = model.ID
	msg.Role = "assistant"
	msg.Timestamp = time.Now()

	stream.Push(piai.EventStart{
		Type:      "start",
		API:       model.API,
		Provider:  model.Provider,
		Model:     model.ID,
		Timestamp: time.Now(),
	})

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		if opts.OnResponse != nil {
			opts.OnResponse(data)
		}

		var chunk map[string]any
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		candidates, ok := chunk["candidates"].([]any)
		if !ok || len(candidates) == 0 {
			continue
		}

		candidate, ok := candidates[0].(map[string]any)
		if !ok {
			continue
		}

		// Finish reason
		if finishReason, ok := candidate["finishReason"].(string); ok {
			msg.StopReason = MapStopReason(finishReason)
		}

		// Content parts
		content, ok := candidate["content"].(map[string]any)
		if !ok {
			continue
		}

		parts, ok := content["parts"].([]any)
		if !ok {
			continue
		}

		for _, part := range parts {
			p, ok := part.(map[string]any)
			if !ok {
				continue
			}

			if IsThinkingPart(p) {
				if text, ok := p["text"].(string); ok {
					thinkBuf.WriteString(text)
					stream.Push(piai.EventThinkingDelta{
						Type:  "thinking_delta",
						Delta: text,
					})
				}
			} else if text, ok := p["text"].(string); ok {
				textBuf.WriteString(text)
				stream.Push(piai.EventTextDelta{
					Type:  "text_delta",
					Delta: text,
				})
			} else if fc, ok := p["functionCall"].(map[string]any); ok {
				name, _ := fc["name"].(string)
				args, _ := fc["args"].(map[string]any)
				argsBytes, _ := json.Marshal(args)
				id := fmt.Sprintf("call_%d", len(toolCalls))
				tc := piai.ToolCall{
					Type:      "toolCall",
					ID:        id,
					Name:      name,
					Arguments: argsBytes,
				}
				toolCalls = append(toolCalls, tc)
				stream.Push(piai.EventToolCallStart{
					Type: "toolcall_start",
					ID:   id,
					Name: name,
				})
				stream.Push(piai.EventToolCallEnd{
					Type:      "toolcall_end",
					ID:        id,
					Arguments: argsBytes,
				})
			}
		}

		// Usage metadata
		if usageMetadata, ok := chunk["usageMetadata"].(map[string]any); ok {
			msg.Usage.Input = int(getFloat(usageMetadata, "promptTokenCount"))
			msg.Usage.Output = int(getFloat(usageMetadata, "candidatesTokenCount"))
			msg.Usage.TotalTokens = int(getFloat(usageMetadata, "totalTokenCount"))
		}
	}

	// Finalize
	if textBuf.Len() > 0 {
		msg.Content = append(msg.Content, piai.TextContent{
			Type: "text",
			Text: textBuf.String(),
		})
		stream.Push(piai.EventTextEnd{Type: "text_end"})
	}

	if thinkBuf.Len() > 0 {
		msg.Content = append(msg.Content, piai.ThinkingContent{
			Type:     "thinking",
			Thinking: thinkBuf.String(),
		})
	}

	for _, tc := range toolCalls {
		msg.Content = append(msg.Content, tc)
	}

	msg.Usage.Cost = piai.CalculateCost(model, msg.Usage)

	stream.Push(piai.EventDone{
		Type:    "done",
		Message: msg,
	})

	return msg, nil
}

func getFloat(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}
