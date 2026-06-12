package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	core "pi-ai-go/core"
	"pi-ai-go/llm"
)

// SummarizeModel is the model used for LLM-based compaction. If nil,
// the compaction falls back to SlidingWindowCompact.
type SummarizeModel struct {
	Model  core.Model
	Stream StreamFn
}

// SummarizePrompt is the canonical summarization prompt. Callers may
// override it via AgentLoopConfig.SummarizePrompt.
const SummarizePrompt = `You are summarizing part of a long conversation between a user and an AI agent for context compaction.

Your job:
1. Preserve the user's goals, preferences, and any concrete tasks they asked for.
2. Preserve the agent's decisions, tool results, and any state changes (e.g. files modified, commands run).
3. Preserve the most recent intent / unfinished work.
4. Drop chit-chat, redundant clarifications, and obviously resolved sub-tasks.
5. Keep it concise: target ~10%% of the original token count.

Format the summary as a structured list of bullet points. Use markdown headings for major topics. Do not invent facts; if a fact is unclear, omit it.

---

CONVERSATION:

%s

---

SUMMARY:`

// CompactByStrategy is the dispatch entry point. It picks the strategy
// from the config and returns a compacted slice.
//
// drop=0 and a nil error mean "no compaction needed".
// drop>0 means drop the first `drop` messages and (if the strategy is
// summarize) prepend a summary user message.
func CompactByStrategy(
	ctx context.Context,
	messages []core.Message,
	model core.Model,
	policy ContextPolicy,
	sm *SummarizeModel,
) ([]core.Message, int, error) {
	policy = policy.withDefaults()

	cw := NewContextWindow(model, policy)
	cw.Used = EstimateMessagesTokens(messages, model)

	if !cw.NeedsCompaction() {
		return messages, 0, nil
	}

	switch policy.Strategy {
	case CompactionStrategySlidingWindow, "":
		compacted, dropped := SlidingWindowCompact(messages, model, cw.SoftTokens, policy.MinTailMessages)
		return compacted, dropped, nil

	case CompactionStrategySummarize:
		if sm == nil || sm.Model.ID == "" {
			// Fall back to sliding window if no summarize model.
			compacted, dropped := SlidingWindowCompact(messages, model, cw.SoftTokens, policy.MinTailMessages)
			return compacted, dropped, nil
		}
		return summarizeAndCompact(ctx, messages, model, sm, policy, cw)
	}

	return messages, 0, nil
}

// summarizeAndCompact uses an LLM to summarize the to-drop portion of
// the messages, then pre-pends the summary as a UserMessage at the cut
// point.
func summarizeAndCompact(
	ctx context.Context,
	messages []core.Message,
	model core.Model,
	sm *SummarizeModel,
	policy ContextPolicy,
	cw *ContextWindow,
) ([]core.Message, int, error) {
	// Decide what to drop. Aim to bring the slice below cw.SoftTokens.
	// We try drop counts in [1, len-minTail] until the projected
	// post-summary usage fits.
	minTail := policy.MinTailMessages
	maxDrop := len(messages) - minTail
	if maxDrop <= 0 {
		return messages, 0, nil
	}

	// Heuristic: drop roughly (Used - SoftTokens) / avg-message-size
	// messages, then adjust. We start at half the slice and binary-search.
	lo, hi := 1, maxDrop
	bestDrop := maxDrop
	for lo <= hi {
		mid := (lo + hi) / 2
		kept := append([]core.Message{}, messages[mid:]...)
		if EstimateMessagesTokens(kept, model) <= cw.SoftTokens {
			bestDrop = mid
			hi = mid - 1
		} else {
			lo = mid + 1
		}
	}

	toSummarize := messages[:bestDrop]
	summary, err := callSummarizeLLM(ctx, sm, toSummarize, SummarizePrompt)
	if err != nil {
		// Fall back to sliding window on LLM error.
		compacted, dropped := SlidingWindowCompact(messages, model, cw.SoftTokens, policy.MinTailMessages)
		return compacted, dropped, nil
	}

	out := CompactSummarizeInPlace(messages, summary, bestDrop, time.Now())
	return out, bestDrop, nil
}

// callSummarizeLLM runs a one-shot LLM completion against the
// summarize model and returns the assistant text.
func callSummarizeLLM(ctx context.Context, sm *SummarizeModel, msgs []core.Message, promptTemplate string) (string, error) {
	if sm == nil || sm.Model.ID == "" {
		return "", fmt.Errorf("agent: summarize model is nil")
	}
	// Serialize messages for the prompt.
	serialized := serializeForPrompt(msgs)
	prompt := fmt.Sprintf(promptTemplate, serialized)

	streamFn := sm.Stream
	if streamFn == nil {
		streamFn = func(ctx context.Context, m core.Model, c core.Context, o core.SimpleStreamOptions) (*core.EventStream[core.AssistantMessageEvent, core.AssistantMessage], error) {
			return llm.StreamSimpleWithContext(ctx, m, c, o)
		}
	}

	llmCtx := core.Context{
		Messages: []core.Message{
			core.UserMessage{
				Role:      "user",
				Content:   prompt,
				Timestamp: time.Now(),
			},
		},
	}
	stream, err := streamFn(ctx, sm.Model, llmCtx, core.SimpleStreamOptions{})
	if err != nil {
		return "", err
	}
	final, err := stream.Result()
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, b := range final.Content {
		if t, ok := b.(core.TextContent); ok {
			sb.WriteString(t.Text)
		}
	}
	return sb.String(), nil
}

// serializeForPrompt flattens a slice of messages into a human-readable
// transcript suitable for inclusion in a summarization prompt.
func serializeForPrompt(messages []core.Message) string {
	var sb strings.Builder
	for _, m := range messages {
		switch mm := m.(type) {
		case core.UserMessage:
			sb.WriteString("USER: ")
			sb.WriteString(stringifyContent(mm.Content))
			sb.WriteString("\n\n")
		case core.AssistantMessage:
			sb.WriteString("ASSISTANT: ")
			for _, b := range mm.Content {
				if t, ok := b.(core.TextContent); ok {
					sb.WriteString(t.Text)
				}
			}
			sb.WriteString("\n\n")
		case core.ToolResultMessage:
			fmt.Fprintf(&sb, "TOOL[%s, id=%s, isError=%t]: ", mm.ToolName, mm.ToolCallID, mm.IsError)
			for _, b := range mm.Content {
				if t, ok := b.(core.TextContent); ok {
					sb.WriteString(t.Text)
				}
			}
			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}

func stringifyContent(c any) string {
	if c == nil {
		return ""
	}
	if s, ok := c.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", c)
}
