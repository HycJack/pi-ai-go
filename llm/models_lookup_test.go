package llm

import (
	"testing"

	"github.com/HycJack/pi-ai-go/core"
)

func lookupSetup() {
	LoadModels(map[core.KnownProvider]map[string]core.Model{
		core.ProviderAnthropic: {
			"claude-3-opus": {ID: "claude-3-opus", Provider: core.ProviderAnthropic, API: core.APIAnthropicMessages},
		},
		core.ProviderOpenAI: {
			"gpt-4": {ID: "gpt-4", Provider: core.ProviderOpenAI, API: core.APIOpenAICompletions},
		},
	})
}

func TestAI_LookupModelExact(t *testing.T) {
	lookupSetup()
	m, err := LookupModelExact("anthropic/claude-3-opus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.ID != "claude-3-opus" || m.Provider != core.ProviderAnthropic {
		t.Errorf("unexpected model: %s/%s", m.Provider, m.ID)
	}
}

func TestAI_LookupModelExactMalformed(t *testing.T) {
	lookupSetup()
	_, err := LookupModelExact("claude-3-opus") // missing provider/id form
	if err == nil {
		t.Error("malformed ref (no slash) should error")
	}
	_, err = LookupModelExact("anthropic/")
	if err == nil {
		t.Error("empty model id should error")
	}
	_, err = LookupModelExact("/claude-3-opus")
	if err == nil {
		t.Error("empty provider should error")
	}
}

func TestAI_LookupModelExactNotFound(t *testing.T) {
	lookupSetup()
	if _, err := LookupModelExact("anthropic/nope"); err == nil {
		t.Error("unknown model should error")
	}
	if _, err := LookupModelExact("fireworks/x"); err == nil {
		t.Error("unknown provider should error")
	}
}

func TestAI_LookupModelBareID(t *testing.T) {
	lookupSetup()
	m, err := LookupModel("claude-3-opus")
	if err != nil {
		t.Fatalf("bare id should resolve across providers, got error: %v", err)
	}
	if m.Provider != core.ProviderAnthropic {
		t.Errorf("expected anthropic, got %s", m.Provider)
	}
}

func TestAI_LookupModelPrefixed(t *testing.T) {
	lookupSetup()
	m, err := LookupModel("openai/gpt-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.ID != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", m.ID)
	}
}

func TestAI_LookupModelBareIDNotFound(t *testing.T) {
	lookupSetup()
	if _, err := LookupModel("does-not-exist"); err == nil {
		t.Error("unknown bare id should error")
	}
}

func TestAI_ToCapabilities(t *testing.T) {
	m := core.Model{
		Reasoning: true,
		Input:    []core.Modality{core.ModalityText, core.ModalityImage},
	}
	caps := ToCapabilities(m)
	if !caps.Text || !caps.Vision {
		t.Errorf("expected text+vision, got %+v", caps)
	}
	if caps.Audio {
		t.Error("audio should be false")
	}
	if !caps.Reasoning {
		t.Error("reasoning should be true")
	}

	audio := core.Model{Input: []core.Modality{core.ModalityAudio}}
	if !ToCapabilities(audio).Audio {
		t.Error("audio should be true")
	}
}