package session

import (
	"time"
	"context"
	"fmt"

	core "pi-ai-go/core"
	"pi-ai-go/llm"
)

// CompactionResult holds the result of a compaction operation.
type CompactionResult struct {
	Summary     string
	TokensSaved int
	EntriesKept int
}

// NeedsCompaction checks if the session needs compaction based on token usage.
func NeedsCompaction(usage core.Usage, settings CompactionSettings) bool {
	total := usage.Input + usage.Output + usage.CacheRead + usage.CacheWrite
	return total > settings.MaxTokensBeforeCompaction
}

// Compact performs context compaction on the given messages.
// It uses the provided LLM model to generate a summary of older messages.
func Compact(
	ctx context.Context,
	model core.Model,
	messages []core.Message,
	settings CompactionSettings,
	streamOpts ...core.SimpleStreamOptions,
) (*CompactionResult, error) {
	if len(messages) <= settings.MinMessagesToKeep {
		return nil, &SessionError{
			Code:    ErrInvalid,
			Message: "not enough messages to compact",
		}
	}

	splitIdx := len(messages) - settings.MinMessagesToKeep
	if splitIdx <= 0 {
		splitIdx = 1
	}

	toSummarize := messages[:splitIdx]
	toKeep := messages[splitIdx:]

	serialized := SerializeMessagesForSummary(toSummarize)
	prompt := fmt.Sprintf(settings.SummaryPrompt, serialized)

	var opts []core.SimpleStreamOptions
	if len(streamOpts) > 0 {
		opts = streamOpts
	}

	summaryMsg, err := llm.CompleteSimple(ctx, model, []core.Message{
		core.UserMessage{Content: prompt},
	}, opts...)
	if err != nil {
		return nil, &SessionError{
			Code:    ErrSummarization,
			Message: "LLM summarization failed",
			Err:     err,
		}
	}

	summary := extractText(summaryMsg)
	if summary == "" {
		return nil, &SessionError{
			Code:    ErrSummarization,
			Message: "LLM returned empty summary",
		}
	}

	tokensBefore := estimateTokens(messages)
	tokensAfter := estimateTokens(toKeep) + estimateSingleText(summary)

	return &CompactionResult{
		Summary:     summary,
		TokensSaved: tokensBefore - tokensAfter,
		EntriesKept: len(toKeep),
	}, nil
}

// CompactSession performs compaction on a session, appending the compaction entry.
func CompactSession(
	ctx context.Context,
	session *Session,
	model core.Model,
	settings CompactionSettings,
	streamOpts ...core.SimpleStreamOptions,
) (*CompactionResult, error) {
	ctx2 := session.BuildContext()
	messages := ctx2.Messages

	result, err := Compact(ctx, model, messages, settings, streamOpts...)
	if err != nil {
		return nil, err
	}

	entries := session.Entries()
	keptStart := len(entries) - result.EntriesKept
	if keptStart < 0 {
		keptStart = 0
	}
	firstKeptID := ""
	if keptStart < len(entries) {
		firstKeptID = entries[keptStart].ID
	}

	compactionEntry := SessionTreeEntry{
		ID:                GenerateID(),
		Type:              EntryCompaction,
		Timestamp:         timeNow(),
		CompactionSummary: result.Summary,
		TokensBefore:      estimateTokens(messages),
		FirstKeptEntryID:  firstKeptID,
	}

	if err := session.Append(compactionEntry); err != nil {
		return nil, err
	}

	return result, nil
}

// --- internal helpers ---

func extractText(msg core.AssistantMessage) string {
	var parts []string
	for _, block := range msg.Content {
		if tc, ok := block.(core.TextContent); ok && tc.Text != "" {
			parts = append(parts, tc.Text)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += "\n" + p
	}
	return result
}

func estimateTokens(messages []core.Message) int {
	total := 0
	for _, msg := range messages {
		switch m := msg.(type) {
		case core.UserMessage:
			total += estimateAnyContent(m.Content)
		case core.AssistantMessage:
			for _, block := range m.Content {
				if tc, ok := block.(core.TextContent); ok {
					total += len(tc.Text)
				}
			}
		case core.ToolResultMessage:
			for _, block := range m.Content {
				if tc, ok := block.(core.TextContent); ok {
					total += len(tc.Text)
				}
			}
		}
	}
	return total / 4
}

func estimateSingleText(text string) int {
	return len(text) / 4
}

func estimateAnyContent(content any) int {
	switch c := content.(type) {
	case string:
		return len(c) / 4
	case []core.ContentBlock:
		total := 0
		for _, block := range c {
			if tc, ok := block.(core.TextContent); ok {
				total += len(tc.Text)
			}
		}
		return total / 4
	default:
		return 0
	}
}

func timeNow() time.Time {
	return time.Now()
}
