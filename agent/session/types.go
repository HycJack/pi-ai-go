// Package session provides a production-grade agent framework with session
// persistence, context compaction, skills, and prompt templates.
package session

import (
	"fmt"
	"time"

	core "pi-ai-go/core"
)

// --- Error types ---

// ErrorCode is a stable, backend-independent error code.
type ErrorCode string

const (
	ErrAborted        ErrorCode = "aborted"
	ErrNotFound       ErrorCode = "not_found"
	ErrPermission     ErrorCode = "permission_denied"
	ErrInvalid        ErrorCode = "invalid"
	ErrTimeout        ErrorCode = "timeout"
	ErrStorage        ErrorCode = "storage"
	ErrSummarization  ErrorCode = "summarization_failed"
	ErrInvalidSession ErrorCode = "invalid_session"
	ErrInvalidEntry   ErrorCode = "invalid_entry"
	ErrUnknown        ErrorCode = "unknown"
	ErrBusy           ErrorCode = "busy"
	ErrInvalidRef     ErrorCode = "invalid_ref"
	ErrInvalidRebase  ErrorCode = "invalid_rebase"
)

// SessionError is the base error type for session operations.
type SessionError struct {
	Code    ErrorCode
	Message string
	Path    string
	Err     error
}

func (e *SessionError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Path, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *SessionError) Unwrap() error { return e.Err }

// --- Skill ---

// Skill represents a skill loaded from a SKILL.md file or provided by an application.
type Skill struct {
	Name                   string // Stable skill name
	Description            string // Short model-visible description
	Content                string // Full skill instructions
	FilePath               string // Absolute path to the skill file
	DisableModelInvocation bool   // Exclude from model-visible skill lists
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
	EntrySessionRef      EntryType = "session/ref"
	EntrySessionCheckout EntryType = "session/checkout"
	EntryEvent           EntryType = "event"
)

// SessionTreeEntry is a single entry in the session tree.
type SessionTreeEntry struct {
	ID        string    `json:"id"`
	Type      EntryType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	ParentID  EntryID   `json:"parentId,omitempty"`

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

	// For EntrySessionRef
	RefName     RefName `json:"refName,omitempty"`
	RefTargetID EntryID `json:"refTargetId,omitempty"`

	// For EntrySessionCheckout
	CheckoutTarget struct {
		Type string  `json:"type"`
		Name RefName `json:"name,omitempty"`
		ID   EntryID `json:"id,omitempty"`
	} `json:"checkoutTarget,omitempty"`

	// For EntryEvent
	EventData any `json:"eventData,omitempty"`
}

// SessionContext is the rebuilt context from session entries.
type SessionContext struct {
	Messages      []core.Message
	ThinkingLevel string
	Model         *SessionModel
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
		MaxTokensBeforeCompaction:   100000,
		TargetTokensAfterCompaction: 50000,
		MinMessagesToKeep:           10,
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

// --- Branch types ---

// EntryID is a unique identifier for a session entry.
type EntryID = string

// RefName is a name for a branch reference.
type RefName = string

// Head represents the current HEAD of the session.
type Head struct {
	Type     string  // "ref" or "detached"
	Name     RefName // for "ref" type
	TargetID EntryID // for "detached" type, optional
}

// SessionCheckoutEntryData is the data for a session/checkout entry.
type SessionCheckoutEntryData struct {
	Target struct {
		Type string  `json:"type"`
		Name RefName `json:"name,omitempty"`
		ID   EntryID `json:"id,omitempty"`
	} `json:"target"`
}

// SessionRefEntryData is the data for a session/ref entry.
type SessionRefEntryData struct {
	Name     RefName `json:"name"`
	TargetID EntryID `json:"targetId,omitempty"`
}

// SessionSnapshot represents the result of replaying the session log.
type SessionSnapshot struct {
	Entries      []SessionTreeEntry
	EntryByID    map[EntryID]SessionTreeEntry
	Head         Head
	HeadTargetID EntryID
	Refs         map[RefName]EntryID
}

// RebaseResult holds the result of a rebase operation.
type RebaseResult struct {
	Entries []struct {
		NewID EntryID `json:"newId"`
		OldID EntryID `json:"oldId"`
	}
	Name      RefName `json:"name"`
	NewBaseID EntryID `json:"newBaseId,omitempty"`
	NewHeadID EntryID `json:"newHeadId,omitempty"`
	OldBaseID EntryID `json:"oldBaseId,omitempty"`
	OldHeadID EntryID `json:"oldHeadId,omitempty"`
}

// SessionCloneOptions provides options for cloning a session.
type SessionCloneOptions struct {
	Checkout       EntryID `json:"checkout,omitempty"`
	From           EntryID `json:"from,omitempty"`
	Refs           string  `json:"refs,omitempty"` // "active", "all", or comma-separated list
	SessionStorage SessionStorage
}

// SessionForkOptions provides options for forking a session.
type SessionForkOptions struct {
	Checkout bool    `json:"checkout,omitempty"`
	From     EntryID `json:"from,omitempty"`
}

// BranchChangeHandler is a callback for branch change events.
type BranchChangeHandler func(payload BranchChangePayload)

// BranchChangePayload is the payload for branch change events.
type BranchChangePayload struct {
	Type     string  `json:"type"` // "checkout", "fork", "rebase"
	Ref      RefName `json:"ref,omitempty"`
	TargetID EntryID `json:"targetId,omitempty"`
}
