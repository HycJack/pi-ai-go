package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	core "pi-ai-go/core"
)

const appendSchema = `{
	"type": "object",
	"properties": {
		"filePath": { "type": "string", "description": "Path of the file to append to. Parent directories are created if missing; the file is created if it does not exist." },
		"content":  { "type": "string", "description": "Content to append to the file." }
	},
	"required": ["filePath", "content"]
}`

// Append returns the append_file tool. It appends content to the end of a file
// (creating it if it does not exist). Use this for incremental generation of
// large files where the model cannot emit the entire content in a single call.
func Append() core.AgentTool {
	return core.AgentTool{
		Name:        "append_file",
		Label:       "Append",
		Description: "Append content to the end of a file. Creates the file and parent directories if they do not exist. Use this to incrementally build large files across multiple calls.",
		Parameters:  mustSchema(appendSchema),
		Execute:     executeAppend,
	}
}

type appendArgs struct {
	FilePath string `json:"filePath"`
	Content  string `json:"content"`
}

func executeAppend(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	var args appendArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return errResult("invalid arguments: " + err.Error()), nil
	}
	if args.FilePath == "" {
		return errResult("filePath is required"), nil
	}

	safePath, err := resolveSafePath(args.FilePath, "")
	if err != nil {
		return errResult(fmt.Sprintf("append_file: %v", err)), nil
	}

	execEnv := core.GetExecutionEnv(ctx)

	// Create parent directories.
	dir := filepath.Dir(safePath)
	if dir != "" {
		if execEnv != nil {
			if err := execEnv.Mkdir(dir, 0o755); err != nil {
				return errResult(fmt.Sprintf("append_file: mkdir: %v", err)), nil
			}
		} else {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return errResult(fmt.Sprintf("append_file: mkdir: %v", err)), nil
			}
		}
	}

	// Open in append/create mode. O_APPEND guarantees atomic writes at the
	// OS level relative to other writers; O_CREATE creates the file if
	// missing. Mode 0644 only applies when creating.
	f, err := os.OpenFile(safePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return errResult(fmt.Sprintf("append_file: open: %v", err)), nil
	}
	defer f.Close()

	if _, err := f.WriteString(args.Content); err != nil {
		return errResult(fmt.Sprintf("append_file: write: %v", err)), nil
	}

	// Report file size after the append so the model can verify how much
	// has been written and decide whether to continue.
	st, _ := os.Stat(safePath)
	totalBytes := int64(0)
	if st != nil {
		totalBytes = st.Size()
	}

	details, _ := json.Marshal(map[string]any{
		"filePath":    safePath,
		"appended":    len(args.Content),
		"totalBytes":  totalBytes,
	})
	return core.AgentToolResult{
		Content: textBlock(fmt.Sprintf("Appended %d bytes to %s (now %d bytes total)", len(args.Content), safePath, totalBytes)),
		Details: details,
	}, nil
}