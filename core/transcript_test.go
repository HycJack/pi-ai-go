package core

import "testing"

func TestNormalizeContextValidation(t *testing.T) {
	_, err := NormalizeContext(Context{
		Messages: []Message{UserMessage{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("valid context rejected: %v", err)
	}

	_, err = NormalizeContext(Context{
		Messages: []Message{UserMessage{Role: "bogus_role", Content: "nope"}},
	})
	if err == nil {
		t.Fatal("invalid role should be rejected")
	}
}

func TestNormalizeContextEmptyContent(t *testing.T) {
	got, err := NormalizeContext(Context{
		Messages: []Message{UserMessage{Role: "user", Content: nil}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	um, ok := got.Messages[0].(UserMessage)
	if !ok {
		t.Fatalf("expected UserMessage, got %T", got.Messages[0])
	}
	if s, ok := um.Content.(string); !ok || s != "" {
		t.Errorf("expected empty string content, got %#v", um.Content)
	}
}

func TestNormalizeContextDeepCopy(t *testing.T) {
	content := []ContentBlock{TextContent{Type: "text", Text: "hello"}}
	original := []Message{
		UserMessage{Role: "user", Content: content},
	}
	got, err := NormalizeContext(Context{Messages: original})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Mutate the caller's content slice after normalization. TextContent is a
	// value struct, so reassigning the element is the observable mutation; it
	// must not leak into the normalized copy.
	content[0] = TextContent{Type: "text", Text: "mutated"}

	uc, ok := got.Messages[0].(UserMessage)
	if !ok {
		t.Fatalf("expected UserMessage, got %T", got.Messages[0])
	}
	blocks := uc.Content.([]ContentBlock)
	if len(blocks) == 0 {
		t.Fatal("expected non-empty content in normalized copy")
	}
	if text := blocks[0].(TextContent).Text; text != "hello" {
		t.Errorf("normalized context should be isolated from caller mutation, got %q", text)
	}
}

func TestNormalizeContextKeepsSystemPrompt(t *testing.T) {
	got, err := NormalizeContext(Context{
		SystemPrompt: "you are helpful",
		Messages:     []Message{UserMessage{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SystemPrompt != "you are helpful" {
		t.Errorf("SystemPrompt must be preserved (not folded, provider injects it), got %q", got.SystemPrompt)
	}
}

func TestNormalizeContextNilMessages(t *testing.T) {
	got, err := NormalizeContext(Context{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Messages) != 0 {
		t.Errorf("expected empty messages, got %d", len(got.Messages))
	}
}

func TestNormalizeContextCopiesToolsSlice(t *testing.T) {
	tools := []Tool{{Name: "read", Description: "read a file"}}
	got, err := NormalizeContext(Context{Tools: tools})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tools[0].Name = "mutated"
	if got.Tools[0].Name != "read" {
		t.Errorf("tools should be deep-copied, got %q", got.Tools[0].Name)
	}
}