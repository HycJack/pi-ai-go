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

	safePath, err := resolveSafePath(args.FilePath, "")
	if err != nil {
		return errResult(fmt.Sprintf("edit_file: %v", err)), nil
	}

	execEnv := core.GetExecutionEnv(ctx)
	var data []byte
	if execEnv != nil {
		data, err = execEnv.ReadFile(safePath)
	} else {
		data, err = os.ReadFile(safePath)
	}
	if err != nil {
		return errResult(fmt.Sprintf("edit_file: %v", err)), nil
	}
	src := string(data)
	count := strings.Count(src, args.OldText)
	if count == 0 {
		return errResult(fmt.Sprintf("edit_file: oldText not found in %s", safePath)), nil
	}

	var out string
	replacements := 1
	if args.AllOccurrences {
		out = strings.ReplaceAll(src, args.OldText, args.NewText)
		replacements = count
	} else {
		// Default: replace only the first occurrence, even when there
		// are multiple matches. The result message notes how many
		// other matches were left untouched.
		out = strings.Replace(src, args.OldText, args.NewText, 1)
		replacements = 1
	}

	if execEnv != nil {
		if err := execEnv.WriteFile(safePath, []byte(out), 0o644); err != nil {
			return errResult(fmt.Sprintf("edit_file: %v", err)), nil
		}
	} else {
		if err := os.WriteFile(safePath, []byte(out), 0o644); err != nil {
			return errResult(fmt.Sprintf("edit_file: %v", err)), nil
		}
	}

	details, _ := json.Marshal(map[string]any{
		"filePath":     safePath,
		"replacements": replacements,
		"matchesLeft":  count - replacements,
		"bytes":        len(out),
	})
	msg := fmt.Sprintf("Edited %s (%d replacement(s))", safePath, replacements)
	if !args.AllOccurrences && count > 1 {
		msg += fmt.Sprintf("; %d more match(es) left — use allOccurrences=true to replace them", count-1)
	}
	return core.AgentToolResult{
		Content: textBlock(msg),
		Details: details,
	}, nil
}
