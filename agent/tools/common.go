package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	core "pi-ai-go/core"
)

// This file provides cross-platform shell-equivalent tools that mirror the
// familiar Unix commands (ls, cd, cat, pwd, find, head, tail) but work on
// both Windows and macOS/Linux without requiring shell parsing. They are
// pure Go implementations on top of os/filepath, so they handle path
// separators and file metadata correctly on every OS.

// detailsJSON encodes v as JSON or returns an empty RawMessage on error.
// It is intended for inline tool-result details, where encoding errors
// should never abort the tool.
func detailsJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}

// ─── list_files (ls) ─────────────────────────────────────────────────────

const listFilesSchema = `{
	"type": "object",
	"properties": {
		"path":      { "type": "string", "description": "Directory to list (default: current working directory)." },
		"showAll":   { "type": "boolean", "description": "Include hidden files (those starting with '.') (default false)." },
		"long":      { "type": "boolean", "description": "Show size, mtime, and mode columns (default true)." }
	}
}`

// ListFiles returns the list_files tool. It mimics `ls [-la]` and lists the
// contents of a directory, including hidden files and long-format details on
// request. It works the same on Windows and macOS/Linux.
func ListFiles() core.AgentTool {
	return core.AgentTool{
		Name:        "list_files",
		Label:       "List",
		Description: "List files in a directory (cross-platform `ls`). Supports showAll to include hidden files, and long format showing size/mtime/mode.",
		Parameters:  mustSchema(listFilesSchema),
		Execute:     executeListFiles,
	}
}

type listFilesArgs struct {
	Path    string `json:"path"`
	ShowAll bool   `json:"showAll"`
	Long    bool   `json:"long"`
}

func executeListFiles(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	var args listFilesArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return errResult("invalid arguments: " + err.Error()), nil
	}
	target := args.Path
	if target == "" {
		target = "."
	}
	safePath, err := resolveSafePath(target, "")
	if err != nil {
		return errResult(fmt.Sprintf("list_files: %v", err)), nil
	}

	info, err := os.Stat(safePath)
	if err != nil {
		return errResult(fmt.Sprintf("list_files: %v", err)), nil
	}
	if !info.IsDir() {
		return errResult(fmt.Sprintf("list_files: not a directory: %s", safePath)), nil
	}

	entries, err := os.ReadDir(safePath)
	if err != nil {
		return errResult(fmt.Sprintf("list_files: %v", err)), nil
	}

	type entry struct {
		name    string
		mode    os.FileMode
		size    int64
		modTime string
		isDir   bool
	}
	out := make([]entry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !args.ShowAll && strings.HasPrefix(name, ".") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, entry{
			name:    name,
			mode:    fi.Mode(),
			size:    fi.Size(),
			modTime: fi.ModTime().Format("2006-01-02 15:04"),
			isDir:   fi.IsDir(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })

	useLong := args.Long || len(out) > 0 // default ON

	var sb strings.Builder
	fmt.Fprintf(&sb, "Directory: %s\n", safePath)
	if len(out) == 0 {
		sb.WriteString("(empty)\n")
	} else {
		if useLong {
			sb.WriteString("mode                       size      modified            name\n")
			for _, e := range out {
				suffix := ""
				if e.isDir {
					suffix = "/"
				}
				fmt.Fprintf(&sb, "%-25s  %8d  %s  %s%s\n",
					e.mode.String(), e.size, e.modTime, e.name, suffix)
			}
		} else {
			for _, e := range out {
				suffix := ""
				if e.isDir {
					suffix = "/"
				}
				fmt.Fprintf(&sb, "%s%s\n", e.name, suffix)
			}
		}
	}
	details := detailsJSON(map[string]any{
		"path":  safePath,
		"count": len(out),
	})
	return core.AgentToolResult{
		Content: textBlock(sb.String()),
		Details: details,
	}, nil
}

// ─── change_dir (cd) ─────────────────────────────────────────────────────

const changeDirSchema = `{
	"type": "object",
	"properties": {
		"path": { "type": "string", "description": "Directory to switch into (absolute or relative to current working directory)." }
	},
	"required": ["path"]
}`

// ChangeDir returns the change_dir tool. It updates the session's working
// directory stored in the ExecutionEnv. On Unix it also calls chdir(2) so
// child processes (run via bash) inherit the new cwd.
func ChangeDir() core.AgentTool {
	return core.AgentTool{
		Name:        "change_dir",
		Label:       "Cd",
		Description: "Change the current working directory. The new directory is remembered for subsequent file operations in this session.",
		Parameters:  mustSchema(changeDirSchema),
		Execute:     executeChangeDir,
	}
}

type changeDirArgs struct {
	Path string `json:"path"`
}

func executeChangeDir(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	var args changeDirArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return errResult("invalid arguments: " + err.Error()), nil
	}
	if args.Path == "" {
		return errResult("path is required"), nil
	}
	safePath, err := resolveSafePath(args.Path, "")
	if err != nil {
		return errResult(fmt.Sprintf("change_dir: %v", err)), nil
	}
	info, err := os.Stat(safePath)
	if err != nil {
		return errResult(fmt.Sprintf("change_dir: %v", err)), nil
	}
	if !info.IsDir() {
		return errResult(fmt.Sprintf("change_dir: not a directory: %s", safePath)), nil
	}

	// Update ExecutionEnv's cwd if available.
	if execEnv := core.GetExecutionEnv(ctx); execEnv != nil {
		if err := execEnv.SetWorkingDir(safePath); err != nil {
			return errResult(fmt.Sprintf("change_dir: %v", err)), nil
		}
	}
	// Also update the OS-level cwd so spawned processes inherit it.
	_ = os.Chdir(safePath)

	details := detailsJSON(map[string]any{
		"previousDir": mustGetwd(),
		"currentDir":  safePath,
	})
	return core.AgentToolResult{
		Content: textBlock(fmt.Sprintf("Changed directory to %s", safePath)),
		Details: details,
	}, nil
}

// mustGetwd returns the current working directory, falling back to "." on error.
func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// ─── print_working_dir (pwd) ──────────────────────────────────────────────

const pwdSchema = `{
	"type": "object",
	"properties": {}
}`

// PrintWorkingDir returns the print_working_dir tool. It prints the absolute
// path of the current working directory.
func PrintWorkingDir() core.AgentTool {
	return core.AgentTool{
		Name:        "print_working_dir",
		Label:       "Pwd",
		Description: "Print the current working directory (cross-platform `pwd`).",
		Parameters:  mustSchema(pwdSchema),
		Execute:     executePwd,
	}
}

func executePwd(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	wd, err := os.Getwd()
	if err != nil {
		return errResult(fmt.Sprintf("print_working_dir: %v", err)), nil
	}
	return core.AgentToolResult{
		Content: textBlock(wd),
		Details: detailsJSON(map[string]any{"workingDir": wd}),
	}, nil
}

// ─── show_file (cat) ──────────────────────────────────────────────────────

const showFileSchema = `{
	"type": "object",
	"properties": {
		"filePath": { "type": "string", "description": "Path of the file to display." }
	},
	"required": ["filePath"]
}`

// ShowFile returns the show_file tool. It is a thin alias for read_file that
// matches the familiar "cat" / "type" command names users know.
func ShowFile() core.AgentTool {
	return core.AgentTool{
		Name:        "show_file",
		Label:       "Cat",
		Description: "Display the entire contents of a file (cross-platform `cat`/`type`). For large files prefer read_file with offset/limit.",
		Parameters:  mustSchema(showFileSchema),
		Execute:     executeShowFile,
	}
}

type showFileArgs struct {
	FilePath string `json:"filePath"`
}

func executeShowFile(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	var args showFileArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return errResult("invalid arguments: " + err.Error()), nil
	}
	if args.FilePath == "" {
		return errResult("filePath is required"), nil
	}
	safePath, err := resolveSafePath(args.FilePath, "")
	if err != nil {
		return errResult(fmt.Sprintf("show_file: %v", err)), nil
	}
	info, err := os.Stat(safePath)
	if err != nil {
		return errResult(fmt.Sprintf("show_file: %v", err)), nil
	}
	if info.IsDir() {
		return errResult(fmt.Sprintf("show_file: is a directory: %s", safePath)), nil
	}
	data, err := os.ReadFile(safePath)
	if err != nil {
		return errResult(fmt.Sprintf("show_file: %v", err)), nil
	}
	text := string(data)
	const maxOut = 200_000
	truncated := false
	if len(text) > maxOut {
		text = text[:maxOut] + "\n\n[... truncated ...]"
		truncated = true
	}
	return core.AgentToolResult{
		Content: textBlock(text),
		Details: detailsJSON(map[string]any{
			"filePath":  safePath,
			"bytes":     len(data),
			"truncated": truncated,
		}),
	}, nil
}

// ─── head / tail ──────────────────────────────────────────────────────────

const headTailSchema = `{
	"type": "object",
	"properties": {
		"filePath": { "type": "string", "description": "Path of the file." },
		"lines":    { "type": "integer", "description": "Number of lines to show (default 10)." }
	},
	"required": ["filePath"]
}`

// Head returns the head tool. It shows the first N lines of a file.
func Head() core.AgentTool {
	return core.AgentTool{
		Name:        "head",
		Label:       "Head",
		Description: "Show the first N lines of a file (cross-platform `head -n`).",
		Parameters:  mustSchema(headTailSchema),
		Execute:     executeHead,
	}
}

// Tail returns the tail tool. It shows the last N lines of a file.
func Tail() core.AgentTool {
	return core.AgentTool{
		Name:        "tail",
		Label:       "Tail",
		Description: "Show the last N lines of a file (cross-platform `tail -n`).",
		Parameters:  mustSchema(headTailSchema),
		Execute:     executeTail,
	}
}

type headTailArgs struct {
	FilePath string `json:"filePath"`
	Lines    int    `json:"lines"`
}

func executeHead(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	return headTailCommon(ctx, params, true)
}

func executeTail(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	return headTailCommon(ctx, params, false)
}

func headTailCommon(ctx context.Context, params json.RawMessage, isHead bool) (core.AgentToolResult, error) {
	var args headTailArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return errResult("invalid arguments: " + err.Error()), nil
	}
	if args.FilePath == "" {
		return errResult("filePath is required"), nil
	}
	if args.Lines <= 0 {
		args.Lines = 10
	}
	safePath, err := resolveSafePath(args.FilePath, "")
	if err != nil {
		return errResult(fmt.Sprintf("head/tail: %v", err)), nil
	}
	data, err := os.ReadFile(safePath)
	if err != nil {
		return errResult(fmt.Sprintf("head/tail: %v", err)), nil
	}
	// Use \n splitting consistent across platforms; data may contain
	// trailing newline which we tolerate.
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	// Drop trailing empty element caused by terminating newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var selected []string
	if isHead {
		end := args.Lines
		if end > len(lines) {
			end = len(lines)
		}
		selected = lines[:end]
	} else {
		start := len(lines) - args.Lines
		if start < 0 {
			start = 0
		}
		selected = lines[start:]
	}
	text := strings.Join(selected, "\n") + "\n"
	tool := "head"
	if !isHead {
		tool = "tail"
	}
	return core.AgentToolResult{
		Content: textBlock(text),
		Details: detailsJSON(map[string]any{
			"tool":     tool,
			"filePath": safePath,
			"lines":    len(selected),
			"total":    len(lines),
		}),
	}, nil
}

// ─── find_files (find) ────────────────────────────────────────────────────

const findFilesSchema = `{
	"type": "object",
	"properties": {
		"path":     { "type": "string", "description": "Directory to search in (default: current working directory)." },
		"name":     { "type": "string", "description": "Substring (case-insensitive) the file or directory name must contain." },
		"maxDepth": { "type": "integer", "description": "Maximum directory depth to descend (0 = unlimited)." },
		"limit":    { "type": "integer", "description": "Maximum number of results (default 200)." }
	}
}`

// FindFiles returns the find_files tool. It walks a directory and returns
// paths whose name matches the given substring. It mirrors the most common
// use-case of `find . -name "..."` without requiring shell quoting.
func FindFiles() core.AgentTool {
	return core.AgentTool{
		Name:        "find_files",
		Label:       "Find",
		Description: "Find files by name (substring match). Mirrors `find <path> -name \"*\"`. Returns paths relative to the search root.",
		Parameters:  mustSchema(findFilesSchema),
		Execute:     executeFindFiles,
	}
}

type findFilesArgs struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	MaxDepth int    `json:"maxDepth"`
	Limit    int    `json:"limit"`
}

func executeFindFiles(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	var args findFilesArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return errResult("invalid arguments: " + err.Error()), nil
	}
	target := args.Path
	if target == "" {
		target = "."
	}
	safePath, err := resolveSafePath(target, "")
	if err != nil {
		return errResult(fmt.Sprintf("find_files: %v", err)), nil
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 200
	}
	needle := strings.ToLower(args.Name)

	var matches []string
	_ = filepath.Walk(safePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if args.MaxDepth > 0 {
			rel, rerr := filepath.Rel(safePath, path)
			if rerr == nil {
				depth := strings.Count(rel, string(filepath.Separator))
				if info.IsDir() {
					depth++
				}
				if depth > args.MaxDepth {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
		}
		if needle == "" || strings.Contains(strings.ToLower(info.Name()), needle) {
			matches = append(matches, path)
		}
		if len(matches) >= limit {
			return filepath.SkipAll
		}
		return nil
	})

	truncated := len(matches) >= limit
	var sb strings.Builder
	if len(matches) == 0 {
		sb.WriteString("(no matches)\n")
	} else {
		for _, m := range matches {
			sb.WriteString(m)
			sb.WriteString("\n")
		}
	}
	if truncated {
		fmt.Fprintf(&sb, "\n[... truncated to %d results ...]", limit)
	}
	return core.AgentToolResult{
		Content: textBlock(sb.String()),
		Details: detailsJSON(map[string]any{
			"path":      safePath,
			"name":      args.Name,
			"count":     len(matches),
			"truncated": truncated,
		}),
	}, nil
}

// ─── OS hint helper ───────────────────────────────────────────────────────

// osHint is a tiny helper used only by tooling/UX code that wants to
// branch on the host OS in a portable way.
func osHint() string {
	if runtime.GOOS == "windows" {
		return "windows"
	}
	if runtime.GOOS == "darwin" {
		return "macos"
	}
	return "linux"
}
