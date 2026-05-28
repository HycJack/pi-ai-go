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

// CompletionsOptions holds OpenAI Completions-specific options.
type CompletionsOptions struct {
	ToolChoice      any    `json:"toolChoice,omitempty"`
	ReasoningEffort string `json:"reasoningEffort,omitempty"`
}

// CompletionsProvider implements the OpenAI Chat Completions API.
type CompletionsProvider struct{}

// NewCompletions creates a new OpenAI Completions provider.
func NewCompletions() *CompletionsProvider {
	return &CompletionsProvider{}
}

func (p *CompletionsProvider) Stream(model piai.Model, ctx piai.Context, opts piai.StreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	return streamCompletions(model, ctx, opts, CompletionsOptions{})
}

func (p *CompletionsProvider) StreamSimple(model piai.Model, ctx piai.Context, opts piai.SimpleStreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	completionsOpts := CompletionsOptions{}
	if opts.Reasoning != "" {
		completionsOpts.ReasoningEffort = string(clampReasoning(opts.Reasoning))
	}
	return streamCompletions(model, ctx, opts.StreamOptions, completionsOpts)
}

func streamCompletions(model piai.Model, c piai.Context, opts piai.StreamOptions, completionsOpts CompletionsOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	apiKey := piai.ResolveAPIKey(model.Provider, opts.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("openai: no API key provided")
	}

	baseURL := piai.ResolveBaseURL(model, defaultCompletionsURL)

	body, err := buildCompletionsBody(model, c, opts, completionsOpts)
	if err != nil {
		return nil, fmt.Errorf("openai: failed to build request: %w", err)
	}

	if opts.OnPayload != nil {
		opts.OnPayload(body)
	}

	stream := piai.NewEventStream[piai.AssistantMessageEvent, piai.AssistantMessage]()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				stream.Error(fmt.Errorf("openai: panic: %v", r))
			}
		}()

		msg, err := doCompletionsStream(baseURL, apiKey, model, body, stream, opts)
		if err != nil {
			stream.Error(err)
			return
		}
		stream.End(msg)
	}()

	return stream, nil
}

func buildCompletionsBody(model piai.Model, c piai.Context, opts piai.StreamOptions, completionsOpts CompletionsOptions) (map[string]any, error) {
	body := map[string]any{
		"model":  model.ID,
		"stream": true,
	}

	if opts.MaxTokens != nil && *opts.MaxTokens > 0 {
		body["max_tokens"] = *opts.MaxTokens
	} else if model.MaxTokens > 0 {
		body["max_tokens"] = model.MaxTokens
	}

	if opts.Temperature != nil {
		body["temperature"] = *opts.Temperature
	}

	// Build messages
	messages := []map[string]any{}

	if c.SystemPrompt != "" {
		messages = append(messages, map[string]any{
			"role":    "system",
			"content": c.SystemPrompt,
		})
	}

	msgs, err := ConvertMessages(c.Messages, model)
	if err != nil {
		return nil, err
	}
	messages = append(messages, msgs...)
	body["messages"] = messages

	// Tools
	if len(c.Tools) > 0 {
		body["tools"] = ConvertTools(c.Tools)
	}

	// Tool choice
	if completionsOpts.ToolChoice != nil {
		body["tool_choice"] = completionsOpts.ToolChoice
	}

	// Reasoning effort
	if completionsOpts.ReasoningEffort != "" {
		body["reasoning_effort"] = completionsOpts.ReasoningEffort
	}

	return body, nil
}

func doCompletionsStream(baseURL, apiKey string, model piai.Model, body map[string]any, stream *piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], opts piai.StreamOptions) (piai.AssistantMessage, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return piai.AssistantMessage{}, err
	}

	url := baseURL + "/chat/completions"

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
		return piai.AssistantMessage{}, fmt.Errorf("openai: API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return processCompletionsSSE(resp.Body, stream, model, opts)
}

func processCompletionsSSE(body io.Reader, stream *piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], model piai.Model, opts piai.StreamOptions) (piai.AssistantMessage, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var (
		msg         piai.AssistantMessage
		textBuf     strings.Builder
		toolCalls   map[int]*piai.ToolCall
		toolIndices []int
	)

	msg.API = model.API
	msg.Provider = model.Provider
	msg.Model = model.ID
	msg.Role = "assistant"
	msg.Timestamp = time.Now()
	toolCalls = make(map[int]*piai.ToolCall)

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

		var chunk map[string]any
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		choices, ok := chunk["choices"].([]any)
		if !ok || len(choices) == 0 {
			continue
		}

		choice, ok := choices[0].(map[string]any)
		if !ok {
			continue
		}

		// Handle finish reason
		if finishReason, ok := choice["finish_reason"].(string); ok && finishReason != "" {
			msg.StopReason = MapStopReason(finishReason)
		}

		// Handle usage
		if usage, ok := chunk["usage"].(map[string]any); ok {
			msg.Usage.Input = int(getFloat(usage, "prompt_tokens"))
			msg.Usage.Output = int(getFloat(usage, "completion_tokens"))
			msg.Usage.CacheRead = int(getFloat(usage, "prompt_tokens_details.cache_read_input_tokens"))
			msg.Usage.TotalTokens = int(getFloat(usage, "total_tokens"))
		}

		delta, ok := choice["delta"].(map[string]any)
		if !ok {
			continue
		}

		// Text content
		if content, ok := delta["content"].(string); ok && content != "" {
			textBuf.WriteString(content)
			stream.Push(piai.EventTextDelta{
				Type:  "text_delta",
				Delta: content,
			})
		}

		// Reasoning content
		if reasoning, ok := delta["reasoning_content"].(string); ok && reasoning != "" {
			stream.Push(piai.EventThinkingDelta{
				Type:  "thinking_delta",
				Delta: reasoning,
			})
		}

		// Tool calls
		if calls, ok := delta["tool_calls"].([]any); ok {
			for _, call := range calls {
				c, ok := call.(map[string]any)
				if !ok {
					continue
				}
				index := int(getFloat(c, "index"))
				id, _ := c["id"].(string)
				function, _ := c["function"].(map[string]any)
				name, _ := function["name"].(string)
				args, _ := function["arguments"].(string)

				if id != "" {
					// New tool call
					toolCalls[index] = &piai.ToolCall{
						Type: "toolCall",
						ID:   id,
						Name: name,
					}
					toolIndices = append(toolIndices, index)
					stream.Push(piai.EventToolCallStart{
						Type: "toolcall_start",
						ID:   id,
						Name: name,
					})
				}

				if tc, ok := toolCalls[index]; ok && args != "" {
					tc.Arguments = append(tc.Arguments, []byte(args)...)
					stream.Push(piai.EventToolCallDelta{
						Type:           "toolcall_delta",
						ID:             tc.ID,
						ArgumentsDelta: args,
					})
				}
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

	for _, index := range toolIndices {
		if tc, ok := toolCalls[index]; ok {
			stream.Push(piai.EventToolCallEnd{
				Type:      "toolcall_end",
				ID:        tc.ID,
				Arguments: tc.Arguments,
			})
			msg.Content = append(msg.Content, *tc)
		}
	}

	msg.Usage.TotalTokens = msg.Usage.Input + msg.Usage.Output
	msg.Usage.Cost = piai.CalculateCost(model, msg.Usage)

	stream.Push(piai.EventDone{
		Type:    "done",
		Message: msg,
	})

	return msg, nil
}

func getFloat(m map[string]any, key string) float64 {
	// Support nested keys like "prompt_tokens_details.cache_read_input_tokens"
	keys := strings.Split(key, ".")
	current := m
	for i, k := range keys {
		if i == len(keys)-1 {
			if v, ok := current[k]; ok {
				if f, ok := v.(float64); ok {
					return f
				}
			}
		} else {
			if next, ok := current[k].(map[string]any); ok {
				current = next
			} else {
				return 0
			}
		}
	}
	return 0
}

func clampReasoning(effort piai.ThinkingLevel) piai.ThinkingLevel {
	if effort == piai.ThinkingXHigh {
		return piai.ThinkingHigh
	}
	return effort
}
