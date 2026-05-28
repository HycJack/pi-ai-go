package providers

import (
	"encoding/json"

	piai "pi-ai-go"
)

// TransformMessages preprocesses messages for cross-provider compatibility.
func TransformMessages(messages []piai.Message, model piai.Model) []piai.Message {
	var result []piai.Message

	for _, msg := range messages {
		switch m := msg.(type) {
		case piai.UserMessage:
			result = append(result, transformUserMessage(m, model))
		case piai.AssistantMessage:
			result = append(result, transformAssistantMessage(m, model))
		case piai.ToolResultMessage:
			result = append(result, transformToolResultMessage(m, model))
		default:
			result = append(result, msg)
		}
	}

	return result
}

func transformUserMessage(msg piai.UserMessage, model piai.Model) piai.UserMessage {
	// If content is a string, no transformation needed
	if _, ok := msg.Content.(string); ok {
		return msg
	}

	// If content is a slice, filter images for non-vision models
	blocks, ok := msg.Content.([]piai.ContentBlock)
	if !ok {
		return msg
	}

	hasVision := false
	for _, modality := range model.Input {
		if modality == piai.ModalityImage {
			hasVision = true
			break
		}
	}

	if hasVision {
		return msg
	}

	// Filter out image content for non-vision models
	var filtered []piai.ContentBlock
	for _, block := range blocks {
		if _, isImage := block.(piai.ImageContent); isImage {
			continue
		}
		filtered = append(filtered, block)
	}

	// If all content was images, convert to text placeholder
	if len(filtered) == 0 {
		return piai.UserMessage{
			Role:      "user",
			Content:   "[image content removed]",
			Timestamp: msg.Timestamp,
		}
	}

	msg.Content = filtered
	return msg
}

func transformAssistantMessage(msg piai.AssistantMessage, model piai.Model) piai.AssistantMessage {
	// Skip errored/aborted messages
	if msg.StopReason == piai.StopError || msg.StopReason == piai.StopAborted {
		return msg
	}

	var transformed []piai.ContentBlock
	for _, block := range msg.Content {
		switch b := block.(type) {
		case piai.ThinkingContent:
			// Keep thinking blocks for same model
			if msg.Model == model.ID {
				transformed = append(transformed, block)
			} else if !b.Redacted {
				// Convert non-redacted thinking to text for different models
				transformed = append(transformed, piai.TextContent{
					Type: "text",
					Text: b.Thinking,
				})
			}
			// Drop redacted thinking for different models
		case piai.ToolCall:
			transformed = append(transformed, block)
		default:
			transformed = append(transformed, block)
		}
	}

	msg.Content = transformed
	return msg
}

func transformToolResultMessage(msg piai.ToolResultMessage, model piai.Model) piai.ToolResultMessage {
	// Check if model supports images
	hasVision := false
	for _, modality := range model.Input {
		if modality == piai.ModalityImage {
			hasVision = true
			break
		}
	}

	if hasVision {
		return msg
	}

	// Filter out image content
	var filtered []piai.ContentBlock
	for _, block := range msg.Content {
		if _, isImage := block.(piai.ImageContent); isImage {
			continue
		}
		filtered = append(filtered, block)
	}

	if len(filtered) == 0 {
		filtered = []piai.ContentBlock{piai.TextContent{
			Type: "text",
			Text: "[image content removed]",
		}}
	}

	msg.Content = filtered
	return msg
}

// NormalizeToolCallIDs normalizes tool call IDs for cross-provider compatibility.
func NormalizeToolCallIDs(messages []piai.Message) []piai.Message {
	idMap := make(map[string]string)
	counter := 0

	for i, msg := range messages {
		switch m := msg.(type) {
		case piai.AssistantMessage:
			for j, block := range m.Content {
				if tc, ok := block.(piai.ToolCall); ok {
					newID := generateShortID(counter)
					idMap[tc.ID] = newID
					tc.ID = newID
					m.Content[j] = tc
					counter++
				}
			}
			messages[i] = m
		case piai.ToolResultMessage:
			if newID, ok := idMap[m.ToolCallID]; ok {
				m.ToolCallID = newID
				messages[i] = m
			}
		}
	}

	return messages
}

func generateShortID(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	if n < 26 {
		return string(chars[n])
	}
	return generateShortID(n/26-1) + string(chars[n%26])
}

// MarshalToolArguments marshals tool arguments to JSON, handling nil.
func MarshalToolArguments(args map[string]any) json.RawMessage {
	if args == nil {
		return json.RawMessage("{}")
	}
	b, _ := json.Marshal(args)
	return b
}
