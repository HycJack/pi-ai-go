package harness

import (
	"errors"
	"testing"

	core "pi-ai-go/core"
)

func TestCoerceToolResultNil(t *testing.T) {
	res, malformed := CoerceToolResult(nil, "id1", "tool1")
	if !malformed {
		t.Error("expected malformed=true for nil input")
	}
	if res.ToolCallID != "id1" {
		t.Errorf("got ToolCallID=%q", res.ToolCallID)
	}
	if res.IsError {
		t.Error("expected IsError=false for nil input")
	}
}

func TestCoerceToolResultString(t *testing.T) {
	res, malformed := CoerceToolResult(map[string]any{"content": "ok"}, "id", "tool")
	if malformed {
		t.Error("expected malformed=false")
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(res.Content))
	}
	tc, ok := res.Content[0].(core.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	if tc.Text != "ok" {
		t.Errorf("expected 'ok', got %q", tc.Text)
	}
}

func TestCoerceToolResultMalformed(t *testing.T) {
	// Wrong type for text — must not panic.
	res, malformed := CoerceToolResult(map[string]any{
		"content": []any{map[string]any{"type": "text", "text": 123}},
	}, "id", "tool")
	if !malformed {
		t.Error("expected malformed=true")
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(res.Content))
	}
}

func TestCoerceToolResultMissingContent(t *testing.T) {
	res, malformed := CoerceToolResult(map[string]any{"toolCallId": "abc"}, "abc", "tool")
	if !malformed {
		t.Error("expected malformed=true")
	}
	if !res.IsError {
		t.Error("expected IsError=true")
	}
}

func TestCoerceToolResultErrorPropagated(t *testing.T) {
	res, _ := CoerceToolResult(map[string]any{"content": "fail", "isError": true}, "id", "tool")
	if !res.IsError {
		t.Error("expected IsError=true")
	}
}

func TestCoerceToolResultNonMapRaw(t *testing.T) {
	res, malformed := CoerceToolResult("hello", "id", "tool")
	if !malformed {
		t.Error("expected malformed=true for non-map raw")
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(res.Content))
	}
}

func TestCoerceToolResultPreservesContent(t *testing.T) {
	blocks := []core.ContentBlock{core.TextContent{Type: "text", Text: "ok"}}
	res, malformed := CoerceToolResult(map[string]any{"content": blocks}, "id", "tool")
	if malformed {
		t.Error("expected malformed=false")
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(res.Content))
	}
}

// errStringer is a small Stringer for testing the Stringer branch.
type errStringer struct{}

func (errStringer) Error() string { return "stranger error" }

func TestCoerceToStringBranches(t *testing.T) {
	if coerceToString(nil) != "" {
		t.Error("nil should be empty")
	}
	if coerceToString("hi") != "hi" {
		t.Error("string passthrough failed")
	}
	if coerceToString([]byte("bytes")) != "bytes" {
		t.Error("[]byte failed")
	}
	if coerceToString(errors.New("err")) != "err" {
		t.Error("error failed")
	}
	if coerceToString(errStringer{}) != "stranger error" {
		t.Error("Stringer failed")
	}
}
