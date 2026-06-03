package agent

import (
	"context"
	"errors"
	"testing"

	core "pi-ai-go/core"
)

func TestToolCallBlockedError(t *testing.T) {
	e := &ToolCallBlockedError{ToolCallID: "c1", ToolName: "rm", Reason: "permission denied"}
	if !errors.Is(e, ErrToolCallBlocked) {
		t.Error("expected errors.Is(ErrToolCallBlocked)")
	}
	if e.Error() == "" {
		t.Error("expected non-empty message")
	}
}

func TestOverflowSignalUnwraps(t *testing.T) {
	orig := &core.OverflowError{Provider: "openai", Message: "context length exceeded"}
	sig := &OverflowSignal{Provider: "openai", ModelID: "gpt-4o", OriginalErr: orig}
	if !errors.Is(sig, ErrOverflow) {
		t.Error("expected errors.Is(ErrOverflow)")
	}
	if !errors.Is(sig, core.ErrOverflow) {
		t.Error("expected errors.Is(core.ErrOverflow)")
	}
	if sig.Error() == "" {
		t.Error("expected non-empty message")
	}
}

func TestAbortError(t *testing.T) {
	ae := &AbortError{Cause: context.Canceled}
	if !errors.Is(ae, ErrAgentAborted) {
		t.Error("expected errors.Is(ErrAgentAborted)")
	}
	if !errors.Is(ae, core.ErrAborted) {
		t.Error("expected errors.Is(core.ErrAborted)")
	}
}

func TestToolNotFoundError(t *testing.T) {
	e := &ToolNotFoundError{ToolName: "bogus", ToolCallID: "c1"}
	if !errors.Is(e, ErrToolNotFound) {
		t.Error("expected errors.Is(ErrToolNotFound)")
	}
}
