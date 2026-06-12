package session

import (
	"path/filepath"
	"testing"
	"time"

	core "pi-ai-go/core"
)

func TestJSONLStorage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	store, err := NewJSONLStorage(path)
	if err != nil {
		t.Fatalf("NewJSONLStorage: %v", err)
	}

	entries := []SessionTreeEntry{
		{ID: "e1", Type: EntrySessionInfo, SessionID: "test", Timestamp: time.Now()},
		{ID: "e2", Type: EntryMessage, Message: core.UserMessage{Role: "user", Content: "hello"}, Timestamp: time.Now()},
	}

	if err := store.Append(entries); err != nil {
		t.Fatalf("Append: %v", err)
	}

	read, err := store.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(read) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(read))
	}
	if read[0].SessionID != "test" {
		t.Errorf("expected session ID test, got %q", read[0].SessionID)
	}

	store.Close()

	// Re-open and verify persistence
	store2, err := NewJSONLStorage(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer store2.Close()

	read2, err := store2.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(read2) != 2 {
		t.Fatalf("expected 2 entries after reopen, got %d", len(read2))
	}
}

func TestJSONLStorageAppendMultiple(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	store, _ := NewJSONLStorage(path)
	defer store.Close()

	store.Append([]SessionTreeEntry{{ID: "1", Type: EntryMessage, Timestamp: time.Now()}})
	store.Append([]SessionTreeEntry{{ID: "2", Type: EntryMessage, Timestamp: time.Now()}})
	store.Append([]SessionTreeEntry{{ID: "3", Type: EntryMessage, Timestamp: time.Now()}})

	read, _ := store.ReadAll()
	if len(read) != 3 {
		t.Fatalf("expected 3, got %d", len(read))
	}
}
