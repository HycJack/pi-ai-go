package providers

import (
	"testing"

	piai "pi-ai-go"
)

func TestProviderInterface(t *testing.T) {
	// Verify that a mock implementation satisfies the Provider interface
	var _ Provider = &mockProvider{}
}

type mockProvider struct{}

func (m *mockProvider) Stream(model piai.Model, context piai.Context, opts piai.StreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	return piai.NewEventStream[piai.AssistantMessageEvent, piai.AssistantMessage](), nil
}

func (m *mockProvider) StreamSimple(model piai.Model, context piai.Context, opts piai.SimpleStreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	return piai.NewEventStream[piai.AssistantMessageEvent, piai.AssistantMessage](), nil
}
