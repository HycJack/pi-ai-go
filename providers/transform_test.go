package providers

import (
	"testing"
	"time"

	piai "pi-ai-go"
)

func TestTransformMessagesVisionModel(t *testing.T) {
	model := piai.Model{
		ID:    "test-model",
		Input: []piai.Modality{piai.ModalityText, piai.ModalityImage},
	}

	messages := []piai.Message{
		piai.UserMessage{
			Role: "user",
			Content: []piai.ContentBlock{
				piai.TextContent{Type: "text", Text: "describe this"},
				piai.ImageContent{Type: "image", Data: "base64data", MimeType: "image/png"},
			},
			Timestamp: time.Now(),
		},
	}

	result := TransformMessages(messages, model)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}

	userMsg, ok := result[0].(piai.UserMessage)
	if !ok {
		t.Fatal("expected UserMessage")
	}

	blocks, ok := userMsg.Content.([]piai.ContentBlock)
	if !ok {
		t.Fatal("expected content blocks")
	}
	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(blocks))
	}
}

func TestTransformMessagesNonVisionModel(t *testing.T) {
	model := piai.Model{
		ID:    "test-model",
		Input: []piai.Modality{piai.ModalityText},
	}

	messages := []piai.Message{
		piai.UserMessage{
			Role: "user",
			Content: []piai.ContentBlock{
				piai.TextContent{Type: "text", Text: "describe this"},
				piai.ImageContent{Type: "image", Data: "base64data", MimeType: "image/png"},
			},
			Timestamp: time.Now(),
		},
	}

	result := TransformMessages(messages, model)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}

	userMsg, ok := result[0].(piai.UserMessage)
	if !ok {
		t.Fatal("expected UserMessage")
	}

	blocks, ok := userMsg.Content.([]piai.ContentBlock)
	if !ok {
		t.Fatal("expected content blocks")
	}
	// Image should be removed
	if len(blocks) != 1 {
		t.Errorf("expected 1 block (text only), got %d", len(blocks))
	}
}

func TestTransformMessagesStringContent(t *testing.T) {
	model := piai.Model{ID: "test", Input: []piai.Modality{piai.ModalityText}}

	messages := []piai.Message{
		piai.UserMessage{
			Role:      "user",
			Content:   "hello",
			Timestamp: time.Now(),
		},
	}

	result := TransformMessages(messages, model)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
}

func TestTransformAssistantMessageThinking(t *testing.T) {
	model := piai.Model{ID: "test-model"}

	messages := []piai.Message{
		piai.AssistantMessage{
			Role:  "assistant",
			Model: "test-model",
			Content: []piai.ContentBlock{
				piai.ThinkingContent{Type: "thinking", Thinking: "reasoning..."},
				piai.TextContent{Type: "text", Text: "answer"},
			},
			Timestamp: time.Now(),
		},
	}

	result := TransformMessages(messages, model)
	assistantMsg := result[0].(piai.AssistantMessage)

	// Thinking should be preserved for same model
	if len(assistantMsg.Content) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(assistantMsg.Content))
	}
}

func TestTransformAssistantMessageThinkingDifferentModel(t *testing.T) {
	model := piai.Model{ID: "other-model"}

	messages := []piai.Message{
		piai.AssistantMessage{
			Role:  "assistant",
			Model: "test-model",
			Content: []piai.ContentBlock{
				piai.ThinkingContent{Type: "thinking", Thinking: "reasoning..."},
				piai.TextContent{Type: "text", Text: "answer"},
			},
			Timestamp: time.Now(),
		},
	}

	result := TransformMessages(messages, model)
	assistantMsg := result[0].(piai.AssistantMessage)

	// Thinking should be converted to text for different model
	if len(assistantMsg.Content) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(assistantMsg.Content))
	}
	// First block should now be text
	if _, ok := assistantMsg.Content[0].(piai.TextContent); !ok {
		t.Error("expected first block to be TextContent")
	}
}

func TestTransformSkipsErroredMessages(t *testing.T) {
	model := piai.Model{ID: "test"}

	messages := []piai.Message{
		piai.AssistantMessage{
			Role:       "assistant",
			StopReason: piai.StopError,
			Content:    []piai.ContentBlock{piai.TextContent{Type: "text", Text: "error"}},
			Timestamp:  time.Now(),
		},
	}

	result := TransformMessages(messages, model)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}

	// Content should remain unchanged
	msg := result[0].(piai.AssistantMessage)
	if len(msg.Content) != 1 {
		t.Errorf("expected 1 content block, got %d", len(msg.Content))
	}
}

func TestNormalizeToolCallIDs(t *testing.T) {
	messages := []piai.Message{
		piai.AssistantMessage{
			Role: "assistant",
			Content: []piai.ContentBlock{
				piai.ToolCall{Type: "toolCall", ID: "call_abc123def456", Name: "test"},
			},
			Timestamp: time.Now(),
		},
		piai.ToolResultMessage{
			Role:       "toolResult",
			ToolCallID: "call_abc123def456",
			ToolName:   "test",
			Content:    []piai.ContentBlock{piai.TextContent{Type: "text", Text: "result"}},
			Timestamp:  time.Now(),
		},
	}

	result := NormalizeToolCallIDs(messages)

	// Check that tool call ID was normalized
	assistantMsg := result[0].(piai.AssistantMessage)
	tc := assistantMsg.Content[0].(piai.ToolCall)

	resultMsg := result[1].(piai.ToolResultMessage)

	if tc.ID != resultMsg.ToolCallID {
		t.Errorf("expected matching IDs: tool call %s, result %s", tc.ID, resultMsg.ToolCallID)
	}
}
