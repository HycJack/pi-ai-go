package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	core "pi-ai-go/core"
)

const globSchema = `{
	"type": "object",
	"properties": {
		"pattern":  { "type": "string", "description": "Glob pattern, e.g. '**/*.go' or 'src/*.txt'." },
		"basePath": { "type": "string", "description": "Directory to start in (default: current working directory)." },
		"limit":    { "type": "integer", "description": "Maximum number of results (default 500)." }
	},
	"required": ["pattern"]
}`

// Glob returns the glob tool. It expands the pattern and returns a
// sorted list of matching paths.
func Glob() core.AgentTool {
	return core.AgentTool{
		Name:        "glob",
		Label:       "Glob",
		Description: "List files matching a glob pattern (e.g. **/*.go).",
		Parameters:  mustSchema(globSchema),
		Execute:     executeGlob,
	}
}

type globArgs struct {
	Pattern  string `json:"pattern"`
	BasePath string `json:"basePath"`
	Limit    int    `json:"limit"`
}

func executeGlob(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	var args globArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return errResult("invalid arguments: " + err.Error()), nil
	}
	if args.Pattern == "" {
		return errResult("pattern is required"), nil
	}
	base := args.BasePath
	if base == "" {
		base, _ = os.Getwd()
	}

	// Convert forward-slash patterns to native separators and split
	// into a base and the glob portion.
	pattern := filepath.FromSlash(args.Pattern)
	pattern = strings.TrimPrefix(pattern, string(filepath.Separator))

	var matches []string
	if strings.Contains(pattern, "**") {
		// Walk-based glob: find the longest non-wildcard prefix and
		// walk from there.
		matches = walkGlob(base, pattern)
	} else {
		full := filepath.Join(base, pattern)
		ms, err := filepath.Glob(full)
		if err != nil {
			return errResult(fmt.Sprintf("glob: %v", err)), nil
		}
		matches = ms
	}

	sort.Strings(matches)
	limit := args.Limit
	if limit <= 0 {
		limit = 500
	}
	truncated := false
	if len(matches) > limit {
		matches = matches[:limit]
		truncated = true
	}

	var sb strings.Builder
	if len(matches) == 0 {
		sb.WriteString("(no matches)\n")
	} else {
		for _, p := range matches {
			sb.WriteString(p)
			sb.WriteString("\n")
		}
	}
	if truncated {
		fmt.Fprintf(&sb, "\n[... truncated to %d results ...]", limit)
	}

	details, _ := json.Marshal(map[string]any{
		"pattern":   args.Pattern,
		"basePath":  base,
		"count":     len(matches),
		"truncated": truncated,
	})
	return core.AgentToolResult{
		Content: textBlock(sb.String()),
		Details: details,
	}, nil
}

// walkGlob walks a directory tree matching a pattern that contains
// "**". It splits the pattern at the first "**" and walks from the
// non-wildcard root, then filters by the remaining pattern.
func walkGlob(base, pattern string) []string {
	// Find the directory portion up to the first "**".
	parts := strings.SplitN(pattern, "**", 2)
	rootPart := parts[0]
	rest := ""
	if len(parts) == 2 {
		rest = "**" + parts[1]
	}
	root := filepath.Join(base, rootPart)
	var out []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		// Build a path relative to `base` so the pattern matches.
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		ok, _ := filepath.Match(rest, rel)
		if !ok {
			// Also try matching the leaf name (so '**/*.go' matches 'foo.go').
			ok, _ = filepath.Match(strings.TrimPrefix(rest, "**/"), filepath.Base(rel))
		}
		if ok {
			out = append(out, path)
		}
		return nil
	})
	return out
}
