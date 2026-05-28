// Package bedrock implements the Amazon Bedrock Converse Stream API provider.
package bedrock

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	piai "pi-ai-go"
)

const defaultRegion = "us-east-1"

// Options holds Bedrock-specific options.
type Options struct {
	Region             string `json:"region,omitempty"`
	Profile            string `json:"profile,omitempty"`
	ToolChoice         any    `json:"toolChoice,omitempty"`
	Reasoning          bool   `json:"reasoning,omitempty"`
	ThinkingBudgets    map[string]int `json:"thinkingBudgets,omitempty"`
	InterleavedThinking bool `json:"interleavedThinking,omitempty"`
	ThinkingDisplay    string `json:"thinkingDisplay,omitempty"`
	RequestMetadata    map[string]string `json:"requestMetadata,omitempty"`
	BearerToken        string `json:"bearerToken,omitempty"`
}

// Provider implements the Amazon Bedrock Converse Stream API.
type Provider struct{}

// New creates a new Bedrock provider.
func New() *Provider {
	return &Provider{}
}

func (p *Provider) Stream(model piai.Model, ctx piai.Context, opts piai.StreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	return streamBedrock(model, ctx, opts, Options{})
}

func (p *Provider) StreamSimple(model piai.Model, ctx piai.Context, opts piai.SimpleStreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	bedrockOpts := Options{
		Reasoning: true,
	}
	if opts.ThinkingBudgets != nil {
		bedrockOpts.ThinkingBudgets = opts.ThinkingBudgets
	}
	return streamBedrock(model, ctx, opts.StreamOptions, bedrockOpts)
}

func streamBedrock(model piai.Model, c piai.Context, opts piai.StreamOptions, bedrockOpts Options) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	apiKey := piai.ResolveAPIKey(model.Provider, opts.APIKey)
	region := bedrockOpts.Region
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		region = defaultRegion
	}

	body, err := buildBedrockBody(model, c, opts, bedrockOpts)
	if err != nil {
		return nil, fmt.Errorf("bedrock: failed to build request: %w", err)
	}

	if opts.OnPayload != nil {
		opts.OnPayload(body)
	}

	stream := piai.NewEventStream[piai.AssistantMessageEvent, piai.AssistantMessage]()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				stream.Error(fmt.Errorf("bedrock: panic: %v", r))
			}
		}()

		msg, err := doBedrockStream(region, apiKey, model, body, stream, opts, bedrockOpts)
		if err != nil {
			stream.Error(err)
			return
		}
		stream.End(msg)
	}()

	return stream, nil
}

func buildBedrockBody(model piai.Model, c piai.Context, opts piai.StreamOptions, bedrockOpts Options) (map[string]any, error) {
	// Convert messages to Bedrock format
	messages, err := convertMessages(c.Messages)
	if err != nil {
		return nil, err
	}

	body := map[string]any{
		"messages": messages,
	}

	// System
	if c.SystemPrompt != "" {
		body["system"] = []any{
			map[string]any{
				"text": c.SystemPrompt,
			},
		}
	}

	// Inference config
	inferenceConfig := map[string]any{}
	if opts.MaxTokens != nil && *opts.MaxTokens > 0 {
		inferenceConfig["maxTokens"] = *opts.MaxTokens
	} else if model.MaxTokens > 0 {
		inferenceConfig["maxTokens"] = model.MaxTokens
	}
	if opts.Temperature != nil {
		inferenceConfig["temperature"] = *opts.Temperature
	}
	if len(inferenceConfig) > 0 {
		body["inferenceConfig"] = inferenceConfig
	}

	// Tools
	if len(c.Tools) > 0 {
		body["toolConfig"] = map[string]any{
			"tools": convertTools(c.Tools),
		}
	}

	// Tool choice
	if bedrockOpts.ToolChoice != nil {
		if tc, ok := body["toolConfig"].(map[string]any); ok {
			tc["toolChoice"] = bedrockOpts.ToolChoice
		}
	}

	// Thinking/Reasoning
	if bedrockOpts.Reasoning {
		thinkingConfig := map[string]any{
			"enabled": true,
		}
		if bedrockOpts.ThinkingBudgets != nil {
			if budget, ok := bedrockOpts.ThinkingBudgets["medium"]; ok {
				thinkingConfig["budgetTokens"] = budget
			}
		}
		body["thinking"] = thinkingConfig
	}

	return body, nil
}

func convertMessages(messages []piai.Message) ([]map[string]any, error) {
	var result []map[string]any

	for _, msg := range messages {
		switch m := msg.(type) {
		case piai.UserMessage:
			content, err := convertUserContent(m.Content)
			if err != nil {
				return nil, err
			}
			result = append(result, map[string]any{
				"role":    "user",
				"content": content,
			})

		case piai.AssistantMessage:
			content := convertAssistantContent(m.Content)
			result = append(result, map[string]any{
				"role":    "assistant",
				"content": content,
			})

		case piai.ToolResultMessage:
			content := convertToolResultContent(m.Content)
			result = append(result, map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"toolResult": map[string]any{
							"toolUseId": m.ToolCallID,
							"content":   content,
							"status":    mapStatus(m.IsError),
						},
					},
				},
			})
		}
	}

	return result, nil
}

func convertUserContent(content any) ([]any, error) {
	switch c := content.(type) {
	case string:
		return []any{
			map[string]any{"text": c},
		}, nil
	case []piai.ContentBlock:
		var blocks []any
		for _, block := range c {
			switch b := block.(type) {
			case piai.TextContent:
				blocks = append(blocks, map[string]any{"text": b.Text})
			case piai.ImageContent:
				blocks = append(blocks, map[string]any{
					"image": map[string]any{
						"format": mimeToFormat(b.MimeType),
						"source": map[string]any{
							"bytes": b.Data,
						},
					},
				})
			}
		}
		return blocks, nil
	default:
		return []any{
			map[string]any{"text": fmt.Sprintf("%v", content)},
		}, nil
	}
}

func convertAssistantContent(content []piai.ContentBlock) []any {
	var blocks []any
	for _, block := range content {
		switch b := block.(type) {
		case piai.TextContent:
			blocks = append(blocks, map[string]any{"text": b.Text})
		case piai.ThinkingContent:
			blocks = append(blocks, map[string]any{
				"thinking": map[string]any{
					"thinking": b.Thinking,
				},
			})
		case piai.ToolCall:
			blocks = append(blocks, map[string]any{
				"toolUse": map[string]any{
					"toolUseId": b.ID,
					"name":      b.Name,
					"input":     json.RawMessage(b.Arguments),
				},
			})
		}
	}
	return blocks
}

func convertToolResultContent(content []piai.ContentBlock) []any {
	var blocks []any
	for _, block := range content {
		if text, ok := block.(piai.TextContent); ok {
			blocks = append(blocks, map[string]any{"text": text.Text})
		}
	}
	return blocks
}

func convertTools(tools []piai.Tool) []any {
	result := make([]any, len(tools))
	for i, tool := range tools {
		t := map[string]any{
			"toolSpec": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
			},
		}
		if len(tool.Parameters) > 0 {
			var params map[string]any
			json.Unmarshal(tool.Parameters, &params)
			t["toolSpec"].(map[string]any)["inputSchema"] = map[string]any{
				"json": params,
			}
		}
		result[i] = t
	}
	return result
}

func mapStatus(isError bool) string {
	if isError {
		return "error"
	}
	return "success"
}

func mimeToFormat(mimeType string) string {
	parts := strings.Split(mimeType, "/")
	if len(parts) > 1 {
		return parts[1]
	}
	return mimeType
}

func doBedrockStream(region, apiKey string, model piai.Model, body map[string]any, stream *piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], opts piai.StreamOptions, bedrockOpts Options) (piai.AssistantMessage, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return piai.AssistantMessage{}, err
	}

	url := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/converse-stream", region, model.ID)

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return piai.AssistantMessage{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	for k, v := range model.Headers {
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
		return piai.AssistantMessage{}, fmt.Errorf("bedrock: API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return processBedrockSSE(resp.Body, stream, model, opts)
}

func processBedrockSSE(body io.Reader, stream *piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], model piai.Model, opts piai.StreamOptions) (piai.AssistantMessage, error) {
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

		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		// Handle different event types
		if contentBlockDelta, ok := event["contentBlockDelta"].(map[string]any); ok {
			delta, _ := contentBlockDelta["delta"].(map[string]any)

			if text, ok := delta["text"].(string); ok {
				textBuf.WriteString(text)
				stream.Push(piai.EventTextDelta{
					Type:  "text_delta",
					Delta: text,
				})
			}

			if thinking, ok := delta["thinking"].(map[string]any); ok {
				if t, ok := thinking["thinking"].(string); ok {
					thinkBuf.WriteString(t)
					stream.Push(piai.EventThinkingDelta{
						Type:  "thinking_delta",
						Delta: t,
					})
				}
			}

			if toolUse, ok := delta["toolUse"].(map[string]any); ok {
				if input, ok := toolUse["input"].(string); ok {
					if len(toolCalls) > 0 {
						last := &toolCalls[len(toolCalls)-1]
						last.Arguments = append(last.Arguments, []byte(input)...)
						stream.Push(piai.EventToolCallDelta{
							Type:           "toolcall_delta",
							ID:             last.ID,
							ArgumentsDelta: input,
						})
					}
				}
			}
		}

		if contentBlockStart, ok := event["contentBlockStart"].(map[string]any); ok {
			start, _ := contentBlockStart["start"].(map[string]any)
			if toolUse, ok := start["toolUse"].(map[string]any); ok {
				id, _ := toolUse["toolUseId"].(string)
				name, _ := toolUse["name"].(string)
				toolCalls = append(toolCalls, piai.ToolCall{
					Type: "toolCall",
					ID:   id,
					Name: name,
				})
				stream.Push(piai.EventToolCallStart{
					Type: "toolcall_start",
					ID:   id,
					Name: name,
				})
			}
		}

		if messageStop, ok := event["messageStop"].(map[string]any); ok {
			if stopReason, ok := messageStop["stopReason"].(string); ok {
				msg.StopReason = mapBedrockStopReason(stopReason)
			}
		}

		if metadata, ok := event["metadata"].(map[string]any); ok {
			if usage, ok := metadata["usage"].(map[string]any); ok {
				msg.Usage.Input = int(getFloat(usage, "inputTokens"))
				msg.Usage.Output = int(getFloat(usage, "outputTokens"))
				msg.Usage.CacheRead = int(getFloat(usage, "cacheReadInputTokens"))
				msg.Usage.CacheWrite = int(getFloat(usage, "cacheWriteInputTokens"))
			}
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
		stream.Push(piai.EventToolCallEnd{
			Type:      "toolcall_end",
			ID:        tc.ID,
			Arguments: tc.Arguments,
		})
		msg.Content = append(msg.Content, tc)
	}

	msg.Usage.TotalTokens = msg.Usage.Input + msg.Usage.Output + msg.Usage.CacheRead + msg.Usage.CacheWrite
	msg.Usage.Cost = piai.CalculateCost(model, msg.Usage)

	stream.Push(piai.EventDone{
		Type:    "done",
		Message: msg,
	})

	return msg, nil
}

func mapBedrockStopReason(reason string) piai.StopReason {
	switch reason {
	case "end_turn":
		return piai.StopStop
	case "tool_use":
		return piai.StopToolUse
	case "max_tokens":
		return piai.StopLength
	case "stop_sequence":
		return piai.StopStop
	default:
		return piai.StopStop
	}
}

func getFloat(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}
