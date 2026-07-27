package main

import (
	"encoding/json"
	"strings"

	"pi-ai-go/core"
)

// ─── 消息历史构建 ───

// buildMessages extracts message history from params (StreamMessage request)
// and appends the current user message.
func buildMessages(params map[string]interface{}, currentMessage string) []core.Message {
	raw, _ := params["messages"].([]interface{})
	msgs := parseMessageHistory(raw)
	msgs = append(msgs, buildCurrentUserMessage(currentMessage, extractImagesFromParams(params)))
	return msgs
}

// buildAgentMessages builds message history from an AgentRequest.
func (a *App) buildAgentMessages(req AgentRequest) []core.Message {
	msgs := parseMessageHistory(toInterfaceSlice(req.Messages))
	msgs = append(msgs, buildCurrentUserMessage(req.Message, req.Images))
	return msgs
}

// extractImageBlocks converts an "images" array from any of the shapes Wails
// may deserialize into a normalized []core.ContentBlock of image content.
func extractImageBlocks(raw interface{}) []core.ContentBlock {
	if raw == nil {
		return nil
	}
	var blocks []core.ContentBlock
	switch items := raw.(type) {
	case []interface{}:
		for _, item := range items {
			if img, ok := item.(map[string]interface{}); ok {
				if b := buildImageBlock(img); b != nil {
					blocks = append(blocks, *b)
				}
			}
		}
	case []map[string]interface{}:
		for _, img := range items {
			if b := buildImageBlock(img); b != nil {
				blocks = append(blocks, *b)
			}
		}
	}
	return blocks
}

// buildImageBlock converts a single image map to an ImageContent block.
func buildImageBlock(img map[string]interface{}) *core.ImageContent {
	data, _ := img["data"].(string)
	if data == "" {
		return nil
	}
	mime, _ := img["mimeType"].(string)
	if mime == "" {
		mime = "image/png"
	}
	if idx := strings.Index(data, "base64,"); idx >= 0 {
		data = data[idx+len("base64,"):]
	}
	return &core.ImageContent{Type: "image", Data: data, MimeType: mime}
}

// extractImagesFromParams pulls the "images" array from StreamMessage params.
func extractImagesFromParams(params map[string]interface{}) []ImageInput {
	blocks := extractImageBlocks(params["images"])
	if len(blocks) == 0 {
		return nil
	}
	out := make([]ImageInput, 0, len(blocks))
	for _, b := range blocks {
		img, ok := b.(core.ImageContent)
		if !ok {
			continue
		}
		out = append(out, ImageInput{Data: img.Data, MimeType: img.MimeType})
	}
	return out
}

// buildCurrentUserMessage builds a UserMessage that may contain text + image blocks.
func buildCurrentUserMessage(text string, images []ImageInput) core.UserMessage {
	if len(images) == 0 {
		return core.UserMessage{Content: text}
	}
	var blocks []core.ContentBlock
	if text != "" {
		blocks = append(blocks, core.TextContent{Type: "text", Text: text})
	}
	for _, img := range images {
		mime := img.MimeType
		if mime == "" {
			mime = "image/png"
		}
		data := img.Data
		if idx := strings.Index(data, "base64,"); idx >= 0 {
			data = data[idx+len("base64,"):]
		}
		blocks = append(blocks, core.ImageContent{
			Type:     "image",
			Data:     data,
			MimeType: mime,
		})
	}
	return core.UserMessage{Content: blocks}
}

// toInterfaceSlice converts []map[string]interface{} to []interface{}.
func toInterfaceSlice(src []map[string]interface{}) []interface{} {
	result := make([]interface{}, len(src))
	for i, m := range src {
		result[i] = m
	}
	return result
}

// parseMessageHistory converts raw frontend message objects to core.Message.
func parseMessageHistory(raw []interface{}) []core.Message {
	if len(raw) == 0 {
		return nil
	}
	var msgs []core.Message
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		if role == "" {
			continue
		}
		content, _ := m["content"].(string)
		imageBlocks := extractImageBlocks(m["images"])
		if role == "user" {
			if len(imageBlocks) > 0 {
				var blocks []core.ContentBlock
				if content != "" {
					blocks = append(blocks, core.TextContent{Type: "text", Text: content})
				}
				blocks = append(blocks, imageBlocks...)
				msgs = append(msgs, core.UserMessage{Content: blocks})
			} else {
				msgs = append(msgs, core.UserMessage{Content: content})
			}
		} else if role == "assistant" {
			msg := core.AssistantMessage{Role: "assistant"}
			if rawTCs, ok := m["tool_calls"].([]interface{}); ok && len(rawTCs) > 0 {
				var toolCalls []core.ContentBlock
				for _, rawTC := range rawTCs {
					if tc, ok := rawTC.(map[string]interface{}); ok {
						tcID, _ := tc["id"].(string)
						tcName, _ := tc["name"].(string)
						tcArgs, _ := tc["arguments"].(string)
						toolCalls = append(toolCalls, core.ToolCall{
							Type:      "toolCall",
							ID:        tcID,
							Name:      tcName,
							Arguments: json.RawMessage(tcArgs),
						})
					}
				}
				if content != "" {
					msg.Content = append([]core.ContentBlock{core.TextContent{Type: "text", Text: content}}, toolCalls...)
				} else {
					msg.Content = toolCalls
				}
			} else if content != "" {
				msg.Content = []core.ContentBlock{core.TextContent{Type: "text", Text: content}}
			}
			msgs = append(msgs, msg)
		} else if role == "tool" {
			toolCallID, _ := m["tool_call_id"].(string)
			toolName, _ := m["tool_name"].(string)
			if toolCallID == "" {
				toolCallID, _ = m["toolCallID"].(string)
			}
			msgs = append(msgs, core.ToolResultMessage{
				Role:       "tool",
				ToolCallID: toolCallID,
				ToolName:   toolName,
				Content:    []core.ContentBlock{core.TextContent{Type: "text", Text: content}},
			})
		}
	}
	return msgs
}
