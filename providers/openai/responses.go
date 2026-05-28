package openai

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

// ResponsesOptions holds OpenAI Responses-specific options.
type ResponsesOptions struct {
	ReasoningEffort  string `json:"reasoningEffort,omitempty"`
	ReasoningSummary string `json:"reasoningSummary,omitempty"`
	ServiceTier      string `json:"serviceTier,omitempty"`
}

// ResponsesProvider implements the OpenAI Responses API.
type ResponsesProvider struct{}

// NewResponses creates a new OpenAI Responses provider.
func NewResponses() *ResponsesProvider {
	return &ResponsesProvider{}
}

func (p *ResponsesProvider) Stream(model piai.Model, ctx piai.Context, opts piai.StreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	return streamResponses(model, ctx, opts, ResponsesOptions{})
}

func (p *ResponsesProvider) StreamSimple(model piai.Model, ctx piai.Context, opts piai.SimpleStreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	responsesOpts := ResponsesOptions{}
	if opts.Reasoning != "" {
		responsesOpts.ReasoningEffort = string(clampReasoning(opts.Reasoning))
	}
	return streamResponses(model, ctx, opts.StreamOptions, responsesOpts)
}

func streamResponses(model piai.Model, c piai.Context, opts piai.StreamOptions, responsesOpts ResponsesOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	apiKey := piai.ResolveAPIKey(model.Provider, opts.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("openai-responses: no API key provided")
	}

	baseURL := piai.ResolveBaseURL(model, defaultResponsesURL)

	body, err := buildResponsesBody(model, c, opts, responsesOpts)
	if err != nil {
		return nil, fmt.Errorf("openai-responses: failed to build request: %w", err)
	}

	if opts.OnPayload != nil {
		opts.OnPayload(body)
	}

	stream := piai.NewEventStream[piai.AssistantMessageEvent, piai.AssistantMessage]()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				stream.Error(fmt.Errorf("openai-responses: panic: %v", r))
			}
		}()

		msg, err := doResponsesStream(baseURL, apiKey, model, body, stream, opts)
		if err != nil {
			stream.Error(err)
			return
		}
		stream.End(msg)
	}()

	return stream, nil
}

func buildResponsesBody(model piai.Model, c piai.Context, opts piai.StreamOptions, responsesOpts ResponsesOptions) (map[string]any, error) {
	body := map[string]any{
		"model":  model.ID,
		"stream": true,
	}

	if opts.MaxTokens != nil && *opts.MaxTokens > 0 {
		body["max_output_tokens"] = *opts.MaxTokens
	} else if model.MaxTokens > 0 {
		body["max_output_tokens"] = model.MaxTokens
	}

	if opts.Temperature != nil {
		body["temperature"] = *opts.Temperature
	}

	// Build input
	input := []map[string]any{}

	if c.SystemPrompt != "" {
		input = append(input, map[string]any{
			"role":    "system",
			"content": c.SystemPrompt,
		})
	}

	msgs, err := convertResponsesMessages(c.Messages)
	if err != nil {
		return nil, err
	}
	input = append(input, msgs...)
	body["input"] = input

	// Tools
	if len(c.Tools) > 0 {
		body["tools"] = convertResponsesTools(c.Tools)
	}

	// Reasoning
	if responsesOpts.ReasoningEffort != "" {
		reasoning := map[string]any{
			"effort": responsesOpts.ReasoningEffort,
		}
		if responsesOpts.ReasoningSummary != "" {
			reasoning["summary"] = responsesOpts.ReasoningSummary
		}
		body["reasoning"] = reasoning
	}

	// Service tier
	if responsesOpts.ServiceTier != "" {
		body["service_tier"] = responsesOpts.ServiceTier
	}

	return body, nil
}

func convertResponsesMessages(messages []piai.Message) ([]map[string]any, error) {
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
			for _, block := range m.Content {
				switch b := block.(type) {
				case piai.TextContent:
					result = append(result, map[string]any{
						"type": "message",
						"role": "assistant",
						"content": []any{
							map[string]any{
								"type": "output_text",
								"text": b.Text,
							},
						},
					})
				case piai.ToolCall:
					result = append(result, map[string]any{
						"type":      "function_call",
						"id":        b.ID,
						"name":      b.Name,
						"arguments": string(b.Arguments),
						"call_id":   b.ID,
					})
				}
			}

		case piai.ToolResultMessage:
			content := convertToolResultContent(m.Content)
			result = append(result, map[string]any{
				"type":    "function_call_output",
				"call_id": m.ToolCallID,
				"output":  content,
			})
		}
	}

	return result, nil
}

func convertResponsesTools(tools []piai.Tool) []map[string]any {
	result := make([]map[string]any, len(tools))
	for i, tool := range tools {
		t := map[string]any{
			"type": "function",
			"name":        tool.Name,
			"description": tool.Description,
		}
		if len(tool.Parameters) > 0 {
			var params map[string]any
			if err := json.Unmarshal(tool.Parameters, &params); err == nil {
				t["parameters"] = params
			}
		}
		result[i] = t
	}
	return result
}

func doResponsesStream(baseURL, apiKey string, model piai.Model, body map[string]any, stream *piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], opts piai.StreamOptions) (piai.AssistantMessage, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return piai.AssistantMessage{}, err
	}

	url := baseURL + "/responses"

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return piai.AssistantMessage{}, err
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
		return piai.AssistantMessage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return piai.AssistantMessage{}, fmt.Errorf("openai-responses: API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return processResponsesSSE(resp.Body, stream, model, opts)
}

func processResponsesSSE(body io.Reader, stream *piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], model piai.Model, opts piai.StreamOptions) (piai.AssistantMessage, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var (
		msg       piai.AssistantMessage
		textBuf   strings.Builder
		toolCalls map[string]*piai.ToolCall
	)

	msg.API = model.API
	msg.Provider = model.Provider
	msg.Model = model.ID
	msg.Role = "assistant"
	msg.Timestamp = time.Now()
	toolCalls = make(map[string]*piai.ToolCall)

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
		if data == "[DONE]" {
			break
		}

		if opts.OnResponse != nil {
			opts.OnResponse(data)
		}

		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		eventType, _ := event["type"].(string)

		switch eventType {
		case "response.created":
			response, _ := event["response"].(map[string]any)
			if response != nil {
				if usage, ok := response["usage"].(map[string]any); ok {
					msg.Usage.Input = int(getFloat(usage, "input_tokens"))
					msg.Usage.Output = int(getFloat(usage, "output_tokens"))
				}
			}

		case "response.output_item.added":
			item, _ := event["item"].(map[string]any)
			if item != nil {
				itemType, _ := item["type"].(string)
				switch itemType {
				case "function_call":
					id, _ := item["id"].(string)
					name, _ := item["name"].(string)
					callID, _ := item["call_id"].(string)
					toolCalls[callID] = &piai.ToolCall{
						Type: "toolCall",
						ID:   id,
						Name: name,
					}
					stream.Push(piai.EventToolCallStart{
						Type: "toolcall_start",
						ID:   id,
						Name: name,
					})
				}
			}

		case "response.output_text.delta":
			delta, _ := event["delta"].(string)
			if delta != "" {
				textBuf.WriteString(delta)
				stream.Push(piai.EventTextDelta{
					Type:  "text_delta",
					Delta: delta,
				})
			}

		case "response.function_call_arguments.delta":
			callID, _ := event["call_id"].(string)
			delta, _ := event["delta"].(string)
			if tc, ok := toolCalls[callID]; ok && delta != "" {
				tc.Arguments = append(tc.Arguments, []byte(delta)...)
				stream.Push(piai.EventToolCallDelta{
					Type:           "toolcall_delta",
					ID:             tc.ID,
					ArgumentsDelta: delta,
				})
			}

		case "response.function_call_arguments.done":
			callID, _ := event["call_id"].(string)
			if tc, ok := toolCalls[callID]; ok {
				stream.Push(piai.EventToolCallEnd{
					Type:      "toolcall_end",
					ID:        tc.ID,
					Arguments: tc.Arguments,
				})
				msg.Content = append(msg.Content, *tc)
			}

		case "response.completed":
			response, _ := event["response"].(map[string]any)
			if response != nil {
				if usage, ok := response["usage"].(map[string]any); ok {
					msg.Usage.Input = int(getFloat(usage, "input_tokens"))
					msg.Usage.Output = int(getFloat(usage, "output_tokens"))
				}
				if status, ok := response["status"].(string); ok {
					msg.StopReason = mapResponseStatus(status)
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

			msg.Usage.TotalTokens = msg.Usage.Input + msg.Usage.Output
			msg.Usage.Cost = piai.CalculateCost(model, msg.Usage)

			stream.Push(piai.EventDone{
				Type:    "done",
				Message: msg,
			})

			return msg, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return msg, fmt.Errorf("openai-responses: SSE read error: %w", err)
	}

	return msg, nil
}

func mapResponseStatus(status string) piai.StopReason {
	switch status {
	case "completed":
		return piai.StopStop
	case "incomplete":
		return piai.StopLength
	case "failed":
		return piai.StopError
	case "cancelled":
		return piai.StopAborted
	default:
		return piai.StopStop
	}
}
