package providers

import (
	"testing"

	piai "pi-ai-go"
)

func TestBuildBaseOptions(t *testing.T) {
	model := piai.Model{Provider: piai.ProviderAnthropic}
	opts := piai.SimpleStreamOptions{
		StreamOptions: piai.StreamOptions{
			Temperature: floatPtr(0.7),
		},
		Reasoning: piai.ThinkingHigh,
	}

	base := BuildBaseOptions(model, opts, "test-key")
	if base.APIKey != "test-key" {
		t.Errorf("expected API key 'test-key', got %s", base.APIKey)
	}
	if base.Temperature == nil || *base.Temperature != 0.7 {
		t.Error("expected temperature 0.7")
	}
}

func TestClampReasoning(t *testing.T) {
	tests := []struct {
		input piai.ThinkingLevel
		want  piai.ThinkingLevel
	}{
		{piai.ThinkingLow, piai.ThinkingLow},
		{piai.ThinkingMedium, piai.ThinkingMedium},
		{piai.ThinkingHigh, piai.ThinkingHigh},
		{piai.ThinkingXHigh, piai.ThinkingHigh},
	}

	for _, tt := range tests {
		got := ClampReasoning(tt.input)
		if got != tt.want {
			t.Errorf("ClampReasoning(%s) = %s, want %s", tt.input, got, tt.want)
		}
	}
}

func TestAdjustMaxTokensForThinking(t *testing.T) {
	// Test with custom budget
	thinking, total := AdjustMaxTokensForThinking(4096, 8192, piai.ThinkingHigh, map[string]int{
		"high": 2000,
	})
	if thinking != 2000 {
		t.Errorf("expected thinking budget 2000, got %d", thinking)
	}
	if total != 6096 {
		t.Errorf("expected total 6096, got %d", total)
	}

	// Test with default budget
	thinking, total = AdjustMaxTokensForThinking(4096, 8192, piai.ThinkingMedium, nil)
	if thinking != 4096 {
		t.Errorf("expected thinking budget 4096, got %d", thinking)
	}
	if total != 8192 {
		t.Errorf("expected total 8192, got %d", total)
	}

	// Test with model max cap
	thinking, total = AdjustMaxTokensForThinking(4096, 6000, piai.ThinkingHigh, nil)
	if total > 6000 {
		t.Errorf("expected total <= 6000, got %d", total)
	}
}

func floatPtr(f float64) *float64 {
	return &f
}
