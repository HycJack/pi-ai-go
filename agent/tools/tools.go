// Package tools provides the built-in tools the pi-ai-go agent can
// call out of the box. The set mirrors pi-mono / oh-my-pi:
//
//   - read_file  - read file contents (optionally with a line range)
//   - write_file - create or overwrite a file
//   - edit_file  - str_replace / patch (single or all occurrences)
//   - bash       - run a shell command (Windows cmd.exe by default)
//   - glob       - list files matching a glob pattern
//   - grep       - search file contents (substring or regex)
//
// All tools accept JSON parameters matching the schema declared in
// their Parameters field and return []core.ContentBlock results.
package tools

import (
	"encoding/json"
	"fmt"

	"pi-ai-go/agent"
	core "pi-ai-go/core"
)

// mustSchema returns a json.RawMessage for the given literal. Panics
// on invalid JSON (caller error).
func mustSchema(s string) json.RawMessage {
	if !json.Valid([]byte(s)) {
		panic(fmt.Sprintf("tools: invalid schema literal: %s", s))
	}
	return json.RawMessage(s)
}

// textBlock is a tiny constructor for a single text content block.
func textBlock(s string) []core.ContentBlock {
	return []core.ContentBlock{core.TextContent{Type: "text", Text: s}}
}

// All returns the canonical built-in tool set. Callers can spread this
// into an agent.AgentLoopConfig.Tools slice in one go.
func All() []agent.AgentTool {
	return []agent.AgentTool{
		Read(),
		Write(),
		Edit(),
		Bash(),
		Glob(),
		Grep(),
	}
}
