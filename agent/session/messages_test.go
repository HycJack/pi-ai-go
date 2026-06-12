package session

import (
	"testing"
	"time"

	core "pi-ai-go/core"
)

func TestBashExecutionToText(t *testing.T) {
	exitCode := 1
	msg := BashExecutionMessage{
		Command:   "ls -la",
		Output:    "file1\nfile2",
		ExitCode:  &exitCode,
		Timestamp: time.Now(),
	}
	text := BashExecutionToText(msg)
	if !containsSubstr(text, "ls -la") {
		t.Error("expected command in output")
	}
	if !containsSubstr(text, "exited with code 1") {
		t.Error("expected exit code in output")
	}
}

func TestBashExecutionCancelled(t *testing.T) {
	msg := BashExecutionMessage{
		Command:   "sleep 100",
		Cancelled: true,
		Timestamp: time.Now(),
	}
	text := BashExecutionToText(msg)
	if !containsSubstr(text, "cancelled") {
		t.Error("expected cancelled in output")
	}
}

func TestBashExecutionTruncated(t *testing.T) {
	msg := BashExecutionMessage{
		Command:        "cat big.log",
		Output:         "truncated...",
		Truncated:      true,
		FullOutputPath: "/tmp/output.log",
		Timestamp:      time.Now(),
	}
	text := BashExecutionToText(msg)
	if !containsSubstr(text, "truncated") {
		t.Error("expected truncated in output")
	}
	if !containsSubstr(text, "/tmp/output.log") {
		t.Error("expected full output path")
	}
}

func TestConvertEntriesToLlmAllTypes(t *testing.T) {
	now := time.Now()
	exitCode := 0
	entries := []SessionTreeEntry{
		{ID: "1", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "hi"}, Timestamp: now},
		{ID: "2", Type: EntryCustomMessage, CustomType: "status", Content: "ready", Timestamp: now},
		{ID: "3", Type: EntryBranchSummary, Summary: "tried approach A", Timestamp: now},
		{ID: "4", Type: EntryCompaction, CompactionSummary: "earlier context", TokensBefore: 500, Timestamp: now},
		{ID: "5", Type: EntryMessage, Message: core.AssistantMessage{Role: "assistant", Content: []core.ContentBlock{core.TextContent{Text: "ok"}}}, Timestamp: now},
		// Non-message entries should be skipped
		{ID: "6", Type: EntryModelChange, Provider: "openai", ModelID: "gpt-4", Timestamp: now},
		{ID: "7", Type: EntryThinkingChange, ThinkingLevel: "high", Timestamp: now},
	}

	msgs := ConvertEntriesToLlm(entries)
	// Should get: user, custom, branch, compaction, assistant = 5
	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(msgs))
	}

	// Verify types
	if _, ok := msgs[0].(core.UserMessage); !ok {
		t.Errorf("msg 0: expected UserMessage, got %T", msgs[0])
	}
	if _, ok := msgs[4].(core.AssistantMessage); !ok {
		t.Errorf("msg 4: expected AssistantMessage, got %T", msgs[4])
	}

	_ = exitCode // suppress unused
}

func TestSerializeMessagesForSummary(t *testing.T) {
	msgs := []core.Message{
		core.UserMessage{Role: "user", Content: "What is Go?"},
		core.AssistantMessage{Role: "assistant", Content: []core.ContentBlock{core.TextContent{Text: "A programming language."}}},
		core.ToolResultMessage{Role: "tool", ToolName: "search", Content: []core.ContentBlock{core.TextContent{Text: "Go is..."}}},
	}

	text := SerializeMessagesForSummary(msgs)
	if !containsSubstr(text, "What is Go?") {
		t.Error("expected user question")
	}
	if !containsSubstr(text, "programming language") {
		t.Error("expected assistant answer")
	}
	if !containsSubstr(text, "search") {
		t.Error("expected tool name")
	}
}
