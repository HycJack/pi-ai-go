package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	core "pi-ai-go/core"
)

const bashSchema = `{
	"type": "object",
	"properties": {
		"command": { "type": "string", "description": "Shell command to run." },
		"timeout": { "type": "integer", "description": "Maximum runtime in milliseconds (default 30000)." },
		"shell":   { "type": "string", "description": "Override the shell binary. 'auto' picks cmd.exe on Windows and sh elsewhere.", "enum": ["auto", "cmd", "powershell", "bash", "sh"] }
	},
	"required": ["command"]
}`

// DefaultBashTimeout is the default max-runtime for a single bash call.
const DefaultBashTimeout = 30 * time.Second

// Bash returns the bash tool. It runs a shell command and returns
// (stdout, stderr, exitCode). On Windows the default shell is
// cmd.exe; on macOS/Linux it is sh.
func Bash() core.AgentTool {
	return core.AgentTool{
		Name:        "bash",
		Label:       "Bash",
		Description: "Run a shell command and return stdout, stderr, and exit code.",
		Parameters:  mustSchema(bashSchema),
		Execute:     executeBash,
	}
}

type bashArgs struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"` // ms
	Shell   string `json:"shell"`
}

func executeBash(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	var args bashArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return errResult("invalid arguments: " + err.Error()), nil
	}
	if args.Command == "" {
		return errResult("command is required"), nil
	}
	timeout := DefaultBashTimeout
	if args.Timeout > 0 {
		timeout = time.Duration(args.Timeout) * time.Millisecond
	}

	shell, shellArgs := pickShell(args.Shell, runtime.GOOS)

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, shell, shellArgs...)
	cmd.Args = append(cmd.Args, args.Command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			return errResult(fmt.Sprintf("bash: %v", err)), nil
		}
	}

	stdoutStr := stdout.String()
	stderrStr := stderr.String()
	combined := stdoutStr
	if stderrStr != "" {
		if combined != "" {
			combined += "\n"
		}
		combined += "[stderr]\n" + stderrStr
	}
	// Cap output to prevent context overflow.
	const maxOut = 100_000
	truncated := false
	if len(combined) > maxOut {
		combined = combined[:maxOut] + "\n[... truncated ...]"
		truncated = true
	}

	isErr := exitCode != 0
	details, _ := json.Marshal(map[string]any{
		"command":   args.Command,
		"exitCode":  exitCode,
		"shell":     shell,
		"truncated": truncated,
		"timedOut":  runCtx.Err() == context.DeadlineExceeded,
	})
	return core.AgentToolResult{
		Content: textBlock(combined),
		Details: details,
		IsError: isErr,
	}, nil
}

// pickShell chooses the shell binary and its prefix arguments.
//
// "auto" (default) maps to:
//   - "cmd" + ["/c"] on Windows
//   - "sh"  + ["-c"]  on macOS / Linux
//
// Named shells:
//   - "cmd"        -> cmd.exe /c <command>
//   - "powershell" -> powershell -NoProfile -Command <command>
//   - "bash"       -> bash -c <command>
//   - "sh"         -> sh -c <command>
func pickShell(name, goos string) (binary string, prefix []string) {
	if name == "" || name == "auto" {
		if goos == "windows" {
			return "cmd.exe", []string{"/c"}
		}
		return "sh", []string{"-c"}
	}
	switch name {
	case "cmd":
		return "cmd.exe", []string{"/c"}
	case "powershell":
		return "powershell", []string{"-NoProfile", "-Command"}
	case "bash":
		return "bash", []string{"-c"}
	case "sh":
		return "sh", []string{"-c"}
	}
	// Fallback: treat the value as a binary name and pass -c / /c.
	if goos == "windows" {
		return name, []string{"/c"}
	}
	return name, []string{"-c"}
}
