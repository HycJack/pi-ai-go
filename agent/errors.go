package agent

import (
	"errors"
	"fmt"

	core "pi-ai-go/core"
)

// Sentinel error reasons used by the agent loop. The aim is for callers to
// be able to discriminate failures with errors.Is / errors.As instead of
// string matching — mirroring oh-my-pi's `ToolCallBlockedError` and
// `HarmonyLeakInterruption` patterns.
var (
	// ErrAgentAborted signals the user cancelled the agent run.
	ErrAgentAborted = errors.New("agent aborted")

	// ErrOverflow is re-exported from core so the agent package does not
	// force every caller to import core for the error check.
	ErrOverflow = core.ErrOverflow

	// ErrToolCallBlocked is the marker for blocked tool calls.
	ErrToolCallBlocked = errors.New("tool call blocked")

	// ErrToolNotFound is returned when the LLM hallucinates a tool name.
	ErrToolNotFound = errors.New("tool not found")

	// ErrToolExecFailure is the generic tool-execution failure marker.
	ErrToolExecFailure = errors.New("tool execution failed")
)

// ToolCallBlockedError is the typed error returned by the BeforeToolCall
// hook when it wants to stop a tool call from running. It is a sentinel
// so callers can use errors.As to retrieve the per-call Reason.
//
// Why a typed error rather than just an *ToolCallBlock return value: a
// persistent session may re-serialize the failed tool result to disk and
// replay it across agent runs. A typed error flows naturally through the
// existing error-propagation path, including typed unwraps.
type ToolCallBlockedError struct {
	ToolCallID string
	ToolName   string
	Reason     string
}

func (e *ToolCallBlockedError) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("tool call blocked: %s (id=%s)", e.ToolName, e.ToolCallID)
	}
	return fmt.Sprintf("tool call blocked: %s (id=%s): %s", e.ToolName, e.ToolCallID, e.Reason)
}

func (e *ToolCallBlockedError) Is(target error) bool {
	return target == ErrToolCallBlocked
}

// OverflowSignal is the typed signal raised when the LLM stream returns
// an overflow (or the agent detects silent overflow via usage). It
// surfaces the provider, the context window and the usage at the moment
// the overflow was detected so compaction logic can size its target.
type OverflowSignal struct {
	Provider       core.KnownProvider
	ModelID        string
	ContextWindow  int
	Usage          int
	OriginalErr    error
	Source         string // "stream" or "silent"
}

func (e *OverflowSignal) Error() string {
	if e.OriginalErr != nil {
		return fmt.Sprintf("%s/%s: context overflow (%s): %v", e.Provider, e.ModelID, e.Source, e.OriginalErr)
	}
	return fmt.Sprintf("%s/%s: context overflow (%s, usage %d / window %d)", e.Provider, e.ModelID, e.Source, e.Usage, e.ContextWindow)
}

func (e *OverflowSignal) Is(target error) bool {
	return target == ErrOverflow
}

func (e *OverflowSignal) Unwrap() error { return e.OriginalErr }

// ToolNotFoundError is the typed signal for hallucinated tool names.
type ToolNotFoundError struct {
	ToolName   string
	ToolCallID string
}

func (e *ToolNotFoundError) Error() string {
	return fmt.Sprintf("tool not found: %s (id=%s)", e.ToolName, e.ToolCallID)
}

func (e *ToolNotFoundError) Is(target error) bool {
	return target == ErrToolNotFound
}

// AbortError signals the user cancelled mid-run. It is distinct from
// context.Canceled so callers can ask "was the agent aborted?" without
// false positives from internal cancellation.
type AbortError struct {
	Cause error
}

func (e *AbortError) Error() string {
	if e.Cause == nil {
		return "agent aborted"
	}
	return "agent aborted: " + e.Cause.Error()
}

func (e *AbortError) Unwrap() error { return e.Cause }
func (e *AbortError) Is(t error) bool {
	return t == ErrAgentAborted || t == core.ErrAborted
}
