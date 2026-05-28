package piai

import (
	"testing"
)

func setupTestModels() {
	LoadModels(map[KnownProvider]map[string]Model{
		ProviderAnthropic: {
			"claude-3-opus": {
				ID:        "claude-3-opus",
				Name:      "Claude 3 Opus",
				API:       APIAnthropicMessages,
				Provider:  ProviderAnthropic,
				Reasoning: true,
				Input:     []Modality{ModalityText, ModalityImage},
				Cost:      Cost{Input: 15, Output: 75},
				ContextWindow: 200000,
				MaxTokens:     4096,
				ThinkingLevelMap: map[string]string{
					"low":    "low",
					"medium": "medium",
					"high":   "high",
				},
			},
			"claude-3-haiku": {
				ID:       "claude-3-haiku",
				Name:     "Claude 3 Haiku",
				API:      APIAnthropicMessages,
				Provider: ProviderAnthropic,
				Input:    []Modality{ModalityText},
				Cost:     Cost{Input: 0.25, Output: 1.25},
				ContextWindow: 200000,
				MaxTokens:     4096,
			},
		},
	})
}

func TestGetModel(t *testing.T) {
	setupTestModels()

	m, err := GetModel(ProviderAnthropic, "claude-3-opus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.ID != "claude-3-opus" {
		t.Errorf("expected claude-3-opus, got %s", m.ID)
	}
}

func TestGetModelNotFound(t *testing.T) {
	setupTestModels()

	_, err := GetModel(ProviderAnthropic, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent model")
	}
}

func TestGetModelUnknownProvider(t *testing.T) {
	setupTestModels()

	_, err := GetModel("unknown-provider", "model")
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestGetProviders(t *testing.T) {
	setupTestModels()

	providers := GetProviders()
	if len(providers) == 0 {
		t.Error("expected non-empty providers")
	}

	found := false
	for _, p := range providers {
		if p == ProviderAnthropic {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected anthropic in providers")
	}
}

func TestGetModels(t *testing.T) {
	setupTestModels()

	models := GetModels(ProviderAnthropic)
	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}
}

func TestCalculateCost(t *testing.T) {
	setupTestModels()

	m, _ := GetModel(ProviderAnthropic, "claude-3-opus")
	usage := Usage{
		Input:  1000,
		Output: 500,
	}

	cost := CalculateCost(m, usage)
	expectedInput := 1000.0 * 15.0 / 1_000_000
	expectedOutput := 500.0 * 75.0 / 1_000_000

	if cost.Input != expectedInput {
		t.Errorf("expected input cost %f, got %f", expectedInput, cost.Input)
	}
	if cost.Output != expectedOutput {
		t.Errorf("expected output cost %f, got %f", expectedOutput, cost.Output)
	}
	if cost.Total != expectedInput+expectedOutput {
		t.Errorf("expected total %f, got %f", expectedInput+expectedOutput, cost.Total)
	}
}

func TestGetSupportedThinkingLevels(t *testing.T) {
	setupTestModels()

	m, _ := GetModel(ProviderAnthropic, "claude-3-opus")
	levels := GetSupportedThinkingLevels(m)
	if len(levels) != 3 {
		t.Errorf("expected 3 levels, got %d", len(levels))
	}

	m2, _ := GetModel(ProviderAnthropic, "claude-3-haiku")
	levels2 := GetSupportedThinkingLevels(m2)
	if levels2 != nil {
		t.Errorf("expected nil levels for non-reasoning model, got %v", levels2)
	}
}

func TestClampThinkingLevel(t *testing.T) {
	setupTestModels()

	m, _ := GetModel(ProviderAnthropic, "claude-3-opus")

	// Exact match
	level := ClampThinkingLevel(m, ThinkingMedium)
	if level != ThinkingMedium {
		t.Errorf("expected medium, got %s", level)
	}

	// Clamp xhigh to high
	level = ClampThinkingLevel(m, ThinkingXHigh)
	if level != ThinkingHigh {
		t.Errorf("expected high, got %s", level)
	}
}

func TestModelsAreEqual(t *testing.T) {
	a := Model{ID: "test", Provider: ProviderAnthropic}
	b := Model{ID: "test", Provider: ProviderAnthropic}
	c := Model{ID: "other", Provider: ProviderAnthropic}

	if !ModelsAreEqual(a, b) {
		t.Error("expected equal models")
	}
	if ModelsAreEqual(a, c) {
		t.Error("expected unequal models")
	}
}
