package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	piai "pi-ai-go"
)

// AgentEventStream is the type alias for the agent event stream.
type AgentEventStream = piai.EventStream[AgentEvent, []piai.Message]

// AgentLoop starts a new agent run with the given prompt messages.
func AgentLoop(ctx context.Context, msgs []piai.Message, config AgentLoopConfig) *AgentEventStream {
	stream := piai.NewEventStream[AgentEvent, []piai.Message]()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				stream.Error(fmt.Errorf("agent: panic: %v", r))
			}
		}()

		stream.Push(EventAgentStart{})

		// Emit message_start/message_end for prompt messages
		for _, m := range msgs {
			if am, ok := m.(piai.AssistantMessage); ok {
				stream.Push(EventMessageStart{Message: am})
				stream.Push(EventMessageEnd{Message: am})
			}
		}

		messages := make([]piai.Message, len(msgs))
		copy(messages, msgs)

		runLoop(ctx, config, messages, stream)
	}()

	return stream
}

// AgentLoopContinue resumes an agent run from existing context.
// The last message must be a user or toolResult message.
func AgentLoopContinue(ctx context.Context, config AgentLoopConfig, messages []piai.Message) *AgentEventStream {
	stream := piai.NewEventStream[AgentEvent, []piai.Message]()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				stream.Error(fmt.Errorf("agent: panic: %v", r))
			}
		}()

		stream.Push(EventAgentStart{})
		runLoop(ctx, config, messages, stream)
	}()

	return stream
}

// runLoop implements the core two-level agent loop.
func runLoop(ctx context.Context, config AgentLoopConfig, messages []piai.Message, stream *AgentEventStream) {
	for {
		// Inner loop: process tool calls and steering messages
		for {
			if ctx.Err() != nil {
				stream.End(messages)
				return
			}

			// Inject steering messages before emitting turn_start
			hasSteering := false
			if config.GetSteeringMessages != nil {
				steering := config.GetSteeringMessages()
				if len(steering) > 0 {
					messages = append(messages, steering...)
					hasSteering = true
				}
			}

			stream.Push(EventTurnStart{})

			// Stream assistant response
			assistantMsg, err := streamAssistantResponse(ctx, config, messages, stream)
			if err != nil {
				errMsg := piai.AssistantMessage{
					Role:         "assistant",
					StopReason:   piai.StopError,
					ErrorMessage: err.Error(),
				}
				messages = append(messages, errMsg)
				stream.Push(EventTurnEnd{Message: errMsg})
				stream.End(messages)
				return
			}

			messages = append(messages, assistantMsg)

			// Check for error/aborted stop reasons
			if assistantMsg.StopReason == piai.StopError || assistantMsg.StopReason == piai.StopAborted {
				stream.Push(EventTurnEnd{Message: assistantMsg})
				stream.End(messages)
				return
			}

			// Extract tool calls
			toolCalls := extractToolCalls(assistantMsg)

			// Execute tool calls if any
			var toolResults []piai.ToolResultMessage
			shouldTerminate := false
			if len(toolCalls) > 0 {
				toolResults, shouldTerminate = executeToolCalls(ctx, config, assistantMsg, toolCalls, messages, stream)
				messages = append(messages, msgSlice(toolResults)...)
			}

			stream.Push(EventTurnEnd{
				Message:     assistantMsg,
				ToolResults: toolResults,
			})

			// If all tools requested termination, stop the loop
			if shouldTerminate {
				stream.End(messages)
				return
			}

			// Prepare next turn hook
			if config.PrepareNextTurn != nil {
				config.PrepareNextTurn(&config, assistantMsg, toolResults, messages)
			}

			// Should stop after turn hook
			if config.ShouldStopAfterTurn != nil && config.ShouldStopAfterTurn(assistantMsg, toolResults) {
				stream.End(messages)
				return
			}

			// If no tool calls and no steering was injected, exit inner loop
			if len(toolCalls) == 0 && !hasSteering {
				break // Exit inner loop, check follow-up
			}
			// Has tool calls or had steering, continue inner loop
		}

		// Check follow-up messages (outer loop)
		hasFollowUp := false
		if config.GetFollowUpMessages != nil {
			followUp := config.GetFollowUpMessages()
			if len(followUp) > 0 {
				messages = append(messages, followUp...)
				hasFollowUp = true
			}
		}

		if !hasFollowUp {
			stream.End(messages)
			return
		}
		// Has follow-up, continue outer loop
	}
}

// streamAssistantResponse streams an LLM response and returns the final message.
func streamAssistantResponse(ctx context.Context, config AgentLoopConfig, messages []piai.Message, stream *AgentEventStream) (piai.AssistantMessage, error) {
	// Transform context
	if config.TransformContext != nil {
		messages = config.TransformContext(messages)
	}

	// Convert to LLM messages
	convertFn := config.ConvertToLlm
	if convertFn == nil {
		convertFn = defaultConvertToLlm
	}
	llmMessages := convertFn(messages)

	// Resolve API key
	apiKey := ""
	if config.GetApiKey != nil {
		apiKey = config.GetApiKey()
	}
	if apiKey == "" {
		apiKey = config.SimpleStreamOptions.APIKey
	}

	opts := config.SimpleStreamOptions
	opts.APIKey = apiKey

	// Build context
	llmCtx := toContextMessages(llmMessages, config.SystemPrompt, config.Tools)

	// Stream response
	streamFn := config.StreamFn
	if streamFn == nil {
		streamFn = func(m piai.Model, c piai.Context, o piai.SimpleStreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
			return piai.StreamSimpleWithContext(ctx, m, c, o)
		}
	}

	llmStream, err := streamFn(config.Model, llmCtx, opts)
	if err != nil {
		return piai.AssistantMessage{}, err
	}
	if llmStream == nil {
		return piai.AssistantMessage{}, fmt.Errorf("agent: stream function returned nil stream")
	}

	// Partial message for streaming updates
	var partialMsg piai.AssistantMessage
	partialMsg.Role = "assistant"
	partialMsg.Timestamp = time.Now()

	stream.Push(EventMessageStart{Message: partialMsg})

	// Iterate over LLM events
	finalMsg, err := llmStream.ForEach(ctx, func(evt piai.AssistantMessageEvent) error {
		// Update partial message based on event type
		switch e := evt.(type) {
		case piai.EventStart:
			partialMsg.API = e.API
			partialMsg.Provider = e.Provider
			partialMsg.Model = e.Model
		case piai.EventTextDelta:
			partialMsg.Content = appendOrUpdateText(partialMsg.Content, e.Delta)
		case piai.EventThinkingDelta:
			partialMsg.Content = appendOrUpdateThinking(partialMsg.Content, e.Delta)
		case piai.EventToolCallStart:
			partialMsg.Content = append(partialMsg.Content, piai.ToolCall{
				Type: "toolCall",
				ID:   e.ID,
				Name: e.Name,
			})
		case piai.EventToolCallDelta:
			partialMsg.Content = updateToolCallArgs(partialMsg.Content, e.ID, e.ArgumentsDelta)
		case piai.EventToolCallEnd:
			partialMsg.Content = finalizeToolCallArgs(partialMsg.Content, e.ID, e.Arguments)
		case piai.EventTextEnd:
			if e.TextSignature != "" {
				partialMsg.Content = setTextSignature(partialMsg.Content, e.TextSignature)
			}
		case piai.EventThinkingEnd:
			if e.ThinkingSignature != "" {
				partialMsg.Content = setThinkingSignature(partialMsg.Content, e.ThinkingSignature)
			}
		case piai.EventDone:
			partialMsg = e.Message
		case piai.EventError:
			return e.Error
		}

		stream.Push(EventMessageUpdate{
			Message:        partialMsg,
			AssistantEvent: evt,
		})
		return nil
	})
	if err != nil {
		return piai.AssistantMessage{}, err
	}

	stream.Push(EventMessageEnd{Message: finalMsg})
	return finalMsg, nil
}

// executeToolCalls executes tool calls and returns the results.
// The second return value indicates if any tool requested termination.
func executeToolCalls(ctx context.Context, config AgentLoopConfig, assistantMsg piai.AssistantMessage, toolCalls []piai.ToolCall, messages []piai.Message, stream *AgentEventStream) ([]piai.ToolResultMessage, bool) {
	// Determine execution mode
	mode := config.ToolExecution
	if mode == "" {
		mode = ToolExecParallel
	}
	// Check if any tool requests sequential
	for _, tc := range toolCalls {
		if tool := findTool(config.Tools, tc.Name); tool != nil && tool.ExecutionMode == ToolExecSequential {
			mode = ToolExecSequential
			break
		}
	}

	if mode == ToolExecSequential {
		return executeToolCallsSequential(ctx, config, assistantMsg, toolCalls, messages, stream)
	}
	return executeToolCallsParallel(ctx, config, assistantMsg, toolCalls, messages, stream)
}

// executeToolCallsSequential executes tool calls one by one.
func executeToolCallsSequential(ctx context.Context, config AgentLoopConfig, assistantMsg piai.AssistantMessage, toolCalls []piai.ToolCall, messages []piai.Message, stream *AgentEventStream) ([]piai.ToolResultMessage, bool) {
	var results []piai.ToolResultMessage
	shouldTerminate := false

	for _, tc := range toolCalls {
		if ctx.Err() != nil {
			break
		}

		result, resultMsg := executeSingleToolCall(ctx, config, assistantMsg, tc, messages, stream)
		results = append(results, resultMsg)
		if result.Terminate {
			shouldTerminate = true
			break
		}
	}

	return results, shouldTerminate
}

// executeToolCallsParallel executes tool calls concurrently.
func executeToolCallsParallel(ctx context.Context, config AgentLoopConfig, assistantMsg piai.AssistantMessage, toolCalls []piai.ToolCall, messages []piai.Message, stream *AgentEventStream) ([]piai.ToolResultMessage, bool) {
	type indexedResult struct {
		index     int
		result    piai.ToolResultMessage
		terminate bool
	}

	results := make([]piai.ToolResultMessage, len(toolCalls))
	var wg sync.WaitGroup
	ch := make(chan indexedResult, len(toolCalls))

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, toolCall piai.ToolCall) {
			defer wg.Done()
			if ctx.Err() != nil {
				return
			}
			agentResult, resultMsg := executeSingleToolCall(ctx, config, assistantMsg, toolCall, messages, stream)
			ch <- indexedResult{index: idx, result: resultMsg, terminate: agentResult.Terminate}
		}(i, tc)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	shouldTerminate := false
	for r := range ch {
		results[r.index] = r.result
		if r.terminate {
			shouldTerminate = true
		}
	}

	return results, shouldTerminate
}

// executeSingleToolCall executes a single tool call through the full lifecycle.
func executeSingleToolCall(ctx context.Context, config AgentLoopConfig, assistantMsg piai.AssistantMessage, tc piai.ToolCall, messages []piai.Message, stream *AgentEventStream) (AgentToolResult, piai.ToolResultMessage) {
	stream.Push(EventToolExecStart{
		ToolCallID: tc.ID,
		ToolName:   tc.Name,
		Args:       tc.Arguments,
	})

	// Prepare
	result, _ := prepareAndExecuteToolCall(ctx, config, assistantMsg, tc, messages, stream)

	// Finalize
	result = finalizeToolCall(config, assistantMsg, tc, messages, result)

	// Emit end event
	resultJSON, _ := json.Marshal(result)
	stream.Push(EventToolExecEnd{
		ToolCallID: tc.ID,
		ToolName:   tc.Name,
		Result:     resultJSON,
		IsError:    result.IsError,
	})

	// Build tool result message
	resultMsg := piai.ToolResultMessage{
		Role:       "tool",
		ToolCallID: tc.ID,
		ToolName:   tc.Name,
		Content:    result.Content,
		IsError:    result.IsError,
	}

	return result, resultMsg
}

// prepareAndExecuteToolCall validates args and executes the tool.
func prepareAndExecuteToolCall(ctx context.Context, config AgentLoopConfig, assistantMsg piai.AssistantMessage, tc piai.ToolCall, messages []piai.Message, stream *AgentEventStream) (AgentToolResult, error) {
	tool := findTool(config.Tools, tc.Name)
	if tool == nil {
		return AgentToolResult{
			Content: []piai.ContentBlock{piai.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Tool not found: %s", tc.Name),
			}},
			IsError: true,
		}, nil
	}

	// BeforeToolCall hook
	if config.BeforeToolCall != nil {
		block := config.BeforeToolCall(BeforeToolCallContext{
			AssistantMessage: assistantMsg,
			ToolCall:         tc,
			Args:             tc.Arguments,
			Messages:         messages,
		})
		if block != nil && block.Block {
			reason := block.Reason
			if reason == "" {
				reason = "Tool execution blocked"
			}
			return AgentToolResult{
				Content: []piai.ContentBlock{piai.TextContent{
					Type: "text",
					Text: reason,
				}},
				IsError: true,
			}, nil
		}
	}

	// Execute
	onUpdate := func(partial json.RawMessage) {
		stream.Push(EventToolExecUpdate{
			ToolCallID:     tc.ID,
			ToolName:       tc.Name,
			Args:           tc.Arguments,
			PartialResult:  partial,
		})
	}

	result, err := tool.Execute(ctx, tc.ID, tc.Arguments, onUpdate)
	if err != nil {
		return AgentToolResult{
			Content: []piai.ContentBlock{piai.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Tool execution error: %v", err),
			}},
			IsError: true,
		}, nil
	}

	return result, nil
}

// finalizeToolCall applies the afterToolCall hook.
func finalizeToolCall(config AgentLoopConfig, assistantMsg piai.AssistantMessage, tc piai.ToolCall, messages []piai.Message, result AgentToolResult) AgentToolResult {
	if config.AfterToolCall == nil {
		return result
	}

	override := config.AfterToolCall(AfterToolCallContext{
		AssistantMessage: assistantMsg,
		ToolCall:         tc,
		Args:             tc.Arguments,
		Result:           result,
		IsError:          result.IsError,
		Messages:         messages,
	})
	if override == nil {
		return result
	}

	if override.Content != nil {
		result.Content = override.Content
	}
	if override.Details != nil {
		result.Details = override.Details
	}
	if override.IsError != nil {
		result.IsError = *override.IsError
	}
	if override.Terminate != nil {
		result.Terminate = *override.Terminate
	}

	return result
}

// --- Helper functions for content manipulation ---

func appendOrUpdateText(blocks []piai.ContentBlock, delta string) []piai.ContentBlock {
	// Find last TextContent block
	for i := len(blocks) - 1; i >= 0; i-- {
		if tc, ok := blocks[i].(piai.TextContent); ok {
			blocks[i] = piai.TextContent{
				Type:          "text",
				Text:          tc.Text + delta,
				TextSignature: tc.TextSignature,
			}
			return blocks
		}
	}
	return append(blocks, piai.TextContent{Type: "text", Text: delta})
}

func appendOrUpdateThinking(blocks []piai.ContentBlock, delta string) []piai.ContentBlock {
	for i := len(blocks) - 1; i >= 0; i-- {
		if tc, ok := blocks[i].(piai.ThinkingContent); ok {
			blocks[i] = piai.ThinkingContent{
				Type:              "thinking",
				Thinking:          tc.Thinking + delta,
				ThinkingSignature: tc.ThinkingSignature,
			}
			return blocks
		}
	}
	return append(blocks, piai.ThinkingContent{Type: "thinking", Thinking: delta})
}

func updateToolCallArgs(blocks []piai.ContentBlock, id string, delta string) []piai.ContentBlock {
	for i, block := range blocks {
		if tc, ok := block.(piai.ToolCall); ok && tc.ID == id {
			blocks[i] = piai.ToolCall{
				Type:      "toolCall",
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: append(tc.Arguments, []byte(delta)...),
			}
			return blocks
		}
	}
	return blocks
}

func finalizeToolCallArgs(blocks []piai.ContentBlock, id string, args json.RawMessage) []piai.ContentBlock {
	for i, block := range blocks {
		if tc, ok := block.(piai.ToolCall); ok && tc.ID == id {
			blocks[i] = piai.ToolCall{
				Type:      "toolCall",
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: args,
			}
			return blocks
		}
	}
	return blocks
}

func setTextSignature(blocks []piai.ContentBlock, sig string) []piai.ContentBlock {
	for i := len(blocks) - 1; i >= 0; i-- {
		if tc, ok := blocks[i].(piai.TextContent); ok {
			blocks[i] = piai.TextContent{
				Type:          "text",
				Text:          tc.Text,
				TextSignature: sig,
			}
			return blocks
		}
	}
	return blocks
}

func setThinkingSignature(blocks []piai.ContentBlock, sig string) []piai.ContentBlock {
	for i := len(blocks) - 1; i >= 0; i-- {
		if tc, ok := blocks[i].(piai.ThinkingContent); ok {
			blocks[i] = piai.ThinkingContent{
				Type:              "thinking",
				Thinking:          tc.Thinking,
				ThinkingSignature: sig,
			}
			return blocks
		}
	}
	return blocks
}

func msgSlice(msgs []piai.ToolResultMessage) []piai.Message {
	result := make([]piai.Message, len(msgs))
	for i, m := range msgs {
		result[i] = m
	}
	return result
}
