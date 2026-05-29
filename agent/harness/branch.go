package harness

import (
	"context"
	"fmt"

	core "pi-ai-go/core"
	"pi-ai-go/ai"
)

// BranchSummaryResult holds the result of branch summarization.
type BranchSummaryResult struct {
	Summary string
	FromID  string
}

// SummarizeBranch generates a summary of a conversation branch using LLM.
// This is used when the agent explores a path and needs to backtrack.
func SummarizeBranch(
	ctx context.Context,
	model core.Model,
	messages []core.Message,
	branchID string,
	streamOpts ...core.SimpleStreamOptions,
) (*BranchSummaryResult, error) {
	if len(messages) == 0 {
		return nil, &HarnessError{
			Code:    ErrInvalid,
			Message: "no messages to summarize",
		}
	}

	serialized := SerializeMessagesForSummary(messages)
	prompt := fmt.Sprintf(`Summarize the following conversation branch concisely.
Focus on: what was attempted, what was learned, and why the branch ended.
This summary will be used to provide context when continuing the conversation.

Branch:
%s`, serialized)

	var opts []core.SimpleStreamOptions
	if len(streamOpts) > 0 {
		opts = streamOpts
	}

	summaryMsg, err := ai.CompleteSimple(ctx, model, []core.Message{
		core.UserMessage{Content: prompt},
	}, opts...)
	if err != nil {
		return nil, &HarnessError{
			Code:    ErrSummarization,
			Message: "branch summarization failed",
			Err:     err,
		}
	}

	summary := extractText(summaryMsg)
	if summary == "" {
		return nil, &HarnessError{
			Code:    ErrSummarization,
			Message: "LLM returned empty branch summary",
		}
	}

	return &BranchSummaryResult{
		Summary: summary,
		FromID:  branchID,
	}, nil
}

// SummarizeBranchSession generates and stores a branch summary in the session.
func SummarizeBranchSession(
	ctx context.Context,
	session *Session,
	model core.Model,
	branchMessages []core.Message,
	branchID string,
	streamOpts ...core.SimpleStreamOptions,
) (*BranchSummaryResult, error) {
	result, err := SummarizeBranch(ctx, model, branchMessages, branchID, streamOpts...)
	if err != nil {
		return nil, err
	}

	entry := SessionTreeEntry{
		ID:        GenerateID(),
		Type:      EntryBranchSummary,
		Timestamp: timeNow(),
		Summary:   result.Summary,
		FromID:    result.FromID,
	}

	if err := session.Append(entry); err != nil {
		return nil, err
	}

	return result, nil
}
