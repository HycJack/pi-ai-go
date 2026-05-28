// Package google implements Google Generative AI (Gemini) API providers.
package google

import (
	"encoding/json"
	"strings"

	piai "pi-ai-go"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com"

// ConvertMessages converts internal messages to Google Gemini format.
func ConvertMessages(messages []piai.Message) ([]map[string]any, error) {
	var result []map[string]any

	for _, msg := range messages {
		switch m := msg.(type) {
		case piai.UserMessage:
			parts, err := convertUserParts(m.Content)
			if err != nil {
				return nil, err
			}
			result = append(result, map[string]any{
				"role":  "user",
				"parts": parts,
			})

		case piai.AssistantMessage:
			parts := convertAssistantParts(m.Content)
			result = append(result, map[string]any{
				"role":  "model",
				"parts": parts,
			})

		case piai.ToolResultMessage:
			parts := convertToolResultParts(m.Content)
			result = append(result, map[string]any{
				"role":  "user",
				"parts": []any{
					map[string]any{
						"functionResponse": map[string]any{
							"name":     m.ToolName,
							"response": parts,
						},
					},
				},
			})
		}
	}

	return result, nil
}

func convertUserParts(content any) ([]any, error) {
	switch c := content.(type) {
	case string:
		return []any{
			map[string]any{"text": c},
		}, nil
	case []piai.ContentBlock:
		var parts []any
		for _, block := range c {
			switch b := block.(type) {
			case piai.TextContent:
				parts = append(parts, map[string]any{"text": b.Text})
			case piai.ImageContent:
				parts = append(parts, map[string]any{
					"inlineData": map[string]any{
						"mimeType": b.MimeType,
						"data":     b.Data,
					},
				})
			}
		}
		return parts, nil
	default:
		return []any{
			map[string]any{"text": strings.TrimSpace(stringify(c))},
		}, nil
	}
}

func convertAssistantParts(content []piai.ContentBlock) []any {
	var parts []any
	for _, block := range content {
		switch b := block.(type) {
		case piai.TextContent:
			parts = append(parts, map[string]any{"text": b.Text})
		case piai.ThinkingContent:
			parts = append(parts, map[string]any{
				"thought": true,
				"text":    b.Thinking,
			})
		case piai.ToolCall:
			parts = append(parts, map[string]any{
				"functionCall": map[string]any{
					"name": b.Name,
					"args": json.RawMessage(b.Arguments),
				},
			})
		}
	}
	return parts
}

func convertToolResultParts(content []piai.ContentBlock) map[string]any {
	result := make(map[string]any)
	for _, block := range content {
		if text, ok := block.(piai.TextContent); ok {
			// Try to parse as JSON
			var v any
			if err := json.Unmarshal([]byte(text.Text), &v); err == nil {
				return v.(map[string]any)
			}
			result["text"] = text.Text
		}
	}
	return result
}

// ConvertTools converts tools to Google Gemini format.
func ConvertTools(tools []piai.Tool) []map[string]any {
	declarations := make([]map[string]any, len(tools))
	for i, tool := range tools {
		d := map[string]any{
			"name": tool.Name,
		}
		if tool.Description != "" {
			d["description"] = tool.Description
		}
		if len(tool.Parameters) > 0 {
			var params map[string]any
			if err := json.Unmarshal(tool.Parameters, &params); err == nil {
				d["parameters"] = params
			}
		}
		declarations[i] = d
	}
	return []map[string]any{
		{
			"functionDeclarations": declarations,
		},
	}
}

// MapStopReason maps Google finish reasons to StopReason.
func MapStopReason(reason string) piai.StopReason {
	switch reason {
	case "STOP":
		return piai.StopStop
	case "MAX_TOKENS":
		return piai.StopLength
	case "SAFETY":
		return piai.StopError
	case "RECITATION":
		return piai.StopError
	case "OTHER":
		return piai.StopError
	default:
		return piai.StopStop
	}
}

// IsThinkingPart checks if a part is a thinking/reasoning part.
func IsThinkingPart(part map[string]any) bool {
	if thought, ok := part["thought"].(bool); ok {
		return thought
	}
	return false
}

func stringify(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}
