package openai

import (
	"testing"

	piai "pi-ai-go"
)

func TestNewCodex(t *testing.T) {
	p := NewCodex()
	if p == nil {
		t.Error("expected non-nil provider")
	}
}

func TestCodexProviderImplementsInterface(t *testing.T) {
	var _ = NewCodex()
}

func TestCodexStreamNoAPIKey(t *testing.T) {
	p := NewCodex()
	model := piai.Model{
		ID:       "codex-mini",
		Provider: piai.ProviderOpenAICodex,
	}

	_, err := p.Stream(model, piai.Context{}, piai.StreamOptions{})
	if err == nil {
		t.Error("expected error for missing API key")
	}
}
