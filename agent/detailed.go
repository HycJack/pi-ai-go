package agent

import (
	"context"
	core "pi-ai-go/core"
)

// AgentLoopDetailedResult is the rich return value produced by
// AgentLoopDetailed. It is the additive counterpart to the bare
// `AgentMessage[]` returned by AgentLoop; the same EventStream is
// returned either way.
//
// Mirrors oh-my-pi's `AgentLoopDetailedResult` so consumers can:
//   - Display per-run usage / cost totals in a TUI.
//   - Decide whether to compact (using ErrorsByKind["overflow"]).
//   - Surface the final stop reason without re-parsing events.
type AgentLoopDetailedResult struct {
	Messages  []core.Message
	Summary   AgentRunSummary
	Coverage  AgentRunCoverage
	Collector *RunCollector // non-nil so callers can keep collecting
}

// AgentLoopDetailed is the additive version of AgentLoop. It returns
// both the agent event stream AND a `detailed` accessor that resolves
// to the rich result once the loop has finished.
func AgentLoopDetailed(
	ctx context.Context,
	prompts []core.Message,
	config AgentLoopConfig,
) (stream *AgentEventStream, detailed func() (AgentLoopDetailedResult, error)) {
	// Ensure we have a collector; the loop will populate it.
	if config.Collector == nil {
		config.Collector = NewRunCollector()
	}
	stream = AgentLoop(ctx, prompts, config)

	detailed = func() (AgentLoopDetailedResult, error) {
		messages, err := stream.Result()
		if err != nil {
			return AgentLoopDetailedResult{}, err
		}
		summary, coverage := config.Collector.Snapshot()
		return AgentLoopDetailedResult{
			Messages:  messages,
			Summary:   summary,
			Coverage:  coverage,
			Collector: config.Collector,
		}, nil
	}
	return stream, detailed
}

// AgentLoopContinueDetailed is the continue-mode counterpart of
// AgentLoopDetailed.
func AgentLoopContinueDetailed(
	ctx context.Context,
	config AgentLoopConfig,
	messages []core.Message,
) (stream *AgentEventStream, detailed func() (AgentLoopDetailedResult, error)) {
	if config.Collector == nil {
		config.Collector = NewRunCollector()
	}
	stream = AgentLoopContinue(ctx, config, messages)

	detailed = func() (AgentLoopDetailedResult, error) {
		resMessages, err := stream.Result()
		if err != nil {
			return AgentLoopDetailedResult{}, err
		}
		summary, coverage := config.Collector.Snapshot()
		return AgentLoopDetailedResult{
			Messages:  resMessages,
			Summary:   summary,
			Coverage:  coverage,
			Collector: config.Collector,
		}, nil
	}
	return stream, detailed
}
