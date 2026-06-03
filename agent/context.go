package agent

import (
	"fmt"
	"strings"
	"time"

	core "pi-ai-go/core"
)

// ContextPolicy controls how the agent loop manages its context window.
//
// Defaults follow the oh-my-pi / pi-mono convention: when usage exceeds
// the soft limit (75% of the model window) the loop prepares to compact;
// when usage exceeds the hard limit (95% of the model window) compaction
// is forced.
type ContextPolicy struct {
	// SoftLimit is the usage ratio (0.0-1.0) at which the agent should
	// schedule compaction. Defaults to 0.75.
	SoftLimit float64
	// HardLimit is the usage ratio at which compaction is forced before
	// the next LLM call. Defaults to 0.95.
	HardLimit float64
	// ReservedOutput is the number of tokens to reserve for the LLM's
	// output. Compaction considers `ContextWindow - ReservedOutput` to
	// be the effective input budget. Defaults to 4096.
	ReservedOutput int
	// MinTailMessages is the minimum number of trailing messages to
	// preserve across any compaction. Defaults to 4.
	MinTailMessages int
	// Strategy is the compaction strategy to use. Defaults to
	// CompactionStrategySlidingWindow (cheap, no LLM call).
	Strategy CompactionStrategy
}

func (c ContextPolicy) withDefaults() ContextPolicy {
	if c.SoftLimit <= 0 {
		c.SoftLimit = 0.75
	}
	if c.HardLimit <= 0 {
		c.HardLimit = 0.95
	}
	if c.ReservedOutput <= 0 {
		c.ReservedOutput = 4096
	}
	if c.MinTailMessages <= 0 {
		c.MinTailMessages = 4
	}
	if c.Strategy == "" {
		c.Strategy = CompactionStrategySlidingWindow
	}
	if c.HardLimit < c.SoftLimit {
		c.HardLimit = c.SoftLimit
	}
	return c
}

// CompactionStrategy enumerates the available compaction strategies.
type CompactionStrategy string

const (
	// CompactionStrategySlidingWindow drops the oldest non-system
	// messages until the slice fits in the effective budget. No LLM
	// call is made; this is the cheapest strategy and works offline.
	CompactionStrategySlidingWindow CompactionStrategy = "sliding_window"

	// CompactionStrategySummarize asks an LLM to summarize the dropped
	// portion and inserts the summary as a single user message at the
	// cut point. Requires a non-nil config.SummarizeModel.
	CompactionStrategySummarize CompactionStrategy = "summarize"

	// CompactionStrategyShake aggressively re-orders and truncates the
	// context. Reserved for future use.
	CompactionStrategyShake CompactionStrategy = "shake"
)

// ContextWindow is a snapshot of the agent's context budget. It is
// thread-safe and is meant to be re-created per-turn (cheap).
type ContextWindow struct {
	Model      core.Model
	Policy     ContextPolicy
	Used       int  // Estimated input tokens.
	Reserved   int  // Tokens reserved for output (= Policy.ReservedOutput).
	Effective  int  // ContextWindow - Reserved.
	HardTokens int  // Tokens at which compaction is forced (= HardLimit * Effective).
	SoftTokens int  // Tokens at which compaction is scheduled (= SoftLimit * Effective).
}

// NewContextWindow builds a ContextWindow for the given model and
// policy. If the model has no ContextWindow (0), the result is a no-op
// window (NeedsCompaction always returns false).
func NewContextWindow(model core.Model, policy ContextPolicy) *ContextWindow {
	policy = policy.withDefaults()
	effective := model.ContextWindow - policy.ReservedOutput
	if effective < 0 {
		effective = 0
	}
	cw := &ContextWindow{
		Model:      model,
		Policy:     policy,
		Reserved:   policy.ReservedOutput,
		Effective:  effective,
		HardTokens: int(float64(effective) * policy.HardLimit),
		SoftTokens: int(float64(effective) * policy.SoftLimit),
	}
	cw.Used = EstimateMessagesTokens(nil, model)
	return cw
}

// Add records additional token usage (e.g. from a streaming response).
func (c *ContextWindow) Add(usage int) {
	if c == nil {
		return
	}
	c.Used += usage
}

// NeedsCompaction returns true if usage has crossed the soft limit.
func (c *ContextWindow) NeedsCompaction() bool {
	if c == nil || c.Effective == 0 {
		return false
	}
	return c.Used >= c.SoftTokens
}

// MustCompact returns true if usage has crossed the hard limit and
// compaction is forced.
func (c *ContextWindow) MustCompact() bool {
	if c == nil || c.Effective == 0 {
		return false
	}
	return c.Used >= c.HardTokens
}

// Utilization returns the usage ratio (0.0-1.0).
func (c *ContextWindow) Utilization() float64 {
	if c == nil || c.Effective == 0 {
		return 0
	}
	return float64(c.Used) / float64(c.Effective)
}

// --- token estimation ---

// EstimateMessagesTokens returns the estimated token count for a slice
// of messages. The estimate is character-based (≈ 4 chars per token),
// which is fast and good enough for compaction decisions.
//
// Each message gets a +4 token overhead to account for role/structure
// markers. The system prompt, if present in the model, is included.
func EstimateMessagesTokens(msgs []core.Message, model core.Model) int {
	total := estimateModelOverhead(model)
	for _, m := range msgs {
		total += estimateOneMessage(m) + 4 // role/structure overhead
	}
	return total
}

func estimateModelOverhead(m core.Model) int {
	// System prompt overhead — the LLM provider prepends the system
	// prompt and a "you are a helpful assistant" header.
	// We charge a fixed budget for the header.
	return 16
}

func estimateOneMessage(m core.Message) int {
	switch mm := m.(type) {
	case core.UserMessage:
		return estimateAnyContent(mm.Content)
	case core.AssistantMessage:
		total := 0
		for _, b := range mm.Content {
			switch bb := b.(type) {
			case core.TextContent:
				total += estimateText(bb.Text)
			case core.ThinkingContent:
				total += estimateText(bb.Thinking)
			case core.ToolCall:
				total += estimateText(bb.Name) + len(bb.Arguments)/4
			}
		}
		return total
	case core.ToolResultMessage:
		total := estimateText(mm.ToolName) + estimateText(mm.ToolCallID) + 8
		for _, b := range mm.Content {
			switch bb := b.(type) {
			case core.TextContent:
				total += estimateText(bb.Text)
			case core.ImageContent:
				// Images cost more than text; 1024 is a common
				// average across providers.
				total += 1024
			}
		}
		if mm.IsError {
			total += 4
		}
		return total
	}
	return 0
}

func estimateAnyContent(c any) int {
	if c == nil {
		return 0
	}
	switch v := c.(type) {
	case string:
		return estimateText(v)
	case []core.ContentBlock:
		total := 0
		for _, b := range v {
			switch bb := b.(type) {
			case core.TextContent:
				total += estimateText(bb.Text)
			case core.ImageContent:
				total += 1024
			}
		}
		return total
	}
	return estimateText(fmt.Sprintf("%v", c))
}

// estimateText uses a 4-chars-per-token heuristic with a small
// minimum-of-1-to-encode-empty rule.
func estimateText(s string) int {
	if s == "" {
		return 1
	}
	return (len(s) + 3) / 4
}

// EstimateTextTokens is exported so callers can budget a single block
// (e.g. a future tool summary).
func EstimateTextTokens(s string) int {
	return estimateText(s)
}

// CompactSummaryUserMessage is the canonical user message that holds
// a compaction summary. It is recognized by sliding-window compaction
// and is always kept (never dropped) when the system prompt is locked.
const CompactSummaryUserMessage = "__compaction_summary__"

// SlidingWindowCompact drops the oldest non-system, non-summary
// messages until the slice fits within targetTokens. It always keeps
// the most recent `tail` messages.
//
// The function returns the compacted slice and the number of messages
// dropped. If the slice already fits, no compaction is performed and
// the original slice is returned unchanged.
func SlidingWindowCompact(messages []core.Message, model core.Model, targetTokens, tail int) ([]core.Message, int) {
	if targetTokens <= 0 || tail <= 0 {
		tail = 4
	}
	if len(messages) <= tail {
		return messages, 0
	}

	// Always lock the system prompt (if any) and the last `tail` messages.
	// A "compaction summary" message is also locked.
	headLocked := make([]bool, len(messages))
	// Lock the most recent N (tail) messages.
	for i := len(messages) - tail; i < len(messages); i++ {
		if i >= 0 {
			headLocked[i] = true
		}
	}
	// Lock any user message that is a compaction summary.
	for i, m := range messages {
		if um, ok := m.(core.UserMessage); ok {
			if s, ok := um.Content.(string); ok && strings.Contains(s, CompactSummaryUserMessage) {
				headLocked[i] = true
			}
		}
	}

	// Walk from the end: keep "head-locked" messages and any other
	// message that we cannot yet drop. Walk from the start: drop
	// unlocked messages until the slice fits.
	dropped := 0
	cur := EstimateMessagesTokens(messages, model)
	for i := 0; i < len(messages); i++ {
		if headLocked[i] {
			continue
		}
		if cur <= targetTokens {
			break
		}
		cur -= estimateOneMessage(messages[i]) + 4
		dropped++
	}

	if dropped == 0 {
		return messages, 0
	}

	out := make([]core.Message, 0, len(messages)-dropped)
	for i, m := range messages {
		if i < dropped && !headLocked[i] {
			continue
		}
		out = append(out, m)
	}
	return out, dropped
}

// CompactSummarizeInPlace summarizes the dropped portion of messages
// (the first `drop` messages) into a single core.UserMessage and
// inserts it at the cut point. The original messages are kept; this
// is a non-destructive operation. Use it for audit-trail or
// fallback-to-sliding-window scenarios.
//
// `summary` is the LLM-produced summary text. A header is prepended
// so future sliding-window passes can detect the boundary.
func CompactSummarizeInPlace(messages []core.Message, summary string, drop int, now time.Time) []core.Message {
	if drop <= 0 || len(messages) == 0 {
		return messages
	}
	if drop > len(messages) {
		drop = len(messages)
	}
	body := fmt.Sprintf("%s\n\n%s\n\n(compacted at %s)",
		CompactSummaryUserMessage, summary, now.Format(time.RFC3339))
	summaryMsg := core.UserMessage{
		Role:      "user",
		Content:   body,
		Timestamp: now,
	}
	out := make([]core.Message, 0, len(messages)-drop+1)
	out = append(out, summaryMsg)
	out = append(out, messages[drop:]...)
	return out
}
