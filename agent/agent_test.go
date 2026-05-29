package agent

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	core "pi-ai-go/core"
)

// mockStreamFn creates a StreamFn that returns a pre-built assistant message.
func mockStreamFn(msg core.AssistantMessage) StreamFn {
	return func(ctx context.Context, model core.Model, llmCtx core.Context, opts core.SimpleStreamOptions) (*core.EventStream[core.AssistantMessageEvent, core.AssistantMessage], error) {
		stream := core.NewEventStream[core.AssistantMessageEvent, core.AssistantMessage]()
		go func() {
			stream.Push(core.EventStart{
				Type:     "start",
				API:      msg.API,
				Provider: msg.Provider,
				Model:    msg.Model,
			})
			for _, block := range msg.Content {
				switch b := block.(type) {
				case core.TextContent:
					stream.Push(core.EventTextDelta{Type: "text_delta", Delta: b.Text})
					stream.Push(core.EventTextEnd{Type: "text_end"})
				case core.ThinkingContent:
					stream.Push(core.EventThinkingDelta{Type: "thinking_delta", Delta: b.Thinking})
					stream.Push(core.EventThinkingEnd{Type: "thinking_end"})
				case core.ToolCall:
					stream.Push(core.EventToolCallStart{Type: "toolcall_start", ID: b.ID, Name: b.Name})
					stream.Push(core.EventToolCallDelta{Type: "toolcall_delta", ID: b.ID, ArgumentsDelta: string(b.Arguments)})
					stream.Push(core.EventToolCallEnd{Type: "toolcall_end", ID: b.ID, Arguments: b.Arguments})
				}
			}
			stream.Push(core.EventDone{Type: "done", Message: msg})
			stream.End(msg)
		}()
		return stream, nil
	}
}

// mockStreamFnError creates a StreamFn that returns an error.
func mockStreamFnError(err error) StreamFn {
	return func(ctx context.Context, model core.Model, llmCtx core.Context, opts core.SimpleStreamOptions) (*core.EventStream[core.AssistantMessageEvent, core.AssistantMessage], error) {
		return nil, err
	}
}

func TestAgentLoopNoToolCalls(t *testing.T) {
	msg := core.AssistantMessage{
		Role:       "assistant",
		StopReason: core.StopStop,
		Content:    []core.ContentBlock{core.TextContent{Type: "text", Text: "Hello!"}},
	}

	config := AgentLoopConfig{
		StreamFn: mockStreamFn(msg),
	}

	stream := AgentLoop(context.Background(), []core.Message{
		core.UserMessage{Role: "user", Content: "Hi"},
	}, config)

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(result))
	}

	// Last message should be the assistant response
	last := result[len(result)-1]
	am, ok := last.(core.AssistantMessage)
	if !ok {
		t.Fatalf("expected AssistantMessage, got %T", last)
	}
	if am.StopReason != core.StopStop {
		t.Errorf("expected stop reason 'stop', got '%s'", am.StopReason)
	}
}

func TestAgentLoopWithToolCall(t *testing.T) {
	// First response: tool call
	toolCallMsg := core.AssistantMessage{
		Role:       "assistant",
		StopReason: core.StopToolUse,
		Content: []core.ContentBlock{
			core.ToolCall{
				Type:      "toolCall",
				ID:        "call_1",
				Name:      "calculator",
				Arguments: json.RawMessage(`{"expression":"2+2"}`),
			},
		},
	}

	// Second response: text after tool result
	textMsg := core.AssistantMessage{
		Role:       "assistant",
		StopReason: core.StopStop,
		Content:    []core.ContentBlock{core.TextContent{Type: "text", Text: "The answer is 4."}},
	}

	callCount := 0
	streamFn := func(ctx context.Context, model core.Model, llmCtx core.Context, opts core.SimpleStreamOptions) (*core.EventStream[core.AssistantMessageEvent, core.AssistantMessage], error) {
		callCount++
		if callCount == 1 {
			return mockStreamFn(toolCallMsg)(ctx, model, llmCtx, opts)
		}
		return mockStreamFn(textMsg)(ctx, model, llmCtx, opts)
	}

	calculator := AgentTool{
		Name:        "calculator",
		Description: "Evaluate a math expression",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"expression":{"type":"string"}}}`),
		Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (AgentToolResult, error) {
			return AgentToolResult{
				Content: []core.ContentBlock{core.TextContent{Type: "text", Text: "4"}},
			}, nil
		},
	}

	config := AgentLoopConfig{
		StreamFn: streamFn,
		Tools:    []AgentTool{calculator},
	}

	stream := AgentLoop(context.Background(), []core.Message{
		core.UserMessage{Role: "user", Content: "What is 2+2?"},
	}, config)

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have: user, assistant(tool_call), toolResult, assistant(text)
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}

	// Check tool result
	tr, ok := result[2].(core.ToolResultMessage)
	if !ok {
		t.Fatalf("expected ToolResultMessage, got %T", result[2])
	}
	if tr.ToolCallID != "call_1" {
		t.Errorf("expected tool call ID 'call_1', got '%s'", tr.ToolCallID)
	}
	if tr.IsError {
		t.Error("tool result should not be an error")
	}

	// Check final message
	am, ok := result[3].(core.AssistantMessage)
	if !ok {
		t.Fatalf("expected AssistantMessage, got %T", result[3])
	}
	if am.StopReason != core.StopStop {
		t.Errorf("expected stop reason 'stop', got '%s'", am.StopReason)
	}
}

func TestAgentLoopToolError(t *testing.T) {
	toolCallMsg := core.AssistantMessage{
		Role:       "assistant",
		StopReason: core.StopToolUse,
		Content: []core.ContentBlock{
			core.ToolCall{
				Type:      "toolCall",
				ID:        "call_1",
				Name:      "failing_tool",
				Arguments: json.RawMessage(`{}`),
			},
		},
	}

	textMsg := core.AssistantMessage{
		Role:       "assistant",
		StopReason: core.StopStop,
		Content:    []core.ContentBlock{core.TextContent{Type: "text", Text: "Done."}},
	}

	callCount := 0
	streamFn := func(ctx context.Context, model core.Model, llmCtx core.Context, opts core.SimpleStreamOptions) (*core.EventStream[core.AssistantMessageEvent, core.AssistantMessage], error) {
		callCount++
		if callCount == 1 {
			return mockStreamFn(toolCallMsg)(ctx, model, llmCtx, opts)
		}
		return mockStreamFn(textMsg)(ctx, model, llmCtx, opts)
	}

	tool := AgentTool{
		Name:       "failing_tool",
		Parameters: json.RawMessage(`{}`),
		Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (AgentToolResult, error) {
			return AgentToolResult{}, errors.New("tool failed")
		},
	}

	config := AgentLoopConfig{
		StreamFn: streamFn,
		Tools:    []AgentTool{tool},
	}

	stream := AgentLoop(context.Background(), []core.Message{
		core.UserMessage{Role: "user", Content: "Do something"},
	}, config)

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find tool result
	var toolResult core.ToolResultMessage
	for _, m := range result {
		if tr, ok := m.(core.ToolResultMessage); ok {
			toolResult = tr
			break
		}
	}

	if !toolResult.IsError {
		t.Error("tool result should be marked as error")
	}
}

func TestAgentLoopBeforeToolCallBlock(t *testing.T) {
	toolCallMsg := core.AssistantMessage{
		Role:       "assistant",
		StopReason: core.StopToolUse,
		Content: []core.ContentBlock{
			core.ToolCall{
				Type:      "toolCall",
				ID:        "call_1",
				Name:      "blocked_tool",
				Arguments: json.RawMessage(`{}`),
			},
		},
	}

	textMsg := core.AssistantMessage{
		Role:       "assistant",
		StopReason: core.StopStop,
		Content:    []core.ContentBlock{core.TextContent{Type: "text", Text: "Done."}},
	}

	callCount := 0
	streamFn := func(ctx context.Context, model core.Model, llmCtx core.Context, opts core.SimpleStreamOptions) (*core.EventStream[core.AssistantMessageEvent, core.AssistantMessage], error) {
		callCount++
		if callCount == 1 {
			return mockStreamFn(toolCallMsg)(ctx, model, llmCtx, opts)
		}
		return mockStreamFn(textMsg)(ctx, model, llmCtx, opts)
	}

	executed := false
	tool := AgentTool{
		Name:       "blocked_tool",
		Parameters: json.RawMessage(`{}`),
		Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (AgentToolResult, error) {
			executed = true
			return AgentToolResult{}, nil
		},
	}

	config := AgentLoopConfig{
		StreamFn: streamFn,
		Tools:    []AgentTool{tool},
		BeforeToolCall: func(ctx BeforeToolCallContext) *ToolCallBlock {
			return &ToolCallBlock{Block: true, Reason: "not allowed"}
		},
	}

	stream := AgentLoop(context.Background(), []core.Message{
		core.UserMessage{Role: "user", Content: "Do something"},
	}, config)

	_, err := stream.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if executed {
		t.Error("tool should not have been executed")
	}
}

func TestAgentLoopAfterToolCallOverride(t *testing.T) {
	toolCallMsg := core.AssistantMessage{
		Role:       "assistant",
		StopReason: core.StopToolUse,
		Content: []core.ContentBlock{
			core.ToolCall{
				Type:      "toolCall",
				ID:        "call_1",
				Name:      "my_tool",
				Arguments: json.RawMessage(`{}`),
			},
		},
	}

	textMsg := core.AssistantMessage{
		Role:       "assistant",
		StopReason: core.StopStop,
		Content:    []core.ContentBlock{core.TextContent{Type: "text", Text: "Done."}},
	}

	callCount := 0
	streamFn := func(ctx context.Context, model core.Model, llmCtx core.Context, opts core.SimpleStreamOptions) (*core.EventStream[core.AssistantMessageEvent, core.AssistantMessage], error) {
		callCount++
		if callCount == 1 {
			return mockStreamFn(toolCallMsg)(ctx, model, llmCtx, opts)
		}
		return mockStreamFn(textMsg)(ctx, model, llmCtx, opts)
	}

	tool := AgentTool{
		Name:       "my_tool",
		Parameters: json.RawMessage(`{}`),
		Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (AgentToolResult, error) {
			return AgentToolResult{
				Content: []core.ContentBlock{core.TextContent{Type: "text", Text: "original"}},
			}, nil
		},
	}

	isErr := true
	config := AgentLoopConfig{
		StreamFn: streamFn,
		Tools:    []AgentTool{tool},
		AfterToolCall: func(ctx AfterToolCallContext) *ToolCallOverride {
			return &ToolCallOverride{
				Content: []core.ContentBlock{core.TextContent{Type: "text", Text: "overridden"}},
				IsError: &isErr,
			}
		},
	}

	stream := AgentLoop(context.Background(), []core.Message{
		core.UserMessage{Role: "user", Content: "Do something"},
	}, config)

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find tool result
	for _, m := range result {
		if tr, ok := m.(core.ToolResultMessage); ok {
			if !tr.IsError {
				t.Error("tool result should be marked as error after override")
			}
			if len(tr.Content) > 0 {
				if tc, ok := tr.Content[0].(core.TextContent); ok {
					if tc.Text != "overridden" {
						t.Errorf("expected 'overridden', got '%s'", tc.Text)
					}
				}
			}
			return
		}
	}
	t.Error("tool result not found")
}

func TestAgentLoopShouldStopAfterTurn(t *testing.T) {
	textMsg := core.AssistantMessage{
		Role:       "assistant",
		StopReason: core.StopStop,
		Content:    []core.ContentBlock{core.TextContent{Type: "text", Text: "Hello!"}},
	}

	config := AgentLoopConfig{
		StreamFn: mockStreamFn(textMsg),
		ShouldStopAfterTurn: func(msg core.AssistantMessage, results []core.ToolResultMessage) bool {
			return true
		},
	}

	stream := AgentLoop(context.Background(), []core.Message{
		core.UserMessage{Role: "user", Content: "Hi"},
	}, config)

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(result))
	}
}

func TestAgentLoopStreamFnError(t *testing.T) {
	config := AgentLoopConfig{
		StreamFn: mockStreamFnError(errors.New("API error")),
	}

	stream := AgentLoop(context.Background(), []core.Message{
		core.UserMessage{Role: "user", Content: "Hi"},
	}, config)

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have user message + error assistant message
	if len(result) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(result))
	}

	last := result[len(result)-1]
	am, ok := last.(core.AssistantMessage)
	if !ok {
		t.Fatalf("expected AssistantMessage, got %T", last)
	}
	if am.StopReason != core.StopError {
		t.Errorf("expected stop reason 'error', got '%s'", am.StopReason)
	}
}

func TestAgentLoopContextCancel(t *testing.T) {
	textMsg := core.AssistantMessage{
		Role:       "assistant",
		StopReason: core.StopStop,
		Content:    []core.ContentBlock{core.TextContent{Type: "text", Text: "Hello!"}},
	}

	config := AgentLoopConfig{
		StreamFn: mockStreamFn(textMsg),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	stream := AgentLoop(ctx, []core.Message{
		core.UserMessage{Role: "user", Content: "Hi"},
	}, config)

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still complete (context cancel just stops the loop early)
	if len(result) == 0 {
		t.Error("expected at least 1 message")
	}
}

func TestAgentLoopParallelToolExecution(t *testing.T) {
	// Two tool calls in one response
	toolCallMsg := core.AssistantMessage{
		Role:       "assistant",
		StopReason: core.StopToolUse,
		Content: []core.ContentBlock{
			core.ToolCall{
				Type:      "toolCall",
				ID:        "call_1",
				Name:      "tool_a",
				Arguments: json.RawMessage(`{}`),
			},
			core.ToolCall{
				Type:      "toolCall",
				ID:        "call_2",
				Name:      "tool_b",
				Arguments: json.RawMessage(`{}`),
			},
		},
	}

	textMsg := core.AssistantMessage{
		Role:       "assistant",
		StopReason: core.StopStop,
		Content:    []core.ContentBlock{core.TextContent{Type: "text", Text: "Done."}},
	}

	callCount := 0
	streamFn := func(ctx context.Context, model core.Model, llmCtx core.Context, opts core.SimpleStreamOptions) (*core.EventStream[core.AssistantMessageEvent, core.AssistantMessage], error) {
		callCount++
		if callCount == 1 {
			return mockStreamFn(toolCallMsg)(ctx, model, llmCtx, opts)
		}
		return mockStreamFn(textMsg)(ctx, model, llmCtx, opts)
	}

	var mu sync.Mutex
	executionOrder := []string{}

	toolA := AgentTool{
		Name:       "tool_a",
		Parameters: json.RawMessage(`{}`),
		Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (AgentToolResult, error) {
			mu.Lock()
			executionOrder = append(executionOrder, "a")
			mu.Unlock()
			return AgentToolResult{
				Content: []core.ContentBlock{core.TextContent{Type: "text", Text: "result_a"}},
			}, nil
		},
	}

	toolB := AgentTool{
		Name:       "tool_b",
		Parameters: json.RawMessage(`{}`),
		Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (AgentToolResult, error) {
			mu.Lock()
			executionOrder = append(executionOrder, "b")
			mu.Unlock()
			return AgentToolResult{
				Content: []core.ContentBlock{core.TextContent{Type: "text", Text: "result_b"}},
			}, nil
		},
	}

	config := AgentLoopConfig{
		StreamFn:       streamFn,
		Tools:          []AgentTool{toolA, toolB},
		ToolExecution:  ToolExecParallel,
	}

	stream := AgentLoop(context.Background(), []core.Message{
		core.UserMessage{Role: "user", Content: "Do both"},
	}, config)

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have: user, assistant(tool_calls), toolResult_a, toolResult_b, assistant(text)
	if len(result) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(result))
	}

	// Both tools should have been called
	if len(executionOrder) != 2 {
		t.Fatalf("expected 2 tool executions, got %d", len(executionOrder))
	}
}

func TestAgentLoopTerminateEarly(t *testing.T) {
	toolCallMsg := core.AssistantMessage{
		Role:       "assistant",
		StopReason: core.StopToolUse,
		Content: []core.ContentBlock{
			core.ToolCall{
				Type:      "toolCall",
				ID:        "call_1",
				Name:      "terminator",
				Arguments: json.RawMessage(`{}`),
			},
		},
	}

	streamFn := func(ctx context.Context, model core.Model, llmCtx core.Context, opts core.SimpleStreamOptions) (*core.EventStream[core.AssistantMessageEvent, core.AssistantMessage], error) {
		return mockStreamFn(toolCallMsg)(ctx, model, llmCtx, opts)
	}

	tool := AgentTool{
		Name:       "terminator",
		Parameters: json.RawMessage(`{}`),
		Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (AgentToolResult, error) {
			return AgentToolResult{
				Content:   []core.ContentBlock{core.TextContent{Type: "text", Text: "terminating"}},
				Terminate: true,
			}, nil
		},
	}

	config := AgentLoopConfig{
		StreamFn: streamFn,
		Tools:    []AgentTool{tool},
	}

	stream := AgentLoop(context.Background(), []core.Message{
		core.UserMessage{Role: "user", Content: "Terminate"},
	}, config)

	result, err := stream.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have: user, assistant(tool_call), toolResult, turn_end, agent_end
	// But no second LLM call because terminate=true
	if len(result) != 3 {
		t.Fatalf("expected 3 messages (terminate skips next LLM call), got %d", len(result))
	}
}

func TestExtractToolCalls(t *testing.T) {
	msg := core.AssistantMessage{
		Content: []core.ContentBlock{
			core.TextContent{Type: "text", Text: "Let me help"},
			core.ToolCall{Type: "toolCall", ID: "c1", Name: "tool1"},
			core.ToolCall{Type: "toolCall", ID: "c2", Name: "tool2"},
		},
	}

	calls := extractToolCalls(msg)
	if len(calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(calls))
	}
	if calls[0].ID != "c1" || calls[1].ID != "c2" {
		t.Errorf("unexpected tool call IDs: %s, %s", calls[0].ID, calls[1].ID)
	}
}

func TestFindTool(t *testing.T) {
	tools := []AgentTool{
		{Name: "a", Description: "tool a"},
		{Name: "b", Description: "tool b"},
	}

	tool := findTool(tools, "b")
	if tool == nil {
		t.Fatal("expected to find tool 'b'")
	}
	if tool.Description != "tool b" {
		t.Errorf("expected 'tool b', got '%s'", tool.Description)
	}

	tool = findTool(tools, "c")
	if tool != nil {
		t.Error("expected nil for non-existent tool")
	}
}
