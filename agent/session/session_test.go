package session

import (
	"testing"
	"time"

	core "pi-ai-go/core"
)

func TestSessionMemoryStorage(t *testing.T) {
	store := NewMemoryStorage()
	session, err := NewSession(NewSessionOptions{Storage: store})
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

func TestSessionCheckoutAndFork(t *testing.T) {
	store := NewMemoryStorage()
	sess, err := NewSession(NewSessionOptions{Storage: store})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer sess.Close()

	err = sess.Append(
		SessionTreeEntry{ID: "e1", Type: EntrySessionInfo, SessionID: "test-sess", Timestamp: time.Now()},
		SessionTreeEntry{ID: "e2", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "base message"}, Timestamp: time.Now()},
	)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	err = sess.Fork("feature", nil)
	if err != nil {
		t.Fatalf("Fork: %v", err)
	}

	err = sess.Append(
		SessionTreeEntry{ID: "e3", Type: EntryMessage, Message: core.AssistantMessage{Role: "assistant", Content: []core.ContentBlock{core.TextContent{Text: "feature reply"}}}, Timestamp: time.Now()},
	)
	if err != nil {
		t.Fatalf("Append to feature: %v", err)
	}

	err = sess.Checkout("main")
	if err != nil {
		t.Fatalf("Checkout main: %v", err)
	}

	entries := sess.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries on main, got %d", len(entries))
	}
}

func TestSessionSemanticPath(t *testing.T) {
	store := NewMemoryStorage()
	sess, err := NewSession(NewSessionOptions{Storage: store})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer sess.Close()

	err = sess.Append(
		SessionTreeEntry{ID: "e1", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "q1"}, Timestamp: time.Now()},
		SessionTreeEntry{ID: "e2", Type: EntryMessage, Message: core.AssistantMessage{Role: "assistant", Content: []core.ContentBlock{core.TextContent{Text: "a1"}}}, Timestamp: time.Now()},
		SessionTreeEntry{ID: "e3", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "q2"}, Timestamp: time.Now()},
	)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	snapshot := sess.Read()
	path := semanticPath(snapshot, "e3")
	if len(path) != 3 {
		t.Fatalf("expected semantic path length 3, got %d", len(path))
	}
}

func TestSessionClone(t *testing.T) {
	store := NewMemoryStorage()
	sess, err := NewSession(NewSessionOptions{Storage: store})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer sess.Close()

	err = sess.Append(
		SessionTreeEntry{ID: "e1", Type: EntrySessionInfo, SessionID: "test-clone", Timestamp: time.Now()},
		SessionTreeEntry{ID: "e2", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "hello"}, Timestamp: time.Now()},
	)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	err = sess.Fork("feature", nil)
	if err != nil {
		t.Fatalf("Fork: %v", err)
	}

	cloneStore := NewMemoryStorage()
	cloned, err := sess.Clone(SessionCloneOptions{
		SessionStorage: cloneStore,
		Refs:           "all",
	})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	defer cloned.Close()

	if cloned.ID() != "test-clone" {
		t.Errorf("expected cloned session ID test-clone, got %q", cloned.ID())
	}
}

func TestSessionRebase(t *testing.T) {
	store := NewMemoryStorage()
	sess, err := NewSession(NewSessionOptions{Storage: store})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer sess.Close()

	err = sess.Append(
		SessionTreeEntry{ID: "e1", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "base"}, Timestamp: time.Now()},
	)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	err = sess.Fork("topic", nil)
	if err != nil {
		t.Fatalf("Fork: %v", err)
	}

	err = sess.Append(
		SessionTreeEntry{ID: "e2", Type: EntryMessage, Message: core.AssistantMessage{Role: "assistant", Content: []core.ContentBlock{core.TextContent{Text: "topic reply"}}}, Timestamp: time.Now()},
	)
	if err != nil {
		t.Fatalf("Append to topic: %v", err)
	}

	err = sess.Checkout("main")
	if err != nil {
		t.Fatalf("Checkout main: %v", err)
	}

	err = sess.Append(
		SessionTreeEntry{ID: "e3", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "update"}, Timestamp: time.Now()},
	)
	if err != nil {
		t.Fatalf("Append to main: %v", err)
	}

	_, err = sess.Rebase("topic", "main")
	if err != nil {
		t.Fatalf("Rebase: %v", err)
	}

	entries := sess.Entries()
	if len(entries) < 5 {
		t.Fatalf("expected at least 5 entries after rebase, got %d", len(entries))
	}
}
