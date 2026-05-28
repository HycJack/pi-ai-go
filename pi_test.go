package piai

import (
	"context"
	"testing"
)

func TestStreamNoProvider(t *testing.T) {
	ClearProviders()
	defer func() {
		ClearProviders()
	}()

	model := Model{
		ID:      "test",
		API:     APIAnthropicMessages,
		Provider: ProviderAnthropic,
	}

	_, err := Stream(context.Background(), model, []Message{
		UserMessage{Role: "user", Content: "hello"},
	})
	if err == nil {
		t.Error("expected error for unregistered provider")
	}
}

func TestCompleteNoProvider(t *testing.T) {
	ClearProviders()
	defer func() {
		ClearProviders()
	}()

	model := Model{
		ID:      "test",
		API:     APIAnthropicMessages,
		Provider: ProviderAnthropic,
	}

	_, err := Complete(context.Background(), model, []Message{
		UserMessage{Role: "user", Content: "hello"},
	})
	if err == nil {
		t.Error("expected error for unregistered provider")
	}
}

func TestStreamSimpleNoProvider(t *testing.T) {
	ClearProviders()
	defer func() {
		ClearProviders()
	}()

	model := Model{
		ID:      "test",
		API:     APIAnthropicMessages,
		Provider: ProviderAnthropic,
	}

	_, err := StreamSimple(context.Background(), model, []Message{
		UserMessage{Role: "user", Content: "hello"},
	})
	if err == nil {
		t.Error("expected error for unregistered provider")
	}
}

func TestCompleteSimpleNoProvider(t *testing.T) {
	ClearProviders()
	defer func() {
		ClearProviders()
	}()

	model := Model{
		ID:      "test",
		API:     APIAnthropicMessages,
		Provider: ProviderAnthropic,
	}

	_, err := CompleteSimple(context.Background(), model, []Message{
		UserMessage{Role: "user", Content: "hello"},
	})
	if err == nil {
		t.Error("expected error for unregistered provider")
	}
}
