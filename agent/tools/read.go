package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	core "pi-ai-go/core"
)

const readSchema = `{
	"type": "object",
	"properties": {
		"filePath": { "type": "string", "description": "Absolute or working-directory-relative path of the file to read." },
		"offset":   { "type": "integer", "description": "0-based line offset to start reading at (optional)." },
		"limit":    { "type": "integer", "description": "Maximum number of lines to return (optional)." }
	},
	"required": ["filePath"]
}`

// Read returns the read_file tool. It reads the file at filePath and
// returns its content. If offset and limit are set, only the
// corresponding slice of lines is returned.
func Read() core.AgentTool {
	return core.AgentTool{
		Name:        "read_file",
		Label:       "Read",
		Description: "Read the contents of a file. Optionally limit by offset/line.",
		Parameters:  mustSchema(readSchema),
		Execute:     executeRead,
	}
}

type readArgs struct {
	FilePath string `json:"filePath"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

func executeRead(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	var args readArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return errResult("invalid arguments: " + err.Error()), nil
	}
	if args.FilePath == "" {
		return errResult("filePath is required"), nil
	}

	safePath, err := resolveSafePath(args.FilePath, "")
	if err != nil {
		return errResult(fmt.Sprintf("read_file: %v", err)), nil
	}

	data, err := os.ReadFile(safePath)
	if err != nil {
		return errResult(fmt.Sprintf("read_file: %v", err)), nil
	}

	text := string(data)
	if args.Offset > 0 || args.Limit > 0 {
		lines := strings.Split(text, "\n")
		start := args.Offset
		if start < 0 {
			start = 0
		}
		if start > len(lines) {
			start = len(lines)
		}
		end := len(lines)
		if args.Limit > 0 && start+args.Limit < end {
			end = start + args.Limit
		}
		text = strings.Join(lines[start:end], "\n")
	}

	// Trim very long outputs to prevent context overflow.
	const maxReadChars = 200_000
	truncated := false
	if len(text) > maxReadChars {
		text = text[:maxReadChars] + "\n\n[... truncated ...]"
		truncated = true
	}

	details := map[string]any{
		"filePath":  safePath,
		"bytes":     len(data),
		"offset":    args.Offset,
		"limit":     args.Limit,
		"truncated": truncated,
	}
	detailJSON, _ := json.Marshal(details)
	return core.AgentToolResult{
		Content: textBlock(text),
		Details: detailJSON,
	}, nil
}

// errResult is a tiny helper for the canonical error result shape.
func errResult(msg string) core.AgentToolResult {
	return core.AgentToolResult{
		Content: textBlock(msg),
		IsError: true,
	}
}

// avoid unused-import noise on core (used by textBlock return type).
var _ = core.TextContent{}
