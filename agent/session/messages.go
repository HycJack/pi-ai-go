package session

import (
	"fmt"
	"strings"
	"time"

	core "pi-ai-go/core"
)

// --- Session-specific message types (NOT core.Message) ---
// These are stored as session entries and converted to core.Message
// (as UserMessage with structured text) when building LLM context.

// CustomMessage represents an application-defined message.
type CustomMessage struct {
	CustomType string
	Content    string
	Display    bool
	Details    any
	Timestamp  time.Time
}

// BranchSummaryMessage summarizes a conversation branch.
type BranchSummaryMessage struct {
	Summary   string
	FromID    string
	Timestamp time.Time
}

// CompactionSummaryMessage summarizes compacted conversation history.
type CompactionSummaryMessage struct {
	Summary      string
	TokensBefore int
	Timestamp    time.Time
}

// BashExecutionMessage records a shell command execution.
type BashExecutionMessage struct {
	Command            string
	Output             string
	ExitCode           *int
	Cancelled          bool
	Truncated          bool
	FullOutputPath     string
	Timestamp          time.Time
	ExcludeFromContext bool
}

// --- Summary formatting ---

const (
	compactionPrefix = "The conversation history before this point was compacted into the following summary:\n\n<summary>\n"
	compactionSuffix = "\n</summary>"

	branchPrefix = "The following is a summary of a branch that this conversation came back from:\n\n<summary>\n"
	branchSuffix = "\n</summary>"
)

// BashExecutionToText converts a bash execution to human-readable text.
func BashExecutionToText(msg BashExecutionMessage) string {
	text := fmt.Sprintf("Ran `%s`\n", msg.Command)
	if msg.Output != "" {
		text += fmt.Sprintf("```\n%s\n```", msg.Output)
	} else {
		text += "(no output)"
	}
	if msg.Cancelled {
		text += "\n\n(command cancelled)"
	} else if msg.ExitCode != nil && *msg.ExitCode != 0 {
		text += fmt.Sprintf("\n\nCommand exited with code %d", *msg.ExitCode)
	}
	if msg.Truncated && msg.FullOutputPath != "" {
		text += fmt.Sprintf("\n\n[Output truncated. Full output: %s]", msg.FullOutputPath)
	}
	return text
}

// --- ConvertToLlm ---
// Converts a mixed list of core.Message and harness message types
// into pure []core.Message for LLM consumption.

// ConvertibleMessage is any type that can be converted to a core.Message.
// core.Message types pass through; harness types are wrapped as UserMessage.
type ConvertibleMessage interface {
	toCoreMessage() core.Message
}

// wrapMessage converts any message (core or harness) to a core.Message.
func wrapMessage(msg any) core.Message {
	switch m := msg.(type) {
	case core.UserMessage:
		return m
	case core.AssistantMessage:
		return m
	case core.ToolResultMessage:
		return m
	case BashExecutionMessage:
		if m.ExcludeFromContext {
			return nil
		}
		return core.UserMessage{Role: "user", Content: BashExecutionToText(m), Timestamp: m.Timestamp}
	case CustomMessage:
		return core.UserMessage{Role: "user", Content: m.Content, Timestamp: m.Timestamp}
	case BranchSummaryMessage:
		return core.UserMessage{Role: "user", Content: branchPrefix + m.Summary + branchSuffix, Timestamp: m.Timestamp}
	case CompactionSummaryMessage:
		return core.UserMessage{Role: "user", Content: compactionPrefix + m.Summary + compactionSuffix, Timestamp: m.Timestamp}
	default:
		return nil
	}
}

// ConvertToLlm converts harness entries to standard LLM messages.
// The input is a list of SessionTreeEntry; message entries are converted
// to core.Message, compaction/branch entries become summary UserMessages.
func ConvertEntriesToLlm(entries []SessionTreeEntry) []core.Message {
	var result []core.Message
	for _, entry := range entries {
		switch entry.Type {
		case EntryMessage:
			if entry.Message != nil {
				result = append(result, entry.Message)
			}
		case EntryCustomMessage:
			result = append(result, core.UserMessage{
				Role:      "user",
				Content:   fmt.Sprintf("[%s] %v", entry.CustomType, entry.Content),
				Timestamp: entry.Timestamp,
			})
		case EntryBranchSummary:
			result = append(result, core.UserMessage{
				Role:      "user",
				Content:   branchPrefix + entry.Summary + branchSuffix,
				Timestamp: entry.Timestamp,
			})
		case EntryCompaction:
			result = append(result, core.UserMessage{
				Role:      "user",
				Content:   compactionPrefix + entry.CompactionSummary + compactionSuffix,
				Timestamp: entry.Timestamp,
			})
		}
	}
	return result
}

// SerializeMessagesForSummary converts messages to text for LLM summarization.
func SerializeMessagesForSummary(messages []core.Message) string {
	var parts []string
	for _, msg := range messages {
		switch m := msg.(type) {
		case core.UserMessage:
			parts = append(parts, fmt.Sprintf("User: %v", m.Content))
		case core.AssistantMessage:
			text := extractAssistantText(m)
			if text != "" {
				parts = append(parts, fmt.Sprintf("Assistant: %s", text))
			}
		case core.ToolResultMessage:
			text := extractToolResultText(m)
			parts = append(parts, fmt.Sprintf("Tool [%s]: %s", m.ToolName, text))
		}
	}
	return strings.Join(parts, "\n\n")
}

func extractAssistantText(msg core.AssistantMessage) string {
	var parts []string
	for _, block := range msg.Content {
		if tc, ok := block.(core.TextContent); ok && tc.Text != "" {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func extractToolResultText(msg core.ToolResultMessage) string {
	var parts []string
	for _, block := range msg.Content {
		if tc, ok := block.(core.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}
