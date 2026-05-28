// Package openai implements OpenAI-compatible API providers.
package openai

import (
	"encoding/json"

	piai "pi-ai-go"
)

const defaultCompletionsURL = "https://api.openai.com/v1"
const defaultResponsesURL = "https://api.openai.com/v1"

// ConvertMessages converts internal messages to OpenAI Chat Completions format.
func ConvertMessages(messages []piai.Message, model piai.Model) ([]map[string]any, error) {
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
			openaiMsg := map[string]any{
				"role": "assistant",
			}
			content := convertAssistantContent(m.Content)
			if len(content) == 1 {
				if textBlock, ok := content[0].(map[string]any); ok && textBlock["type"] == "text" {
					openaiMsg["content"] = textBlock["text"]
				} else {
					openaiMsg["content"] = content
				}
			} else {
				openaiMsg["content"] = content
			}
			result = append(result, openaiMsg)

		case piai.ToolResultMessage:
			result = append(result, map[string]any{
				"role":       "tool",
				"tool_call_id": m.ToolCallID,
				"content":    convertToolResultContent(m.Content),
			})
		}
	}

	return result, nil
}

func convertUserContent(content any) (any, error) {
	switch c := content.(type) {
	case string:
		return c, nil
	case []piai.ContentBlock:
		var blocks []any
		for _, block := range c {
			switch b := block.(type) {
			case piai.TextContent:
				blocks = append(blocks, map[string]any{
					"type": "text",
					"text": b.Text,
				})
			case piai.ImageContent:
				blocks = append(blocks, map[string]any{
					"type": "image_url",
					"image_url": map[string]any{
						"url": "data:" + b.MimeType + ";base64," + b.Data,
					},
				})
			}
		}
		return blocks, nil
	default:
		return c, nil
	}
}

func convertAssistantContent(content []piai.ContentBlock) []any {
	var blocks []any
	var toolCalls []any

	for _, block := range content {
		switch b := block.(type) {
		case piai.TextContent:
			blocks = append(blocks, map[string]any{
				"type": "text",
				"text": b.Text,
			})
		case piai.ThinkingContent:
			// OpenAI uses reasoning_content or similar
			blocks = append(blocks, map[string]any{
				"type": "reasoning_content",
				"reasoning_content": map[string]any{
					"text": b.Thinking,
				},
			})
		case piai.ToolCall:
			toolCalls = append(toolCalls, map[string]any{
				"id":   b.ID,
				"type": "function",
				"function": map[string]any{
					"name":      b.Name,
					"arguments": string(b.Arguments),
				},
			})
		}
	}

	if len(toolCalls) > 0 {
		blocks = append(blocks, toolCalls...)
	}

	return blocks
}

func convertToolResultContent(content []piai.ContentBlock) string {
	var parts []string
	for _, block := range content {
		if text, ok := block.(piai.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return joinStrings(parts, "\n")
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}

// ConvertTools converts tools to OpenAI format.
func ConvertTools(tools []piai.Tool) []map[string]any {
	result := make([]map[string]any, len(tools))
	for i, tool := range tools {
		t := map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
			},
		}
		if len(tool.Parameters) > 0 {
			var params map[string]any
			json.Unmarshal(tool.Parameters, &params)
			t["function"].(map[string]any)["parameters"] = params
		}
		result[i] = t
	}
	return result
}

// MapStopReason maps OpenAI finish reasons to StopReason.
func MapStopReason(reason string) piai.StopReason {
	switch reason {
	case "stop":
		return piai.StopStop
	case "length":
		return piai.StopLength
	case "tool_calls":
		return piai.StopToolUse
	case "function_call":
		return piai.StopToolUse
	default:
		return piai.StopStop
	}
}
