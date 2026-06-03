package agent

import (
	"strings"
	"testing"
	"time"

	core "pi-ai-go/core"
)

func TestContextWindowSoftHardLimits(t *testing.T) {
	model := core.Model{ContextWindow: 1000}
	policy := ContextPolicy{SoftLimit: 0.5, HardLimit: 0.9, ReservedOutput: 0}
	cw := NewContextWindow(model, policy)
	if cw.Effective != 1000 {
		t.Errorf("Effective = %d", cw.Effective)
	}
	if cw.SoftTokens != 500 {
		t.Errorf("SoftTokens = %d", cw.SoftTokens)
	}
	if cw.HardTokens != 900 {
		t.Errorf("HardTokens = %d", cw.HardTokens)
	}
	cw.Add(450)
	if cw.NeedsCompaction() {
		t.Error("should not need compaction at 450")
	}
	cw.Add(60)
	if !cw.NeedsCompaction() {
		t.Error("should need compaction at 510")
	}
	if cw.MustCompact() {
		t.Error("should not yet be hard at 510")
	}
	cw.Add(400)
	if !cw.MustCompact() {
		t.Error("should be hard at 910")
	}
}

func TestEstimateMessagesTokens(t *testing.T) {
	model := core.Model{}
	msgs := []core.Message{
		core.UserMessage{Content: strings.Repeat("a", 400), Timestamp: time.Now()},
		core.AssistantMessage{Content: []core.ContentBlock{core.TextContent{Text: strings.Repeat("b", 800)}}, Timestamp: time.Now()},
	}
	got := EstimateMessagesTokens(msgs, model)
	// 400/4 + 800/4 + 4 (overhead) + 4 = 100 + 200 + 8 = 308
	// + 16 model overhead
	want := 324
	if got != want {
		t.Errorf("EstimateMessagesTokens = %d, want %d", got, want)
	}
}

func TestSlidingWindowCompact(t *testing.T) {
	model := core.Model{ContextWindow: 1000}
	msgs := make([]core.Message, 20)
	for i := range msgs {
		msgs[i] = core.UserMessage{
			Role:      "user",
			Content:   strings.Repeat("x", 1000), // ~250 tokens each
			Timestamp: time.Now(),
		}
	}
	// Target 1500 tokens ≈ 6 messages (each ~250 tokens)
	compacted, dropped := SlidingWindowCompact(msgs, model, 1500, 4)
	if dropped == 0 {
		t.Error("expected some messages dropped")
	}
	if len(compacted) >= len(msgs) {
		t.Errorf("compacted should be shorter: %d >= %d", len(compacted), len(msgs))
	}
	// Tail must always be preserved.
	if len(compacted) < 4 {
		t.Errorf("compacted must keep at least 4 tail messages, got %d", len(compacted))
	}
}

func TestSlidingWindowPreservesSummary(t *testing.T) {
	model := core.Model{ContextWindow: 1000}
	msgs := []core.Message{
		core.UserMessage{Content: "__compaction_summary__ prior summary", Timestamp: time.Now()},
		core.UserMessage{Content: "old1", Timestamp: time.Now()},
		core.UserMessage{Content: "old2", Timestamp: time.Now()},
		core.UserMessage{Content: "new1", Timestamp: time.Now()},
		core.UserMessage{Content: "new2", Timestamp: time.Now()},
	}
	// Target so small that without summary protection everything would drop.
	compacted, _ := SlidingWindowCompact(msgs, model, 1, 2)
	hasSummary := false
	for _, m := range compacted {
		if um, ok := m.(core.UserMessage); ok {
			if s, ok := um.Content.(string); ok && strings.Contains(s, CompactSummaryUserMessage) {
				hasSummary = true
			}
		}
	}
	if !hasSummary {
		t.Error("compaction summary should have been preserved")
	}
}

func TestCompactByStrategySliding(t *testing.T) {
	model := core.Model{ContextWindow: 100}
	policy := ContextPolicy{SoftLimit: 0.5, HardLimit: 0.9, ReservedOutput: 0, MinTailMessages: 1}
	msgs := []core.Message{
		core.UserMessage{Content: strings.Repeat("x", 400), Timestamp: time.Now()},
		core.UserMessage{Content: strings.Repeat("y", 400), Timestamp: time.Now()},
		core.UserMessage{Content: strings.Repeat("z", 400), Timestamp: time.Now()},
	}
	compacted, dropped, err := CompactByStrategy(nil, msgs, model, policy, nil)
	if err != nil {
		t.Fatalf("CompactByStrategy: %v", err)
	}
	if dropped == 0 {
		t.Error("expected drop")
	}
	if len(compacted) >= len(msgs) {
		t.Errorf("compacted should be shorter: %d >= %d", len(compacted), len(msgs))
	}
}

func TestCompactByStrategyNoopWhenUnderLimit(t *testing.T) {
	model := core.Model{ContextWindow: 10000}
	policy := ContextPolicy{SoftLimit: 0.5, HardLimit: 0.9, ReservedOutput: 0}
	msgs := []core.Message{
		core.UserMessage{Content: "hello", Timestamp: time.Now()},
	}
	compacted, dropped, err := CompactByStrategy(nil, msgs, model, policy, nil)
	if err != nil {
		t.Fatalf("CompactByStrategy: %v", err)
	}
	if dropped != 0 {
		t.Errorf("expected no drop, got %d", dropped)
	}
	if len(compacted) != len(msgs) {
		t.Errorf("expected unchanged length")
	}
}

func TestCompactSummarizeInPlace(t *testing.T) {
	msgs := []core.Message{
		core.UserMessage{Content: "old1", Timestamp: time.Now()},
		core.UserMessage{Content: "old2", Timestamp: time.Now()},
		core.UserMessage{Content: "new1", Timestamp: time.Now()},
	}
	out := CompactSummarizeInPlace(msgs, "summary text", 2, time.Now())
	if len(out) != 2 { // 1 summary + 1 new message
		t.Errorf("expected 2 messages, got %d", len(out))
	}
	um, ok := out[0].(core.UserMessage)
	if !ok {
		t.Fatal("expected UserMessage")
	}
	if !strings.Contains(stringifyContent(um.Content), "summary text") {
		t.Error("summary not inserted")
	}
}
