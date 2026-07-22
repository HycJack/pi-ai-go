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

// withCwdInTempDir sets the process working directory to a fresh temp
// directory and returns the (post-chdir) resolved cwd. Tests should
// build paths by joining with the returned value so that resolveSafePath
// (which jails paths to the working directory and resolves symlinks)
// accepts them.
//
// On Windows, t.TempDir() often returns an 8.3 short-name path that
// resolves to a different absolute path than the one used in
// os.Chdir — this causes false negatives in the jail check. We
// resolve the symlinks before chdir so that both sides agree.
func withCwdInTempDir(t *testing.T) string {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(resolved); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	return resolved
}

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
	dir := withCwdInTempDir(t)
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
	dir := withCwdInTempDir(t)
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
	dir := withCwdInTempDir(t)
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
	dir := withCwdInTempDir(t)
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
	dir := withCwdInTempDir(t)
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
	if len(all) != 14 {
		t.Errorf("expected 14 tools, got %d", len(all))
	}
	names := map[string]bool{}
	for _, tool := range all {
		names[tool.Name] = true
	}
	want := []string{
		"read_file", "write_file", "append_file", "edit_file", "bash",
		"list_files", "show_file", "head", "tail", "find_files",
		"change_dir", "print_working_dir",
		"glob", "grep",
	}
	for _, name := range want {
		if !names[name] {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestAppend(t *testing.T) {
	dir := withCwdInTempDir(t)

	// Case 1: file does not exist; append creates it.
	p := filepath.Join(dir, "new.txt")
	r := Append()
	res, _ := r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"filePath": p, "content": "first chunk\n",
	}), nil)
	if res.IsError {
		t.Fatalf("expected success, got: %s", textOf(t, res.Content))
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "first chunk\n" {
		t.Errorf("expected 'first chunk\\n', got %q", string(data))
	}

	// Case 2: subsequent appends accumulate content.
	res, _ = r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"filePath": p, "content": "second chunk\n",
	}), nil)
	if res.IsError {
		t.Fatalf("expected success, got: %s", textOf(t, res.Content))
	}
	res, _ = r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"filePath": p, "content": "third chunk\n",
	}), nil)
	if res.IsError {
		t.Fatalf("expected success, got: %s", textOf(t, res.Content))
	}
	data, _ = os.ReadFile(p)
	want := "first chunk\nsecond chunk\nthird chunk\n"
	if string(data) != want {
		t.Errorf("expected %q, got %q", want, string(data))
	}

	// Case 3: parent directories are created on demand.
	nested := filepath.Join(dir, "deep", "nested", "sub", "file.txt")
	res, _ = r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"filePath": nested, "content": "hello",
	}), nil)
	if res.IsError {
		t.Fatalf("expected success creating nested dirs, got: %s", textOf(t, res.Content))
	}
	data, _ = os.ReadFile(nested)
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}

	// Case 4: result reports totalBytes so model can track progress.
	res, _ = r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"filePath": p, "content": "more\n",
	}), nil)
	if res.IsError {
		t.Fatalf("expected success, got: %s", textOf(t, res.Content))
	}
	out := textOf(t, res.Content)
	if !strings.Contains(out, "bytes total") {
		t.Errorf("expected progress info in result, got: %s", out)
	}
}

func TestListFiles(t *testing.T) {
	dir := withCwdInTempDir(t)
	for _, n := range []string{"a.txt", "b.txt", ".hidden"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := ListFiles()
	res, _ := r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"path": dir,
	}), nil)
	if res.IsError {
		t.Fatalf("expected success, got: %s", textOf(t, res.Content))
	}
	out := textOf(t, res.Content)
	if !strings.Contains(out, "a.txt") || !strings.Contains(out, "b.txt") {
		t.Errorf("expected files listed, got: %s", out)
	}
	if strings.Contains(out, ".hidden") {
		t.Errorf("hidden file should be excluded by default, got: %s", out)
	}
	if !strings.Contains(out, "sub/") {
		t.Errorf("expected sub/ marker for directory, got: %s", out)
	}

	// showAll includes hidden files.
	res, _ = r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"path": dir, "showAll": true,
	}), nil)
	if !strings.Contains(textOf(t, res.Content), ".hidden") {
		t.Errorf("expected .hidden with showAll, got: %s", textOf(t, res.Content))
	}
}

func TestShowFileAndHeadTail(t *testing.T) {
	dir := withCwdInTempDir(t)
	p := filepath.Join(dir, "lines.txt")
	if err := os.WriteFile(p, []byte("alpha\nbravo\ncharlie\ndelta\necho\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// show_file prints the whole content
	r := ShowFile()
	res, _ := r.Execute(context.Background(), "id", mustJSON(t, map[string]any{"filePath": p}), nil)
	if res.IsError {
		t.Fatalf("expected success, got: %s", textOf(t, res.Content))
	}
	if !strings.Contains(textOf(t, res.Content), "charlie") {
		t.Errorf("expected file contents, got: %s", textOf(t, res.Content))
	}

	// head defaults to 10 lines but file only has 5
	h := Head()
	res, _ = h.Execute(context.Background(), "id", mustJSON(t, map[string]any{"filePath": p, "lines": 2}), nil)
	if res.IsError {
		t.Fatalf("head failed: %s", textOf(t, res.Content))
	}
	out := textOf(t, res.Content)
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "bravo") {
		t.Errorf("expected first two lines, got: %s", out)
	}
	if strings.Contains(out, "charlie") {
		t.Errorf("did not expect charlie in head, got: %s", out)
	}

	// tail returns the last N lines
	tt := Tail()
	res, _ = tt.Execute(context.Background(), "id", mustJSON(t, map[string]any{"filePath": p, "lines": 2}), nil)
	if res.IsError {
		t.Fatalf("tail failed: %s", textOf(t, res.Content))
	}
	out = textOf(t, res.Content)
	if !strings.Contains(out, "delta") || !strings.Contains(out, "echo") {
		t.Errorf("expected last two lines, got: %s", out)
	}
	if strings.Contains(out, "alpha") {
		t.Errorf("did not expect alpha in tail, got: %s", out)
	}
}

func TestChangeDirAndPwd(t *testing.T) {
	dir := withCwdInTempDir(t)
	sub := filepath.Join(dir, "nested")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	r := ChangeDir()
	res, _ := r.Execute(context.Background(), "id", mustJSON(t, map[string]any{"path": sub}), nil)
	if res.IsError {
		t.Fatalf("change_dir failed: %s", textOf(t, res.Content))
	}
	wd, _ := os.Getwd()
	if filepath.Clean(wd) != filepath.Clean(sub) {
		t.Errorf("expected cwd=%s, got %s", sub, wd)
	}

	// print_working_dir reports the new cwd
	p := PrintWorkingDir()
	res, _ = p.Execute(context.Background(), "id", mustJSON(t, map[string]any{}), nil)
	if res.IsError {
		t.Fatalf("pwd failed: %s", textOf(t, res.Content))
	}
	if !strings.Contains(textOf(t, res.Content), filepath.Base(sub)) {
		t.Errorf("expected %s in pwd output, got: %s", filepath.Base(sub), textOf(t, res.Content))
	}
}

func TestFindFiles(t *testing.T) {
	dir := withCwdInTempDir(t)
	for _, n := range []string{"apple.txt", "banana.txt", "Apricot.md", "sub/apply.md"} {
		full := filepath.Join(dir, n)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := FindFiles()
	// Case-insensitive substring "ap" should match three files.
	res, _ := r.Execute(context.Background(), "id", mustJSON(t, map[string]any{
		"path": dir, "name": "ap",
	}), nil)
	if res.IsError {
		t.Fatalf("find_files failed: %s", textOf(t, res.Content))
	}
	out := textOf(t, res.Content)
	if strings.Count(out, "\n") < 3 {
		t.Errorf("expected at least 3 matches for 'ap', got: %s", out)
	}
}
