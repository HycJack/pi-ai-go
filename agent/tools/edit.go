package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	core "pi-ai-go/core"
)

const editSchema = `{
	"type": "object",
	"properties": {
		"filePath":       { "type": "string", "description": "Path of the file to edit." },
		"oldText":        { "type": "string", "description": "Exact substring to find." },
		"newText":        { "type": "string", "description": "Replacement text." },
		"allOccurrences": { "type": "boolean", "description": "Replace every match (default: only the first one).", "default": false }
	},
	"required": ["filePath", "oldText", "newText"]
}`

// Edit returns the edit_file tool. It performs a string replacement
// on a file. By default only the first occurrence is replaced; set
// allOccurrences=true to replace every match.
func Edit() core.AgentTool {
	return core.AgentTool{
		Name:        "edit_file",
		Label:       "Edit",
		Description: "Replace oldText with newText in a file. Default: first match only. allOccurrences=true replaces every match.",
		Parameters:  mustSchema(editSchema),
		Execute:     executeEdit,
	}
}

type editArgs struct {
	FilePath       string `json:"filePath"`
	OldText        string `json:"oldText"`
	NewText        string `json:"newText"`
	AllOccurrences bool   `json:"allOccurrences"`
}

func executeEdit(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	var args editArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return errResult("invalid arguments: " + err.Error()), nil
	}
	if args.FilePath == "" || args.OldText == "" {
		return errResult("filePath and oldText are required"), nil
	}

	data, err := os.ReadFile(args.FilePath)
	if err != nil {
		return errResult(fmt.Sprintf("edit_file: %v", err)), nil
	}
	src := string(data)
	count := strings.Count(src, args.OldText)
	if count == 0 {
		return errResult(fmt.Sprintf("edit_file: oldText not found in %s", args.FilePath)), nil
	}
	if !args.AllOccurrences && count > 1 {
		return errResult(fmt.Sprintf("edit_file: oldText matches %d times in %s; pass allOccurrences=true to replace all", count, args.FilePath)), nil
	}

	var out string
	replacements := 1
	if args.AllOccurrences {
		out = strings.ReplaceAll(src, args.OldText, args.NewText)
		replacements = count
	} else {
		out = strings.Replace(src, args.OldText, args.NewText, 1)
	}

	if err := os.WriteFile(args.FilePath, []byte(out), 0o644); err != nil {
		return errResult(fmt.Sprintf("edit_file: %v", err)), nil
	}

	details, _ := json.Marshal(map[string]any{
		"filePath":     args.FilePath,
		"replacements": replacements,
		"bytes":        len(out),
	})
	return core.AgentToolResult{
		Content: textBlock(fmt.Sprintf("Edited %s (%d replacement(s))", args.FilePath, replacements)),
		Details: details,
	}, nil
}
