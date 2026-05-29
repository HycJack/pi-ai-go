package harness

import (
	"testing"
	"time"

	core "pi-ai-go/core"
)

func TestSessionMemoryStorage(t *testing.T) {
	store := NewMemoryStorage()
	session, err := NewSession(store)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()

	if session.ID() != "" {
		t.Errorf("expected empty ID, got %q", session.ID())
	}

	// Append entries
	err = session.Append(
		SessionTreeEntry{ID: "e1", Type: EntrySessionInfo, SessionID: "sess-1", Timestamp: time.Now()},
		SessionTreeEntry{ID: "e2", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "hello"}, Timestamp: time.Now()},
		SessionTreeEntry{ID: "e3", Type: EntryMessage, Message: core.AssistantMessage{Role: "assistant", Content: []core.ContentBlock{core.TextContent{Type: "text", Text: "hi"}}}, Timestamp: time.Now()},
	)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	if session.ID() != "sess-1" {
		t.Errorf("expected ID sess-1, got %q", session.ID())
	}

	entries := session.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}

func TestBuildSessionContext(t *testing.T) {
	now := time.Now()
	entries := []SessionTreeEntry{
		{ID: "e1", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "q1"}, Timestamp: now},
		{ID: "e2", Type: EntryMessage, Message: core.AssistantMessage{Role: "assistant", Content: []core.ContentBlock{core.TextContent{Text: "a1"}}}, Timestamp: now},
		{ID: "e3", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "q2"}, Timestamp: now},
	}

	ctx := BuildSessionContext(entries)
	if len(ctx.Messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(ctx.Messages))
	}
}

func TestBuildSessionContextWithCompaction(t *testing.T) {
	now := time.Now()
	entries := []SessionTreeEntry{
		{ID: "e1", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "old"}, Timestamp: now},
		{ID: "e2", Type: EntryCompaction, CompactionSummary: "discussed X", TokensBefore: 1000, FirstKeptEntryID: "e3", Timestamp: now},
		{ID: "e3", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "recent"}, Timestamp: now},
		{ID: "e4", Type: EntryMessage, Message: core.AssistantMessage{Role: "assistant", Content: []core.ContentBlock{core.TextContent{Text: "reply"}}}, Timestamp: now},
	}

	ctx := BuildSessionContext(entries)
	// Should have: compaction summary + e3 + e4 = 3 messages
	if len(ctx.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(ctx.Messages))
	}
	// First message should be compaction summary (UserMessage)
	if um, ok := ctx.Messages[0].(core.UserMessage); ok {
		content, _ := um.Content.(string)
		if content == "" {
			t.Error("expected compaction summary text")
		}
	} else {
		t.Errorf("expected UserMessage for compaction, got %T", ctx.Messages[0])
	}
}

func TestBuildSessionContextWithModelChange(t *testing.T) {
	now := time.Now()
	entries := []SessionTreeEntry{
		{ID: "e1", Type: EntryModelChange, Provider: "anthropic", ModelID: "claude-3", Timestamp: now},
		{ID: "e2", Type: EntryThinkingChange, ThinkingLevel: "high", Timestamp: now},
		{ID: "e3", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "test"}, Timestamp: now},
	}

	ctx := BuildSessionContext(entries)
	if ctx.Model == nil || ctx.Model.Provider != "anthropic" {
		t.Errorf("expected model anthropic, got %v", ctx.Model)
	}
	if ctx.ThinkingLevel != "high" {
		t.Errorf("expected thinking level high, got %q", ctx.ThinkingLevel)
	}
}

func TestBranchSummary(t *testing.T) {
	now := time.Now()
	entries := []SessionTreeEntry{
		{ID: "e1", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "test"}, Timestamp: now},
		{ID: "e2", Type: EntryBranchSummary, Summary: "explored option A", FromID: "branch-1", Timestamp: now},
		{ID: "e3", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "continue"}, Timestamp: now},
	}

	ctx := BuildSessionContext(entries)
	if len(ctx.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(ctx.Messages))
	}
	// Second message should be branch summary as UserMessage
	if um, ok := ctx.Messages[1].(core.UserMessage); ok {
		content, _ := um.Content.(string)
		if content == "" {
			t.Error("expected branch summary text")
		}
	} else {
		t.Errorf("expected UserMessage for branch summary, got %T", ctx.Messages[1])
	}
}

func TestConvertEntriesToLlm(t *testing.T) {
	now := time.Now()
	entries := []SessionTreeEntry{
		{ID: "e1", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "hello"}, Timestamp: now},
		{ID: "e2", Type: EntryCustomMessage, CustomType: "system", Content: "notice", Timestamp: now},
		{ID: "e3", Type: EntryCompaction, CompactionSummary: "previous context", TokensBefore: 500, Timestamp: now},
		{ID: "e4", Type: EntryMessage, Message: core.AssistantMessage{Role: "assistant", Content: []core.ContentBlock{core.TextContent{Text: "ok"}}}, Timestamp: now},
	}

	msgs := ConvertEntriesToLlm(entries)
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
}
