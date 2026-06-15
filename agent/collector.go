package agent

import (
	"sync"
	"sync/atomic"
	"time"

	core "pi-ai-go/core"
)

// AgentRunSummary is a per-run rollup returned alongside the messages.
// It mirrors oh-my-pi's `AgentRunSummary` interface so consumers can
// show usage totals in a TUI/CLI without re-parsing the event stream.
type AgentRunSummary struct {
	StepCount       int             // Total LLM turns (each agent_loop iteration that hits an LLM).
	ToolCallCount   int             // Total tool calls executed.
	ErrorCount      int             // Total tool calls / LLM calls that errored.
	StartedAt       time.Time       // Wall-clock run start.
	EndedAt         time.Time       // Wall-clock run end.
	Duration        time.Duration   // EndedAt - StartedAt.
	TotalUsage      core.Usage      // Aggregate input/output/cacheRead/cacheWrite.
	TotalCost       float64         // Aggregate cost.
	ErrorsByKind    map[string]int  // Bucket counts for errors: "auth" / "rate_limit" / "server" / "overflow" / "tool" / "abort" / "other".
	Providers       map[string]int  // Provider hit count (StepCount, plus 1 per step).
	StopReasonFinal core.StopReason // Final assistant StopReason (last one emitted).
}

// AgentRunCoverage is per-provider / per-model coverage stats. Used by
// harness-style consumers to log what the run touched.
type AgentRunCoverage struct {
	ProviderHits map[string]int // provider id -> step count
	ModelHits    map[string]int // model id -> step count
	ToolsByName  map[string]int // tool name -> call count
	TotalUsage   core.Usage
	TotalCost    float64
}

// RunCollector accumulates per-run stats in a lock-free / mostly-atomic
// way. Use it from the agent loop to populate AgentRunSummary and
// AgentRunCoverage at agent_end time.
type RunCollector struct {
	startedAt time.Time
	endedAt   atomic.Int64 // unix-nano; 0 = not ended yet
	mu        sync.Mutex

	stepCount    atomic.Int32
	toolCount    atomic.Int32
	errorCount   atomic.Int32
	errorsByKind map[string]int
	providerHits map[string]int
	modelHits    map[string]int
	toolsByName  map[string]int
	usage        core.Usage
	cost         atomic.Uint64 // float64 bits

	stopReasonFinal atomic.Uint32
}

// NewRunCollector creates a collector with a startedAt timestamp.
func NewRunCollector() *RunCollector {
	return &RunCollector{
		startedAt:    time.Now(),
		errorsByKind: make(map[string]int),
		providerHits: make(map[string]int),
		modelHits:    make(map[string]int),
		toolsByName:  make(map[string]int),
	}
}

// RecordStep records one LLM turn. Call once per inner-loop iteration
// that issued a real LLM call.
func (r *RunCollector) RecordStep(model core.Model) {
	r.stepCount.Add(1)
	r.mu.Lock()
	r.providerHits[string(model.Provider)]++
	r.modelHits[model.ID]++
	r.mu.Unlock()
}

// RecordToolCall records one tool-call result.
func (r *RunCollector) RecordToolCall(name string, isError bool) {
	r.toolCount.Add(1)
	if isError {
		r.errorCount.Add(1)
	}
	r.mu.Lock()
	r.toolsByName[name]++
	r.mu.Unlock()
}

// RecordError categorizes an error into the per-kind counter.
func (r *RunCollector) RecordError(err error) {
	if err == nil {
		return
	}
	r.errorCount.Add(1)
	kind := classifyError(err)
	r.mu.Lock()
	r.errorsByKind[kind]++
	r.mu.Unlock()
}

// RecordUsage accumulates usage into the per-run totals.
func (r *RunCollector) RecordUsage(u core.Usage) {
	r.mu.Lock()
	r.usage.Input += u.Input
	r.usage.Output += u.Output
	r.usage.CacheRead += u.CacheRead
	r.usage.CacheWrite += u.CacheWrite
	// Add the total cost (input + output + cache) from the breakdown.
	r.usage.Cost.Input += u.Cost.Input
	r.usage.Cost.Output += u.Cost.Output
	r.usage.Cost.CacheRead += u.Cost.CacheRead
	r.usage.Cost.CacheWrite += u.Cost.CacheWrite
	r.usage.Cost.Total += u.Cost.Total
	r.cost.Store(mathToBits(r.costBits() + u.Cost.Total))
	r.mu.Unlock()
}

// SetStopReason records the final stop reason (last one wins).
func (r *RunCollector) SetStopReason(sr core.StopReason) {
	r.stopReasonFinal.Store(uint32(stopReasonIndex(sr)))
}

// MarkRunEnded finalizes the run and prevents further RecordXxx from
// mutating the summary. Returns true exactly once.
func (r *RunCollector) MarkRunEnded() bool {
	return r.endedAt.CompareAndSwap(0, time.Now().UnixNano())
}

// Snapshot returns a finalized AgentRunSummary and AgentRunCoverage.
// Safe to call from any goroutine.
func (r *RunCollector) Snapshot() (AgentRunSummary, AgentRunCoverage) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check endedAt BEFORE constructing time.Unix: time.Unix(0,0) returns
	// the Unix epoch (1970-01-01), not Go's zero value, so IsZero() would
	// return false and Duration would compute as a huge negative number.
	var endedTime time.Time
	if nano := r.endedAt.Load(); nano != 0 {
		endedTime = time.Unix(0, nano)
	}

	var dur time.Duration
	if !endedTime.IsZero() {
		dur = endedTime.Sub(r.startedAt)
	}

	sum := AgentRunSummary{
		StepCount:       int(r.stepCount.Load()),
		ToolCallCount:   int(r.toolCount.Load()),
		ErrorCount:      int(r.errorCount.Load()),
		StartedAt:       r.startedAt,
		EndedAt:         endedTime,
		Duration:        dur,
		TotalUsage:      r.usage,
		TotalCost:       mathFromBits(r.cost.Load()),
		ErrorsByKind:    copyMap(r.errorsByKind),
		Providers:       copyMap(r.providerHits),
		StopReasonFinal: stopReasonFromIndex(uint8(r.stopReasonFinal.Load())),
	}

	cov := AgentRunCoverage{
		ProviderHits: copyMap(r.providerHits),
		ModelHits:    copyMap(r.modelHits),
		ToolsByName:  copyMap(r.toolsByName),
		TotalUsage:   r.usage,
		TotalCost:    mathFromBits(r.cost.Load()),
	}

	return sum, cov
}

// costBits returns the current accumulated cost as a uint64. Must be
// called with r.mu held.
func (r *RunCollector) costBits() float64 {
	return mathFromBits(r.cost.Load())
}

func classifyError(err error) string {
	if err == nil {
		return ""
	}
	if core.IsAuthError(err) {
		return "auth"
	}
	var rl *core.RateLimitError
	if errorsAs(err, &rl) {
		return "rate_limit"
	}
	var srv *core.ServerError
	if errorsAs(err, &srv) {
		return "server"
	}
	if core.IsContextOverflowError(err) {
		return "overflow"
	}
	var tcb *ToolCallBlockedError
	if errorsAs(err, &tcb) {
		return "tool_blocked"
	}
	var tnf *ToolNotFoundError
	if errorsAs(err, &tnf) {
		return "tool_not_found"
	}
	var ae *AbortError
	if errorsAs(err, &ae) {
		return "abort"
	}
	return "other"
}

// errorsAs is a tiny shim around errors.As so this file does not need
// to import "errors" twice.
func errorsAs(err error, target any) bool {
	return errorsAsImpl(err, target)
}

// stopReasonIndex maps a core.StopReason to a small uint8 for atomic
// storage. Returns 0 for unknown values.
func stopReasonIndex(sr core.StopReason) uint8 {
	switch sr {
	case core.StopStop:
		return 1
	case core.StopLength:
		return 2
	case core.StopToolUse:
		return 3
	case core.StopError:
		return 4
	case core.StopAborted:
		return 5
	}
	return 0
}

func stopReasonFromIndex(i uint8) core.StopReason {
	switch i {
	case 1:
		return core.StopStop
	case 2:
		return core.StopLength
	case 3:
		return core.StopToolUse
	case 4:
		return core.StopError
	case 5:
		return core.StopAborted
	}
	return ""
}
