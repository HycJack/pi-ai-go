package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"pi-ai-go/agent"
)

const grepSchema = `{
	"type": "object",
	"properties": {
		"pattern":  { "type": "string", "description": "Substring or regex (regex=true). Case-insensitive by default." },
		"include":  { "type": "string", "description": "Glob to filter file paths, e.g. '*.go' (optional)." },
		"basePath": { "type": "string", "description": "Directory to search (default: current working directory)." },
		"regex":    { "type": "boolean", "description": "Interpret pattern as a regular expression (default false).", "default": false },
		"limit":    { "type": "integer", "description": "Maximum number of matches to return (default 200)." }
	},
	"required": ["pattern"]
}`

// Grep returns the grep tool. It searches for a substring (or regex)
// in the contents of files under basePath. Results are formatted
// "file:line: text".
func Grep() agent.AgentTool {
	return agent.AgentTool{
		Name:        "grep",
		Label:       "Grep",
		Description: "Search file contents. Returns file:line hits. Supports substring and regex modes.",
		Parameters:  mustSchema(grepSchema),
		Execute:     executeGrep,
	}
}

type grepArgs struct {
	Pattern  string `json:"pattern"`
	Include  string `json:"include"`
	BasePath string `json:"basePath"`
	Regex    bool   `json:"regex"`
	Limit    int    `json:"limit"`
}

func executeGrep(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
	var args grepArgs
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
	limit := args.Limit
	if limit <= 0 {
		limit = 200
	}

	var matcher func(line string) bool
	if args.Regex {
		re, err := regexp.Compile("(?i)" + args.Pattern)
		if err != nil {
			return errResult(fmt.Sprintf("grep: bad regex: %v", err)), nil
		}
		matcher = func(line string) bool { return re.MatchString(line) }
	} else {
		needle := strings.ToLower(args.Pattern)
		matcher = func(line string) bool { return strings.Contains(strings.ToLower(line), needle) }
	}

	var includeGlob func(path string) bool
	if args.Include != "" {
		pattern := filepath.FromSlash(args.Include)
		includeGlob = func(path string) bool {
			ok, _ := filepath.Match(pattern, filepath.Base(path))
			return ok
		}
	}

	type hit struct {
		path string
		line int
		text string
	}
	var hits []hit
	truncated := false
	walkErr := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if includeGlob != nil && !includeGlob(path) {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			text := scanner.Text()
			if !matcher(text) {
				continue
			}
			if len(hits) >= limit {
				truncated = true
				return filepath.SkipAll
			}
			hits = append(hits, hit{path: path, line: lineNo, text: text})
		}
		return nil
	})
	if walkErr != nil {
		return errResult(fmt.Sprintf("grep: %v", walkErr)), nil
	}

	var sb strings.Builder
	if len(hits) == 0 {
		sb.WriteString("(no matches)\n")
	} else {
		for _, h := range hits {
			fmt.Fprintf(&sb, "%s:%d: %s\n", h.path, h.line, h.text)
		}
	}
	if truncated {
		fmt.Fprintf(&sb, "\n[... truncated to %d results ...]", limit)
	}

	details, _ := json.Marshal(map[string]any{
		"pattern":   args.Pattern,
		"basePath":  base,
		"count":     len(hits),
		"truncated": truncated,
	})
	return agent.AgentToolResult{
		Content: textBlock(sb.String()),
		Details: details,
	}, nil
}
