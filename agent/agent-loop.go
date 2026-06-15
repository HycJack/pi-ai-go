package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	core "pi-ai-go/core"
	"pi-ai-go/llm"
)

// AgentEventStream is the type alias for the agent event stream.
type AgentEventStream = core.EventStream[AgentEvent, []core.Message]

// AgentLoop starts a new agent run with the given prompt messages.
func AgentLoop(ctx context.Context, msgs []core.Message, config AgentLoopConfig) *AgentEventStream {
	stream := core.NewEventStream[AgentEvent, []core.Message]()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				stream.Error(fmt.Errorf("agent: panic: %v", r))
			}
		}()

		stream.Push(EventAgentStart{})

		// NOTE: We intentionally do NOT emit EventMessageStart/EventMessageEnd
		// for prompt messages here. These messages are already part of the
		// conversation history and are included in the final result from
		// stream.Result(). Emitting events for them would cause the Agent's
		// processStream to duplicate them (since Run already appends prompts
		// to a.state.Messages before calling AgentLoop).

		messages := make([]core.Message, len(msgs))
		copy(messages, msgs)

		runLoop(ctx, config, messages, stream)
	}()

	return stream
}

// AgentLoopContinue resumes an agent run from existing context.
// The last message must be a user or toolResult message.
func AgentLoopContinue(ctx context.Context, config AgentLoopConfig, messages []core.Message) *AgentEventStream {
	stream := core.NewEventStream[AgentEvent, []core.Message]()

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
func runLoop(ctx context.Context, config AgentLoopConfig, messages []core.Message, stream *AgentEventStream) {
	// --- oh-my-pi parity wiring ---
	yielder := config.Yielder
	if yielder == nil {
		yielder = NewYielder(YieldConfig{})
	}
	yielder.Reset()

	collector := config.Collector
	if collector == nil {
		collector = NewRunCollector()
	}

	// Helper that finalizes a run (attach summary to the event and end
	// the stream). Using a closure keeps the call sites uniform.
	finalize := func() {
		collector.MarkRunEnded()
		summary, coverage := collector.Snapshot()
		stream.Push(EventAgentEnd{
			Messages: messages,
			Summary:  &summary,
			Coverage: &coverage,
		})
		stream.End(messages)
	}

	for {
		// Inner loop: process tool calls and steering messages
		for {
			if ctx.Err() != nil {
				finalize()
				return
			}

			// Cooperative yield between turns.
			if err := yielder.YieldIfDue(ctx); err != nil {
				finalize()
				return
			}

			// Inject steering messages before emitting turn_start.
			// Two sources: a MessageQueue (preferred, new) or the
			// legacy GetSteeringMessages callback.
			hasSteering := false
			if config.Queue != nil {
				steering := config.Queue.DrainSteering()
				if len(steering) > 0 {
					messages = append(messages, steering...)
					hasSteering = true
				}
			} else if config.GetSteeringMessages != nil {
				steering := config.GetSteeringMessages()
				if len(steering) > 0 {
					messages = append(messages, steering...)
					hasSteering = true
				}
			}

			stream.Push(EventTurnStart{})

			// Stream assistant response.
			assistantMsg, err := streamAssistantResponse(ctx, config, messages, stream)
			if err != nil {
				collector.RecordError(err)
				collector.RecordStep(config.Model)
				collector.SetStopReason(core.StopError)
				errMsg := core.AssistantMessage{
					Role:         "assistant",
					StopReason:   core.StopError,
					ErrorMessage: err.Error(),
				}
				messages = append(messages, errMsg)
				stream.Push(EventTurnEnd{Message: errMsg})
				finalize()
				return
			}

			messages = append(messages, assistantMsg)
			collector.RecordStep(config.Model)
			collector.RecordUsage(assistantMsg.Usage)
			collector.SetStopReason(assistantMsg.StopReason)

			// Context-window management: if the policy is set and
			// the cumulative usage has crossed the soft limit, run
			// compaction BEFORE the next tool calls fire. The hard
			// limit forces a compaction regardless of the strategy.
			if config.ContextPolicy != nil {
				if newMsgs, dropped, ok := maybeCompact(ctx, &config, messages, assistantMsg.Usage, stream); ok {
					messages = newMsgs
					collector.SetStopReason(assistantMsg.StopReason) // unchanged
					_ = dropped                                      // already emitted
				}
			}

			// Check for error/aborted stop reasons
			if assistantMsg.StopReason == core.StopError || assistantMsg.StopReason == core.StopAborted {
				stream.Push(EventTurnEnd{Message: assistantMsg})
				finalize()
				return
			}

			// Extract tool calls
			toolCalls := extractToolCalls(assistantMsg)

			// Execute tool calls if any
			var toolResults []core.ToolResultMessage
			shouldTerminate := false
			if len(toolCalls) > 0 {
				toolResults, shouldTerminate = executeToolCalls(ctx, config, assistantMsg, toolCalls, messages, stream)
				messages = append(messages, msgSlice(toolResults)...)
				// Record tool calls to collector
				for _, result := range toolResults {
					collector.RecordToolCall(result.ToolName, result.IsError)
				}
			}

			stream.Push(EventTurnEnd{
				Message:     assistantMsg,
				ToolResults: toolResults,
			})

			// If all tools requested termination, stop the loop
			if shouldTerminate {
				finalize()
				return
			}

			// Prepare next turn hook
			if config.PrepareNextTurn != nil {
				config.PrepareNextTurn(&config, assistantMsg, toolResults, messages)
			}

			// Should stop after turn hook
			if config.ShouldStopAfterTurn != nil && config.ShouldStopAfterTurn(assistantMsg, toolResults) {
				finalize()
				return
			}

			// If no tool calls and no steering was injected, exit inner loop
			if len(toolCalls) == 0 && !hasSteering {
				break // Exit inner loop, check follow-up
			}
			// Has tool calls or had steering, continue inner loop
		}

		// Check follow-up messages (outer loop). Source preference is
		// the same as for steering: Queue first, legacy callback second.
		hasFollowUp := false
		if config.Queue != nil {
			followUp := config.Queue.DrainFollowUp()
			if len(followUp) > 0 {
				messages = append(messages, followUp...)
				hasFollowUp = true
			}
		} else if config.GetFollowUpMessages != nil {
			followUp := config.GetFollowUpMessages()
			if len(followUp) > 0 {
				messages = append(messages, followUp...)
				hasFollowUp = true
			}
		}

		if !hasFollowUp {
			finalize()
			return
		}
		// Has follow-up, continue outer loop
	}
}

// streamAssistantResponse streams an LLM response and returns the final message.
func streamAssistantResponse(ctx context.Context, config AgentLoopConfig, messages []core.Message, stream *AgentEventStream) (core.AssistantMessage, error) {
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
		streamFn = func(ctx context.Context, m core.Model, c core.Context, o core.SimpleStreamOptions) (*core.EventStream[core.AssistantMessageEvent, core.AssistantMessage], error) {
			return llm.StreamSimpleWithContext(ctx, m, c, o)
		}
	}

	llmStream, err := streamFn(ctx, config.Model, llmCtx, opts)
	if err != nil {
		return core.AssistantMessage{}, err
	}
	if llmStream == nil {
		return core.AssistantMessage{}, fmt.Errorf("agent: stream function returned nil stream")
	}

	// Partial message for streaming updates
	var partialMsg core.AssistantMessage
	partialMsg.Role = "assistant"
	partialMsg.Timestamp = time.Now()

	stream.Push(EventMessageStart{Message: partialMsg})

	// Iterate over LLM events
	finalMsg, err := llmStream.ForEach(ctx, func(evt core.AssistantMessageEvent) error {
		// Update partial message based on event type
		switch e := evt.(type) {
		case core.EventStart:
			partialMsg.API = e.API
			partialMsg.Provider = e.Provider
			partialMsg.Model = e.Model
		case core.EventTextDelta:
			partialMsg.Content = appendOrUpdateText(partialMsg.Content, e.Delta)
		case core.EventThinkingDelta:
			partialMsg.Content = appendOrUpdateThinking(partialMsg.Content, e.Delta)
		case core.EventToolCallStart:
			partialMsg.Content = append(partialMsg.Content, core.ToolCall{
				Type: "toolCall",
				ID:   e.ID,
				Name: e.Name,
			})
		case core.EventToolCallDelta:
			partialMsg.Content = updateToolCallArgs(partialMsg.Content, e.ID, e.ArgumentsDelta)
		case core.EventToolCallEnd:
			partialMsg.Content = finalizeToolCallArgs(partialMsg.Content, e.ID, e.Arguments)
		case core.EventTextEnd:
			if e.TextSignature != "" {
				partialMsg.Content = setTextSignature(partialMsg.Content, e.TextSignature)
			}
		case core.EventThinkingEnd:
			if e.ThinkingSignature != "" {
				partialMsg.Content = setThinkingSignature(partialMsg.Content, e.ThinkingSignature)
			}
		case core.EventDone:
			partialMsg = e.Message
		case core.EventError:
			return e.Error
		}

		stream.Push(EventMessageUpdate{
			Message:        partialMsg,
			AssistantEvent: evt,
		})
		return nil
	})
	if err != nil {
		return core.AssistantMessage{}, err
	}

	stream.Push(EventMessageEnd{Message: finalMsg})

	// Detect overflow AFTER the stream has resolved so partial usage
	// data is still available. This catches silent-overflow cases that
	// the provider may report with a generic 4xx and no body.
	if sig := detectOverflowFromMessage(finalMsg, config.Model); sig != nil {
		if config.OnOverflow != nil {
			if hookErr := config.OnOverflow(sig); hookErr != nil {
				return finalMsg, hookErr
			}
		}
	}
	return finalMsg, nil
}

// detectOverflowFromMessage returns a non-nil OverflowSignal when the
// stream-final message looks like it overflowed. It uses the same
// patterns as core.Retry's IsContextOverflowError, but additionally
// checks the partial usage / context window pair (silent overflow).
func detectOverflowFromMessage(msg core.AssistantMessage, model core.Model) *OverflowSignal {
	if msg.StopReason == core.StopError && core.IsContextOverflowError(parseErrFromMessage(msg)) {
		return &OverflowSignal{
			Provider:      model.Provider,
			ModelID:       model.ID,
			ContextWindow: model.ContextWindow,
			Usage:         msg.Usage.Input,
			OriginalErr:   parseErrFromMessage(msg),
			Source:        "stream",
		}
	}
	// Silent overflow: usage > context window.
	if model.ContextWindow > 0 && msg.Usage.Input > model.ContextWindow {
		return &OverflowSignal{
			Provider:      model.Provider,
			ModelID:       model.ID,
			ContextWindow: model.ContextWindow,
			Usage:         msg.Usage.Input,
			Source:        "silent",
		}
	}
	return nil
}

func parseErrFromMessage(msg core.AssistantMessage) error {
	if msg.ErrorMessage == "" {
		return nil
	}
	return &overflowStringError{msg: msg.ErrorMessage}
}

// overflowStringError wraps a plain error message so it round-trips
// through core.IsContextOverflowError's pattern check.
type overflowStringError struct{ msg string }

func (e *overflowStringError) Error() string { return e.msg }

// maybeCompact runs the configured compaction strategy when usage
// crosses the policy's soft or hard limit. It returns the (possibly
// compacted) message slice, the number of messages dropped, and a
// `ok` flag indicating whether anything was actually done.
//
// The function is a no-op when no ContextPolicy is set, when the model
// has a zero context window, or when usage is below the soft limit.
func maybeCompact(
	ctx context.Context,
	config *AgentLoopConfig,
	messages []core.Message,
	lastUsage core.Usage,
	stream *AgentEventStream,
) (newMsgs []core.Message, dropped int, didCompact bool) {
	if config.ContextPolicy == nil {
		return messages, 0, false
	}
	cw := NewContextWindow(config.Model, *config.ContextPolicy)
	if cw.Effective == 0 {
		return messages, 0, false
	}
	// Use the LLM-reported input as the source of truth; fall back
	// to the local estimate if the LLM did not report usage.
	cw.Used = lastUsage.Input
	if cw.Used == 0 {
		cw.Used = EstimateMessagesTokens(messages, config.Model)
	}

	if !cw.NeedsCompaction() {
		return messages, 0, false
	}

	triggeredBy := "soft"
	if cw.MustCompact() {
		triggeredBy = "hard"
	}
	policy := *config.ContextPolicy
	compacted, drop, err := CompactByStrategy(ctx, messages, config.Model, policy, config.SummarizeModel)
	if err != nil || drop == 0 {
		return messages, 0, false
	}

	tokensBefore := cw.Used
	tokensAfter := EstimateMessagesTokens(compacted, config.Model)

	stream.Push(EventCompaction{
		Strategy:     policy.withDefaults().Strategy,
		TokensBefore: tokensBefore,
		TokensAfter:  tokensAfter,
		Dropped:      drop,
		TriggeredBy:  triggeredBy,
	})
	if config.OnCompaction != nil {
		config.OnCompaction(CompactionEvent{
			Strategy:     policy.withDefaults().Strategy,
			TokensBefore: tokensBefore,
			TokensAfter:  tokensAfter,
			Dropped:      drop,
			TriggeredBy:  triggeredBy,
		})
	}
	return compacted, drop, true
}

// executeToolCalls executes tool calls and returns the results.
// The second return value indicates if any tool requested termination.
func executeToolCalls(ctx context.Context, config AgentLoopConfig, assistantMsg core.AssistantMessage, toolCalls []core.ToolCall, messages []core.Message, stream *AgentEventStream) ([]core.ToolResultMessage, bool) {
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
func executeToolCallsSequential(ctx context.Context, config AgentLoopConfig, assistantMsg core.AssistantMessage, toolCalls []core.ToolCall, messages []core.Message, stream *AgentEventStream) ([]core.ToolResultMessage, bool) {
	var results []core.ToolResultMessage
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
func executeToolCallsParallel(ctx context.Context, config AgentLoopConfig, assistantMsg core.AssistantMessage, toolCalls []core.ToolCall, messages []core.Message, stream *AgentEventStream) ([]core.ToolResultMessage, bool) {
	type indexedResult struct {
		index     int
		result    core.ToolResultMessage
		terminate bool
	}

	results := make([]core.ToolResultMessage, len(toolCalls))
	var wg sync.WaitGroup
	ch := make(chan indexedResult, len(toolCalls))

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, toolCall core.ToolCall) {
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
func executeSingleToolCall(ctx context.Context, config AgentLoopConfig, assistantMsg core.AssistantMessage, tc core.ToolCall, messages []core.Message, stream *AgentEventStream) (AgentToolResult, core.ToolResultMessage) {
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
	resultMsg := core.ToolResultMessage{
		Role:       "tool",
		ToolCallID: tc.ID,
		ToolName:   tc.Name,
		Content:    result.Content,
		IsError:    result.IsError,
	}

	return result, resultMsg
}

// prepareAndExecuteToolCall validates args and executes the tool.
func prepareAndExecuteToolCall(ctx context.Context, config AgentLoopConfig, assistantMsg core.AssistantMessage, tc core.ToolCall, messages []core.Message, stream *AgentEventStream) (AgentToolResult, error) {
	tool := findTool(config.Tools, tc.Name)
	if tool == nil {
		return AgentToolResult{
			Content: []core.ContentBlock{core.TextContent{
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
				Content: []core.ContentBlock{core.TextContent{
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
			ToolCallID:    tc.ID,
			ToolName:      tc.Name,
			Args:          tc.Arguments,
			PartialResult: partial,
		})
	}

	result, err := tool.Execute(ctx, tc.ID, tc.Arguments, onUpdate)
	if err != nil {
		return AgentToolResult{
			Content: []core.ContentBlock{core.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Tool execution error: %v", err),
			}},
			IsError: true,
		}, nil
	}

	return result, nil
}

// finalizeToolCall applies the afterToolCall hook.
func finalizeToolCall(config AgentLoopConfig, assistantMsg core.AssistantMessage, tc core.ToolCall, messages []core.Message, result AgentToolResult) AgentToolResult {
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

func appendOrUpdateText(blocks []core.ContentBlock, delta string) []core.ContentBlock {
	// Find last TextContent block
	for i := len(blocks) - 1; i >= 0; i-- {
		if tc, ok := blocks[i].(core.TextContent); ok {
			blocks[i] = core.TextContent{
				Type:          "text",
				Text:          tc.Text + delta,
				TextSignature: tc.TextSignature,
			}
			return blocks
		}
	}
	return append(blocks, core.TextContent{Type: "text", Text: delta})
}

func appendOrUpdateThinking(blocks []core.ContentBlock, delta string) []core.ContentBlock {
	for i := len(blocks) - 1; i >= 0; i-- {
		if tc, ok := blocks[i].(core.ThinkingContent); ok {
			blocks[i] = core.ThinkingContent{
				Type:              "thinking",
				Thinking:          tc.Thinking + delta,
				ThinkingSignature: tc.ThinkingSignature,
			}
			return blocks
		}
	}
	return append(blocks, core.ThinkingContent{Type: "thinking", Thinking: delta})
}

func updateToolCallArgs(blocks []core.ContentBlock, id string, delta string) []core.ContentBlock {
	for i, block := range blocks {
		if tc, ok := block.(core.ToolCall); ok && tc.ID == id {
			blocks[i] = core.ToolCall{
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

func finalizeToolCallArgs(blocks []core.ContentBlock, id string, args json.RawMessage) []core.ContentBlock {
	for i, block := range blocks {
		if tc, ok := block.(core.ToolCall); ok && tc.ID == id {
			blocks[i] = core.ToolCall{
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

func setTextSignature(blocks []core.ContentBlock, sig string) []core.ContentBlock {
	for i := len(blocks) - 1; i >= 0; i-- {
		if tc, ok := blocks[i].(core.TextContent); ok {
			blocks[i] = core.TextContent{
				Type:          "text",
				Text:          tc.Text,
				TextSignature: sig,
			}
			return blocks
		}
	}
	return blocks
}

func setThinkingSignature(blocks []core.ContentBlock, sig string) []core.ContentBlock {
	for i := len(blocks) - 1; i >= 0; i-- {
		if tc, ok := blocks[i].(core.ThinkingContent); ok {
			blocks[i] = core.ThinkingContent{
				Type:              "thinking",
				Thinking:          tc.Thinking,
				ThinkingSignature: sig,
			}
			return blocks
		}
	}
	return blocks
}

func msgSlice(msgs []core.ToolResultMessage) []core.Message {
	result := make([]core.Message, len(msgs))
	for i, m := range msgs {
		result[i] = m
	}
	return result
}
