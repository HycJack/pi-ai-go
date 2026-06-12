package agent

import (
	"context"
	"encoding/json"

	core "pi-ai-go/core"
)

// ToolExecutionMode is an alias for core.ToolExecutionMode.
// || core.ToolExecutionMode 的别名，保持向后兼容
type ToolExecutionMode = core.ToolExecutionMode

const (
	ToolExecParallel   = core.ToolExecParallel
	ToolExecSequential = core.ToolExecSequential
)

// AgentEvent is the interface for all agent streaming events.
type AgentEvent interface {
	agentEventTag()
}

// EventAgentStart signals the start of an agent run.
type EventAgentStart struct{}

func (EventAgentStart) agentEventTag() {}

// EventAgentEnd signals the end of an agent run with final messages.
//
// Summary and Coverage are populated when the AgentLoop was constructed
// with a Collector (either via the config or via AgentLoopDetailed).
// They are nil if the run was short-circuited before the collector was
// attached.
type EventAgentEnd struct {
	Messages []core.Message
	Summary  *AgentRunSummary
	Coverage *AgentRunCoverage
}

func (EventAgentEnd) agentEventTag() {}

// EventTurnStart signals the start of a turn (LLM call + tool execution).
type EventTurnStart struct{}

func (EventTurnStart) agentEventTag() {}

// EventTurnEnd signals the end of a turn.
type EventTurnEnd struct {
	Message     core.AssistantMessage
	ToolResults []core.ToolResultMessage
}

func (EventTurnEnd) agentEventTag() {}

// EventMessageStart signals the start of an assistant message stream.
type EventMessageStart struct {
	Message core.AssistantMessage
}

func (EventMessageStart) agentEventTag() {}

// EventMessageUpdate signals a delta in the assistant message stream.
type EventMessageUpdate struct {
	Message        core.AssistantMessage
	AssistantEvent core.AssistantMessageEvent
}

func (EventMessageUpdate) agentEventTag() {}

// EventMessageEnd signals the end of an assistant message stream.
type EventMessageEnd struct {
	Message core.AssistantMessage
}

func (EventMessageEnd) agentEventTag() {}

// EventToolExecStart signals the start of a tool execution.
type EventToolExecStart struct {
	ToolCallID string
	ToolName   string
	Args       json.RawMessage
}

func (EventToolExecStart) agentEventTag() {}

// EventToolExecUpdate signals a partial result update during tool execution.
type EventToolExecUpdate struct {
	ToolCallID    string
	ToolName      string
	Args          json.RawMessage
	PartialResult json.RawMessage
}

func (EventToolExecUpdate) agentEventTag() {}

// EventToolExecEnd signals the end of a tool execution.
type EventToolExecEnd struct {
	ToolCallID string
	ToolName   string
	Result     json.RawMessage
	IsError    bool
}

// EventCompaction signals a context-compaction operation. It is emitted
// AFTER the messages slice has been replaced in the loop, so consumers
// can re-render the truncated history.
type EventCompaction struct {
	Strategy     CompactionStrategy
	TokensBefore int
	TokensAfter  int
	Dropped      int
	TriggeredBy  string
}

func (EventCompaction) agentEventTag() {}

func (EventToolExecEnd) agentEventTag() {}

// AgentTool is an alias for core.AgentTool.
// || core.AgentTool 的别名，保持向后兼容
type AgentTool = core.AgentTool

// ToolExecuteFunc is an alias for core.ToolExecuteFunc.
// || core.ToolExecuteFunc 的别名，保持向后兼容
type ToolExecuteFunc = core.ToolExecuteFunc

// AgentToolResult is an alias for core.AgentToolResult.
// || core.AgentToolResult 的别名，保持向后兼容
type AgentToolResult = core.AgentToolResult

// BeforeToolCallContext is passed to the beforeToolCall hook.
type BeforeToolCallContext struct {
	AssistantMessage core.AssistantMessage
	ToolCall         core.ToolCall
	Args             json.RawMessage
	Messages         []core.Message
}

// ToolCallBlock is returned by beforeToolCall to block execution.
type ToolCallBlock struct {
	Block  bool
	Reason string
}

// AfterToolCallContext is passed to the afterToolCall hook.
type AfterToolCallContext struct {
	AssistantMessage core.AssistantMessage
	ToolCall         core.ToolCall
	Args             json.RawMessage
	Result           AgentToolResult
	IsError          bool
	Messages         []core.Message
}

// ToolCallOverride is returned by afterToolCall to override the result.
type ToolCallOverride struct {
	Content   []core.ContentBlock
	Details   json.RawMessage
	IsError   *bool
	Terminate *bool
}

// StreamFn is the type for custom streaming functions.
type StreamFn func(context.Context, core.Model, core.Context, core.SimpleStreamOptions) (*core.EventStream[core.AssistantMessageEvent, core.AssistantMessage], error)

// AgentLoopConfig configures the agent loop.
type AgentLoopConfig struct {
	core.SimpleStreamOptions

	Model         core.Model
	SystemPrompt  string
	Tools         []AgentTool
	ToolExecution ToolExecutionMode

	// ConvertToLlm transforms messages before each LLM call.
	// If nil, default conversion (filter to user/assistant/toolResult) is used.
	ConvertToLlm func([]core.Message) []core.Message

	// TransformContext transforms messages for context window management.
	TransformContext func([]core.Message) []core.Message

	// GetApiKey resolves the API key dynamically (e.g., for expiring OAuth tokens).
	GetApiKey func() string

	// ShouldStopAfterTurn is called after each turn. Return true to stop the loop.
	ShouldStopAfterTurn func(core.AssistantMessage, []core.ToolResultMessage) bool

	// PrepareNextTurn is called after each turn. Can modify config for the next turn.
	PrepareNextTurn func(config *AgentLoopConfig, assistantMsg core.AssistantMessage, toolResults []core.ToolResultMessage, messages []core.Message)

	// GetSteeringMessages returns messages injected mid-run while tools are executing.
	GetSteeringMessages func() []core.Message

	// GetFollowUpMessages returns messages injected after the agent would otherwise stop.
	GetFollowUpMessages func() []core.Message

	// BeforeToolCall is called before tool execution. Can block execution.
	BeforeToolCall func(BeforeToolCallContext) *ToolCallBlock

	// AfterToolCall is called after tool execution. Can override result.
	AfterToolCall func(AfterToolCallContext) *ToolCallOverride

	// StreamFn is a custom streaming function. If nil, core.StreamSimple is used.
	StreamFn StreamFn

	// --- New (oh-my-pi parity) ---

	// Yielder is the cooperative-scheduling checkpoint. If nil, a
	// default 50ms yielder is used. Callers can override it to disable
	// yields (set Interval = math.MaxInt64) or to instrument them.
	Yielder *Yielder

	// Collector is the per-run stats collector. If nil, an internal
	// collector is created. The agent_end event payload will include a
	// snapshot of this collector.
	Collector *RunCollector

	// Queue controls how Steering/FollowUp messages are scheduled.
	// If nil, the loop falls back to the legacy GetSteeringMessages /
	// GetFollowUpMessages callbacks.
	Queue *MessageQueue

	// OnOverflow, if set, is called when the LLM stream returns an
	// overflow error or when the agent detects silent overflow via
	// usage. The hook is informational; it does not change the loop
	// behavior. The loop will still surface the overflow via the
	// assistant message (StopReason=StopError) and the event stream.
	//
	// Return a non-nil error to short-circuit the loop with that error.
	OnOverflow func(*OverflowSignal) error

	// OverflowHook, if set, is invoked when silent overflow is detected
	// mid-stream. The hook is given a chance to truncate / compact the
	// context before the next LLM call. Return a (possibly new) message
	// slice to continue the loop, or an error to abort.
	OverflowHook func(messages []core.Message) ([]core.Message, error)

	// ContextPolicy, if non-nil, enables context-window management.
	// The agent loop will check usage after every LLM call and trigger
	// compaction if usage exceeds the policy's soft limit. Hard limit
	// forces compaction before the next call.
	ContextPolicy *ContextPolicy

	// SummarizeModel, if set, is used by the "summarize" compaction
	// strategy. Ignored when ContextPolicy.Strategy != summarize.
	SummarizeModel *SummarizeModel

	// SummarizePrompt overrides the default summarization prompt.
	SummarizePrompt string

	// OnCompaction, if set, is called after a successful compaction.
	// It is purely observational; the loop continues with the
	// compacted slice regardless of the return value.
	OnCompaction func(CompactionEvent)
}

// CompactionEvent is the payload passed to AgentLoopConfig.OnCompaction.
type CompactionEvent struct {
	Strategy     CompactionStrategy
	TokensBefore int
	TokensAfter  int
	Dropped      int
	TriggeredBy  string // "soft" / "hard" / "overflow"
}

// findTool looks up a tool by name.
func findTool(tools []AgentTool, name string) *AgentTool {
	for i := range tools {
		if tools[i].Name == name {
			return &tools[i]
		}
	}
	return nil
}

// defaultConvertToLlm filters messages to LLM-compatible types.
func defaultConvertToLlm(msgs []core.Message) []core.Message {
	result := make([]core.Message, 0, len(msgs))
	for _, m := range msgs {
		switch m.(type) {
		case core.UserMessage, core.AssistantMessage, core.ToolResultMessage:
			result = append(result, m)
		}
	}
	return result
}

// extractToolCalls extracts tool calls from an assistant message.
func extractToolCalls(msg core.AssistantMessage) []core.ToolCall {
	var calls []core.ToolCall
	for _, block := range msg.Content {
		if tc, ok := block.(core.ToolCall); ok {
			calls = append(calls, tc)
		}
	}
	return calls
}

// toContextMessages converts a slice of Messages to a context for LLM calls.
func toContextMessages(msgs []core.Message, systemPrompt string, tools []AgentTool) core.Context {
	llmTools := make([]core.Tool, len(tools))
	for i, t := range tools {
		llmTools[i] = core.Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		}
	}
	return core.Context{
		SystemPrompt: systemPrompt,
		Messages:     msgs,
		Tools:        llmTools,
	}
}
