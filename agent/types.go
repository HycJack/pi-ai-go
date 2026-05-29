package agent

import (
	"context"
	"encoding/json"

	piai "pi-ai-go"
)

// ToolExecutionMode controls how tools are executed.
type ToolExecutionMode string

const (
	ToolExecParallel   ToolExecutionMode = "parallel"
	ToolExecSequential ToolExecutionMode = "sequential"
)

// AgentEvent is the interface for all agent streaming events.
type AgentEvent interface {
	agentEventTag()
}

// EventAgentStart signals the start of an agent run.
type EventAgentStart struct{}

func (EventAgentStart) agentEventTag() {}

// EventAgentEnd signals the end of an agent run with final messages.
type EventAgentEnd struct {
	Messages []piai.Message
}

func (EventAgentEnd) agentEventTag() {}

// EventTurnStart signals the start of a turn (LLM call + tool execution).
type EventTurnStart struct{}

func (EventTurnStart) agentEventTag() {}

// EventTurnEnd signals the end of a turn.
type EventTurnEnd struct {
	Message     piai.AssistantMessage
	ToolResults []piai.ToolResultMessage
}

func (EventTurnEnd) agentEventTag() {}

// EventMessageStart signals the start of an assistant message stream.
type EventMessageStart struct {
	Message piai.AssistantMessage
}

func (EventMessageStart) agentEventTag() {}

// EventMessageUpdate signals a delta in the assistant message stream.
type EventMessageUpdate struct {
	Message        piai.AssistantMessage
	AssistantEvent piai.AssistantMessageEvent
}

func (EventMessageUpdate) agentEventTag() {}

// EventMessageEnd signals the end of an assistant message stream.
type EventMessageEnd struct {
	Message piai.AssistantMessage
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
	ToolCallID   string
	ToolName     string
	Args         json.RawMessage
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

func (EventToolExecEnd) agentEventTag() {}

// AgentTool defines a tool that the agent can call.
type AgentTool struct {
	Name         string
	Description  string
	Parameters   json.RawMessage // JSON Schema
	Label        string
	Execute      ToolExecuteFunc
	ExecutionMode ToolExecutionMode // "" = inherit from config
}

// ToolExecuteFunc is the function signature for tool execution.
type ToolExecuteFunc func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (AgentToolResult, error)

// AgentToolResult is the result of a tool execution.
type AgentToolResult struct {
	Content   []piai.ContentBlock
	Details   json.RawMessage
	IsError   bool
	Terminate bool
}

// BeforeToolCallContext is passed to the beforeToolCall hook.
type BeforeToolCallContext struct {
	AssistantMessage piai.AssistantMessage
	ToolCall         piai.ToolCall
	Args             json.RawMessage
	Messages         []piai.Message
}

// ToolCallBlock is returned by beforeToolCall to block execution.
type ToolCallBlock struct {
	Block  bool
	Reason string
}

// AfterToolCallContext is passed to the afterToolCall hook.
type AfterToolCallContext struct {
	AssistantMessage piai.AssistantMessage
	ToolCall         piai.ToolCall
	Args             json.RawMessage
	Result           AgentToolResult
	IsError          bool
	Messages         []piai.Message
}

// ToolCallOverride is returned by afterToolCall to override the result.
type ToolCallOverride struct {
	Content   []piai.ContentBlock
	Details   json.RawMessage
	IsError   *bool
	Terminate *bool
}

// StreamFn is the type for custom streaming functions.
type StreamFn func(piai.Model, piai.Context, piai.SimpleStreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error)

// AgentLoopConfig configures the agent loop.
type AgentLoopConfig struct {
	piai.SimpleStreamOptions

	Model        piai.Model
	SystemPrompt string
	Tools        []AgentTool
	ToolExecution ToolExecutionMode

	// ConvertToLlm transforms messages before each LLM call.
	// If nil, default conversion (filter to user/assistant/toolResult) is used.
	ConvertToLlm func([]piai.Message) []piai.Message

	// TransformContext transforms messages for context window management.
	TransformContext func([]piai.Message) []piai.Message

	// GetApiKey resolves the API key dynamically (e.g., for expiring OAuth tokens).
	GetApiKey func() string

	// ShouldStopAfterTurn is called after each turn. Return true to stop the loop.
	ShouldStopAfterTurn func(piai.AssistantMessage, []piai.ToolResultMessage) bool

	// PrepareNextTurn is called after each turn. Can modify config for the next turn.
	PrepareNextTurn func(config *AgentLoopConfig, assistantMsg piai.AssistantMessage, toolResults []piai.ToolResultMessage, messages []piai.Message)

	// GetSteeringMessages returns messages injected mid-run while tools are executing.
	GetSteeringMessages func() []piai.Message

	// GetFollowUpMessages returns messages injected after the agent would otherwise stop.
	GetFollowUpMessages func() []piai.Message

	// BeforeToolCall is called before tool execution. Can block execution.
	BeforeToolCall func(BeforeToolCallContext) *ToolCallBlock

	// AfterToolCall is called after tool execution. Can override result.
	AfterToolCall func(AfterToolCallContext) *ToolCallOverride

	// StreamFn is a custom streaming function. If nil, piai.StreamSimple is used.
	StreamFn StreamFn
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
func defaultConvertToLlm(msgs []piai.Message) []piai.Message {
	result := make([]piai.Message, 0, len(msgs))
	for _, m := range msgs {
		switch m.(type) {
		case piai.UserMessage, piai.AssistantMessage, piai.ToolResultMessage:
			result = append(result, m)
		}
	}
	return result
}

// extractToolCalls extracts tool calls from an assistant message.
func extractToolCalls(msg piai.AssistantMessage) []piai.ToolCall {
	var calls []piai.ToolCall
	for _, block := range msg.Content {
		if tc, ok := block.(piai.ToolCall); ok {
			calls = append(calls, tc)
		}
	}
	return calls
}

// toContextMessages converts a slice of Messages to a context for LLM calls.
func toContextMessages(msgs []piai.Message, systemPrompt string, tools []AgentTool) piai.Context {
	llmTools := make([]piai.Tool, len(tools))
	for i, t := range tools {
		llmTools[i] = piai.Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		}
	}
	return piai.Context{
		SystemPrompt: systemPrompt,
		Messages:     msgs,
		Tools:        llmTools,
	}
}
