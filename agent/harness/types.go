// Package harness provides a production-grade agent framework with session
// persistence, context compaction, skills, and prompt templates.
package harness

import (
	"fmt"
	"time"

	core "pi-ai-go/core"
)

// --- Error types ---

// ErrorCode is a stable, backend-independent error code.
type ErrorCode string

const (
	ErrAborted         ErrorCode = "aborted"
	ErrNotFound        ErrorCode = "not_found"
	ErrPermission      ErrorCode = "permission_denied"
	ErrInvalid         ErrorCode = "invalid"
	ErrTimeout         ErrorCode = "timeout"
	ErrStorage         ErrorCode = "storage"
	ErrSummarization   ErrorCode = "summarization_failed"
	ErrInvalidSession  ErrorCode = "invalid_session"
	ErrInvalidEntry    ErrorCode = "invalid_entry"
	ErrUnknown         ErrorCode = "unknown"
)

// HarnessError is the base error type for harness operations.
type HarnessError struct {
	Code    ErrorCode
	Message string
	Path    string
	Err     error
}

func (e *HarnessError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Path, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *HarnessError) Unwrap() error { return e.Err }

// SessionError is returned by session operations.
type SessionError = HarnessError

// CompactionError is returned by compaction operations.
type CompactionError = HarnessError

// --- Skill ---

// Skill represents a skill loaded from a SKILL.md file or provided by an application.
type Skill struct {
	Name                 string // Stable skill name
	Description          string // Short model-visible description
	Content              string // Full skill instructions
	FilePath             string // Absolute path to the skill file
	DisableModelInvocation bool // Exclude from model-visible skill lists
}

// --- PromptTemplate ---

// PromptTemplate is a prompt template with variable placeholders.
type PromptTemplate struct {
	Name        string // Stable template name
	Description string // Optional description
	Content     string // Template content with {{variable}} placeholders
}

// FormatInvocation formats a skill invocation prompt.
func FormatInvocation(skill Skill, additionalInstructions string) string {
	s := fmt.Sprintf("<skill name=%q location=%q>\nReferences are relative to %s.\n\n%s\n</skill>",
		skill.Name, skill.FilePath, dirName(skill.FilePath), skill.Content)
	if additionalInstructions != "" {
		return s + "\n\n" + additionalInstructions
	}
	return s
}

// FormatTemplateInvocation formats a prompt template invocation with variable substitution.
func FormatTemplateInvocation(tmpl PromptTemplate, args map[string]string) string {
	result := tmpl.Content
	for k, v := range args {
		placeholder := "{{" + k + "}}"
		result = replaceAll(result, placeholder, v)
	}
	return result
}

func dirName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}

func replaceAll(s, old, new string) string {
	result := ""
	for {
		idx := indexOf(s, old)
		if idx < 0 {
			return result + s
		}
		result += s[:idx] + new
		s = s[idx+len(old):]
	}
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// --- Session Tree Entry ---

// EntryType identifies the kind of session tree entry.
type EntryType string

const (
	EntryMessage         EntryType = "message"
	EntryCustomMessage   EntryType = "custom_message"
	EntryBranchSummary   EntryType = "branch_summary"
	EntryCompaction      EntryType = "compaction"
	EntryModelChange     EntryType = "model_change"
	EntryThinkingChange  EntryType = "thinking_level_change"
	EntrySessionInfo     EntryType = "session_info"
	EntryLabel           EntryType = "label"
)

// SessionTreeEntry is a single entry in the session tree.
type SessionTreeEntry struct {
	ID        string    `json:"id"`
	Type      EntryType `json:"type"`
	Timestamp time.Time `json:"timestamp"`

	// For EntryMessage
	Message core.Message `json:"message,omitempty"`

	// For EntryCustomMessage
	CustomType string `json:"customType,omitempty"`
	Content    any    `json:"content,omitempty"` // string or []ContentBlock
	Display    bool   `json:"display,omitempty"`
	Details    any    `json:"details,omitempty"`

	// For EntryBranchSummary
	Summary string `json:"summary,omitempty"`
	FromID  string `json:"fromId,omitempty"`

	// For EntryCompaction
	CompactionSummary string `json:"compactionSummary,omitempty"`
	TokensBefore      int    `json:"tokensBefore,omitempty"`
	FirstKeptEntryID  string `json:"firstKeptEntryId,omitempty"`

	// For EntryModelChange
	Provider string `json:"provider,omitempty"`
	ModelID  string `json:"modelId,omitempty"`

	// For EntryThinkingChange
	ThinkingLevel string `json:"thinkingLevel,omitempty"`

	// For EntrySessionInfo
	SessionID   string `json:"sessionId,omitempty"`
	Description string `json:"description,omitempty"`
}

// SessionContext is the rebuilt context from session entries.
type SessionContext struct {
	Messages     []core.Message
	ThinkingLevel string
	Model        *SessionModel
}

// SessionModel represents the active model in a session.
type SessionModel struct {
	Provider string
	ModelID  string
}

// --- Session Storage interface ---

// SessionStorage defines the persistence interface for sessions.
type SessionStorage interface {
	// Append writes entries to storage.
	Append(entries []SessionTreeEntry) error
	// ReadAll reads all entries from storage.
	ReadAll() ([]SessionTreeEntry, error)
	// Close closes the storage.
	Close() error
}

// --- Compaction settings ---

// CompactionSettings configures context compaction behavior.
type CompactionSettings struct {
	// MaxTokensBeforeCompaction triggers compaction when usage exceeds this.
	MaxTokensBeforeCompaction int
	// TargetTokensAfterCompaction is the target usage after compaction.
	TargetTokensAfterCompaction int
	// MinMessagesToKeep is the minimum number of recent messages to preserve.
	MinMessagesToKeep int
	// SummaryPrompt is the prompt sent to the LLM for summarization.
	SummaryPrompt string
}

// DefaultCompactionSettings returns sensible defaults.
func DefaultCompactionSettings() CompactionSettings {
	return CompactionSettings{
		MaxTokensBeforeCompaction:  100000,
		TargetTokensAfterCompaction: 50000,
		MinMessagesToKeep:          10,
		SummaryPrompt: `Summarize the following conversation history concisely, preserving:
- Key decisions and conclusions
- Important facts and context
- File operations (reads/writes) and their results
- Tool call outcomes
- The user's goals and current progress

Conversation:
%s`,
	}
}
