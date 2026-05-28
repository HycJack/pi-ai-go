package piai

import (
	"testing"
)

type mockProvider struct{}

func (m *mockProvider) Stream(model Model, context Context, opts StreamOptions) (*EventStream[AssistantMessageEvent, AssistantMessage], error) {
	return NewEventStream[AssistantMessageEvent, AssistantMessage](), nil
}

func (m *mockProvider) StreamSimple(model Model, context Context, opts SimpleStreamOptions) (*EventStream[AssistantMessageEvent, AssistantMessage], error) {
	return NewEventStream[AssistantMessageEvent, AssistantMessage](), nil
}

func TestRegisterAndGetProvider(t *testing.T) {
	ClearProviders()
	defer ClearProviders()

	p := &mockProvider{}
	RegisterProvider(APIAnthropicMessages, p, "test")

	got, err := GetProvider(APIAnthropicMessages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != p {
		t.Error("expected same provider")
	}
}

func TestGetProviderNotFound(t *testing.T) {
	ClearProviders()
	defer ClearProviders()

	_, err := GetProvider(APIAnthropicMessages)
	if err == nil {
		t.Error("expected error for unregistered provider")
	}
}

func TestGetRegisteredProviders(t *testing.T) {
	ClearProviders()
	defer ClearProviders()

	RegisterProvider(APIAnthropicMessages, &mockProvider{}, "test")
	RegisterProvider(APIOpenAICompletions, &mockProvider{}, "test")

	providers := GetRegisteredProviders()
	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}
}

func TestUnregisterProviders(t *testing.T) {
	ClearProviders()
	defer ClearProviders()

	RegisterProvider(APIAnthropicMessages, &mockProvider{}, "src1")
	RegisterProvider(APIOpenAICompletions, &mockProvider{}, "src1")
	RegisterProvider(APIBedrockConverse, &mockProvider{}, "src2")

	UnregisterProviders("src1")

	providers := GetRegisteredProviders()
	if len(providers) != 1 {
		t.Errorf("expected 1 provider after unregister, got %d", len(providers))
	}

	_, err := GetProvider(APIBedrockConverse)
	if err != nil {
		t.Error("expected bedrock to still be registered")
	}
}

func TestClearProviders(t *testing.T) {
	RegisterProvider(APIAnthropicMessages, &mockProvider{}, "test")
	ClearProviders()

	providers := GetRegisteredProviders()
	if len(providers) != 0 {
		t.Errorf("expected 0 providers after clear, got %d", len(providers))
	}
}
