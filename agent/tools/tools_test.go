package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	core "pi-ai-go/core"
)

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// textOf joins all TextContent blocks of an AgentToolResult into one
// string. Non-text blocks are ignored.
func textOf(t *testing.T, content []core.ContentBlock) string {
	t.Helper()
	var sb strings.Builder
	for _, b := range content {
		if tb, ok := b.(core.TextContent); ok {
			sb.WriteString(tb.Text)
		}
	}
	return sb.String()
}

func TestRead(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(p, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := Read()
	res, _ := r.Execute(context.Background(), "id", mustJSON(t, map[string]any{"filePath": p}), nil)
	if res.IsError {
		t.Fatalf("expected success, got: %s", textOf(t, res.Content))
	}
	if !strings.Contains(textOf(t, res.Content), "line1") {
		t.Errorf("expected line1 in result, got: %s", textOf(t, res.Content))
	}

	// Offset + limit
	res, _ = r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"filePath": p, "offset": 1, "limit": 1,
	}), nil)
	out := textOf(t, res.Content)
	if !strings.Contains(out, "line2") || strings.Contains(out, "line3") {
		t.Errorf("expected only line2, got: %s", out)
	}

	// Missing file
	res, _ = r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"filePath": filepath.Join(dir, "nope.txt"),
	}), nil)
	if !res.IsError {
		t.Error("expected error result for missing file")
	}
}

func TestWrite(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "out.txt")
	r := Write()
	res, _ := r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"filePath": p, "content": "hello world",
	}), nil)
	if res.IsError {
		t.Fatalf("expected success, got: %s", textOf(t, res.Content))
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("got %q", string(data))
	}
}

func TestEditSingleOccurrence(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(p, []byte("foo bar foo"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := Edit()
	res, _ := r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"filePath": p, "oldText": "foo", "newText": "baz",
	}), nil)
	if res.IsError {
		t.Fatalf("expected success, got: %s", textOf(t, res.Content))
	}
	data, _ := os.ReadFile(p)
	if string(data) != "baz bar foo" {
		t.Errorf("got %q", string(data))
	}

	// Default: still replace only the first occurrence when there
	// are multiple matches, with a note in the result.
	if err := os.WriteFile(p, []byte("foo foo foo"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, _ = r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"filePath": p, "oldText": "foo", "newText": "X",
	}), nil)
	if res.IsError {
		t.Fatalf("default should not error on multi-match, got: %s", textOf(t, res.Content))
	}
	data, _ = os.ReadFile(p)
	if string(data) != "X foo foo" {
		t.Errorf("expected first-only replacement, got %q", string(data))
	}

	// allOccurrences
	res, _ = r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"filePath": p, "oldText": "foo", "newText": "X", "allOccurrences": true,
	}), nil)
	if res.IsError {
		t.Fatalf("expected success, got: %s", textOf(t, res.Content))
	}
	data, _ = os.ReadFile(p)
	if string(data) != "X X X" {
		t.Errorf("got %q", string(data))
	}
}

func TestBashAutoShell(t *testing.T) {
	r := Bash()
	res, _ := r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"command": "echo hello",
	}), nil)
	if runtime.GOOS == "windows" {
		// cmd.exe is ubiquitous on Windows; this test only runs if it succeeds.
		if res.IsError {
			t.Skipf("cmd.exe not available: %s", textOf(t, res.Content))
		}
	} else {
		if res.IsError {
			t.Fatalf("expected success, got: %s", textOf(t, res.Content))
		}
		if !strings.Contains(strings.TrimSpace(textOf(t, res.Content)), "hello") {
			t.Errorf("expected 'hello' in output, got: %s", textOf(t, res.Content))
		}
	}
}

func TestBashErrorPropagation(t *testing.T) {
	r := Bash()
	res, _ := r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"command": "exit 7",
	}), nil)
	if !res.IsError {
		t.Errorf("expected IsError=true for non-zero exit")
	}
}

func TestGlob(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.go", "b.go", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := Glob()
	res, _ := r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"pattern":  "*.go",
		"basePath": dir,
	}), nil)
	if res.IsError {
		t.Fatalf("expected success, got: %s", textOf(t, res.Content))
	}
	out := textOf(t, res.Content)
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "b.go") {
		t.Errorf("expected go files in output, got: %s", out)
	}
	if strings.Contains(out, "c.txt") {
		t.Errorf("did not expect c.txt, got: %s", out)
	}
}

func TestGrep(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(p, []byte("alpha\nBeta\ngamma\nBetaX\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := Grep()
	// Substring, case-insensitive.
	res, _ := r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"pattern":  "beta",
		"basePath": dir,
	}), nil)
	if res.IsError {
		t.Fatalf("expected success, got: %s", textOf(t, res.Content))
	}
	out := textOf(t, res.Content)
	if !strings.Contains(out, "Beta") || !strings.Contains(out, "BetaX") {
		t.Errorf("expected both Beta matches, got: %s", out)
	}
	if strings.Count(out, "\n") < 2 {
		t.Errorf("expected at least 2 matches, got: %s", out)
	}

	// Regex
	res, _ = r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"pattern":  "^gamma$",
		"basePath": dir,
		"regex":    true,
	}), nil)
	if !strings.Contains(textOf(t, res.Content), "gamma") {
		t.Errorf("expected gamma match, got: %s", textOf(t, res.Content))
	}
}

func TestAllReturnsAllTools(t *testing.T) {
	all := All()
	if len(all) != 6 {
		t.Errorf("expected 6 tools, got %d", len(all))
	}
	names := map[string]bool{}
	for _, tool := range all {
		names[tool.Name] = true
	}
	for _, want := range []string{"read_file", "write_file", "edit_file", "bash", "glob", "grep"} {
		if !names[want] {
			t.Errorf("missing tool: %s", want)
		}
	}
}
