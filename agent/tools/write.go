package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	core "pi-ai-go/core"
)

const writeSchema = `{
	"type": "object",
	"properties": {
		"filePath": { "type": "string", "description": "Path of the file to write. Parent directories are created if missing." },
		"content":  { "type": "string", "description": "Full content to write to the file." }
	},
	"required": ["filePath", "content"]
}`

// Write returns the write_file tool. It creates parent directories as
// needed and overwrites the destination if it exists.
func Write() core.AgentTool {
	return core.AgentTool{
		Name:        "write_file",
		Label:       "Write",
		Description: "Create or overwrite a file with the given content. Creates parent directories.",
		Parameters:  mustSchema(writeSchema),
		Execute:     executeWrite,
	}
}

type writeArgs struct {
	FilePath string `json:"filePath"`
	Content  string `json:"content"`
}

func executeWrite(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	var args writeArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return errResult("invalid arguments: " + err.Error()), nil
	}
	if args.FilePath == "" {
		return errResult("filePath is required"), nil
	}

	// Create parent directories.
	dir := filepath.Dir(args.FilePath)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return errResult(fmt.Sprintf("write_file: mkdir: %v", err)), nil
		}
	}

	// Write atomically: write to a temp file, then rename.
	tmp := args.FilePath + ".tmp"
	if err := os.WriteFile(tmp, []byte(args.Content), 0o644); err != nil {
		return errResult(fmt.Sprintf("write_file: %v", err)), nil
	}
	if err := os.Rename(tmp, args.FilePath); err != nil {
		_ = os.Remove(tmp)
		return errResult(fmt.Sprintf("write_file: rename: %v", err)), nil
	}

	details, _ := json.Marshal(map[string]any{
		"filePath": args.FilePath,
		"bytes":    len(args.Content),
	})
	return core.AgentToolResult{
		Content: textBlock(fmt.Sprintf("Wrote %d bytes to %s", len(args.Content), args.FilePath)),
		Details: details,
	}, nil
}
