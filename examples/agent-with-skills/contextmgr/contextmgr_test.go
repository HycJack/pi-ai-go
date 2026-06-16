package contextmgr

import (
	"testing"

	"pi-ai-go/core"
)

func TestEstimateTokensChinese(t *testing.T) {
	msg := core.UserMessage{
		Role:    "user",
		Content: "你好世界",
	}
	tokens := estimateMessageTokens(msg)
	// 4 CJK chars + 4 overhead = ~2 + 4 = 6
	if tokens < 4 || tokens > 10 {
		t.Errorf("CJK estimate = %d, expected 4-10", tokens)
	}
}

func TestEstimateTokensEnglish(t *testing.T) {
	msg := core.UserMessage{
		Role:    "user",
		Content: "Hello, world!",
	}
	tokens := estimateMessageTokens(msg)
	// 13 chars / 4 = ~3 + 4 overhead = 7
	if tokens < 5 || tokens > 12 {
		t.Errorf("English estimate = %d, expected 5-12", tokens)
	}
}

func TestEstimateTokensMixed(t *testing.T) {
	msg := core.UserMessage{
		Role:    "user",
		Content: "Hello 你好 World 世界",
	}
	tokens := estimateMessageTokens(msg)
	// CJK: 4 chars ≈ 3 tokens; ASCII: 11 chars ≈ 3 tokens; + 4 overhead = 10
	if tokens < 6 || tokens > 14 {
		t.Errorf("Mixed estimate = %d, expected 6-14", tokens)
	}
}

func TestComputeStats(t *testing.T) {
	settings := Settings{
		MaxContextTokens:    1000,
		SoftLimitRatio:      0.7,
		HardLimitRatio:      0.95,
		ReservedForResponse: 100,
	}
	messages := []core.Message{
		core.UserMessage{Content: "Hello, world!"},
		core.AssistantMessage{
			Content: []core.ContentBlock{
				core.TextContent{Text: "Hi there!"},
			},
		},
	}
	stats := ComputeStats(messages, settings)
	if stats.MessageCount != 2 {
		t.Errorf("MessageCount = %d, want 2", stats.MessageCount)
	}
	if stats.EstimatedTokens <= 0 {
		t.Errorf("EstimatedTokens = %d, want > 0", stats.EstimatedTokens)
	}
	if stats.MaxContext != 1000 {
		t.Errorf("MaxContext = %d, want 1000", stats.MaxContext)
	}
	if stats.SoftLimit != int(float64(900)*0.7) {
		t.Errorf("SoftLimit = %d, want %d", stats.SoftLimit, int(float64(900)*0.7))
	}
}

func TestShouldCompact(t *testing.T) {
	settings := Settings{
		MaxContextTokens:    1000,
		SoftLimitRatio:      0.5,
		HardLimitRatio:      0.95,
		ReservedForResponse: 0,
	}
	// Below soft limit
	stats := Stats{EstimatedTokens: 100}
	if ShouldCompact(stats, settings) {
		t.Error("ShouldCompact with 100 tokens (soft=500) = true, want false")
	}
	// Above soft limit
	stats = Stats{EstimatedTokens: 600}
	if !ShouldCompact(stats, settings) {
		t.Error("ShouldCompact with 600 tokens (soft=500) = false, want true")
	}
}

func TestShouldTruncate(t *testing.T) {
	settings := Settings{
		MaxContextTokens:    1000,
		SoftLimitRatio:      0.5,
		HardLimitRatio:      0.95,
		ReservedForResponse: 0,
	}
	stats := Stats{EstimatedTokens: 1000} // exceeds hard (950)
	if !ShouldTruncate(stats, settings) {
		t.Error("ShouldTruncate at 1000 tokens (hard=950) = false, want true")
	}
	stats = Stats{EstimatedTokens: 100} // below
	if ShouldTruncate(stats, settings) {
		t.Error("ShouldTruncate at 100 tokens = true, want false")
	}
}

func TestTruncate(t *testing.T) {
	messages := []core.Message{
		core.UserMessage{Content: "1"},
		core.AssistantMessage{Content: []core.ContentBlock{core.TextContent{Text: "1"}}},
		core.UserMessage{Content: "2"},
		core.AssistantMessage{Content: []core.ContentBlock{core.TextContent{Text: "2"}}},
		core.UserMessage{Content: "3"},
	}

	result := Truncate(messages, 2)
	if len(result) != 2 {
		t.Errorf("len(Truncate) = %d, want 2", len(result))
	}

	// Should keep last 2: index 3 and 4 in original
	// messages[3] = AssistantMessage{"2"}, messages[4] = UserMessage{"3"}
	// result[0] should be AssistantMessage{"2"}, result[1] should be UserMessage{"3"}
	if am, ok := result[0].(core.AssistantMessage); !ok {
		t.Errorf("Truncate[0] type = %T, want core.AssistantMessage", result[0])
	} else {
		if len(am.Content) == 0 {
			t.Error("Truncate[0] has empty content")
		} else if tc, ok := am.Content[0].(core.TextContent); !ok || tc.Text != "2" {
			t.Errorf("Truncate[0] content = %v, want TextContent{2}", am.Content[0])
		}
	}

	if um, ok := result[1].(core.UserMessage); !ok || um.Content != "3" {
		t.Errorf("Truncate[1] = %v, want UserMessage{3}", result[1])
	}

	// Truncate with n >= len should return original
	result = Truncate(messages, 10)
	if len(result) != len(messages) {
		t.Errorf("Truncate with n=10 returned %d, want %d", len(result), len(messages))
	}
}

func TestDefaultSettings(t *testing.T) {
	tests := []struct {
		model  string
		expect int
	}{
		{"gpt-4o", 128000},
		{"claude-sonnet-4-5", 200000},
		{"deepseek-chat", 64000},
		{"unknown-model", 128000}, // default
	}
	for _, tt := range tests {
		s := DefaultSettings(tt.model)
		if s.MaxContextTokens != tt.expect {
			t.Errorf("DefaultSettings(%q).MaxContextTokens = %d, want %d",
				tt.model, s.MaxContextTokens, tt.expect)
		}
	}
}

func TestFormatStats(t *testing.T) {
	stats := Stats{
		MessageCount:    5,
		EstimatedTokens: 500,
		MaxContext:      1000,
		SoftLimit:       700,
		HardLimit:       950,
		UsageRatio:      0.5,
	}
	s := FormatStats(stats)
	if s == "" {
		t.Error("FormatStats returned empty")
	}
}

func TestEstimateTokensEmpty(t *testing.T) {
	tokens := EstimateTokens(nil)
	if tokens != 0 {
		t.Errorf("Empty EstimateTokens = %d, want 0", tokens)
	}
}
