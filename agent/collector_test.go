package agent

import (
	"errors"
	"testing"

	core "pi-ai-go/core"
)

func TestRunCollectorBasic(t *testing.T) {
	r := NewRunCollector()
	r.RecordStep(core.Model{Provider: "openai", ID: "gpt-4o"})
	r.RecordStep(core.Model{Provider: "openai", ID: "gpt-4o"})
	r.RecordToolCall("bash", false)
	r.RecordToolCall("bash", true)
	r.RecordUsage(core.Usage{Input: 100, Output: 50, Cost: core.CostBreakdown{Total: 0.001}})
	r.RecordUsage(core.Usage{Input: 200, Output: 100, Cost: core.CostBreakdown{Total: 0.002}})
	r.SetStopReason(core.StopStop)
	r.MarkRunEnded()

	sum, cov := r.Snapshot()
	if sum.StepCount != 2 {
		t.Errorf("StepCount = %d", sum.StepCount)
	}
	if sum.ToolCallCount != 2 {
		t.Errorf("ToolCallCount = %d", sum.ToolCallCount)
	}
	if sum.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d", sum.ErrorCount)
	}
	if sum.TotalUsage.Input != 300 || sum.TotalUsage.Output != 150 {
		t.Errorf("TotalUsage = %+v", sum.TotalUsage)
	}
	if sum.TotalCost < 0.0029 || sum.TotalCost > 0.0031 {
		t.Errorf("TotalCost = %f", sum.TotalCost)
	}
	if sum.Providers["openai"] != 2 {
		t.Errorf("Providers = %v", sum.Providers)
	}
	if sum.StopReasonFinal != core.StopStop {
		t.Errorf("StopReasonFinal = %s", sum.StopReasonFinal)
	}
	if cov.ToolsByName["bash"] != 2 {
		t.Errorf("ToolsByName = %v", cov.ToolsByName)
	}
}

func TestClassifyError(t *testing.T) {
	r := NewRunCollector()
	r.RecordError(&core.AuthError{Provider: "openai"})
	r.RecordError(&core.RateLimitError{Provider: "openai"})
	r.RecordError(&core.ServerError{Provider: "openai", StatusCode: 500})
	r.RecordError(&core.OverflowError{Provider: "openai"})
	r.RecordError(&ToolCallBlockedError{ToolName: "rm"})
	r.RecordError(&ToolNotFoundError{ToolName: "x"})
	r.RecordError(&AbortError{})
	r.RecordError(errors.New("unknown"))

	sum, _ := r.Snapshot()
	want := map[string]int{
		"auth": 1, "rate_limit": 1, "server": 1, "overflow": 1,
		"tool_blocked": 1, "tool_not_found": 1, "abort": 1, "other": 1,
	}
	for k, v := range want {
		if sum.ErrorsByKind[k] != v {
			t.Errorf("ErrorsByKind[%s] = %d, want %d", k, sum.ErrorsByKind[k], v)
		}
	}
}
