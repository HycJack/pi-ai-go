package contextmgr

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"pi-ai-go/agent/session"
	"pi-ai-go/core"
	"pi-ai-go/llm"
)

type Settings struct {
	MaxContextTokens    int
	SoftLimitRatio      float64
	HardLimitRatio      float64
	MinRecentMessages   int
	ReservedForResponse int
}

func DefaultSettings(modelID string) Settings {
	s := Settings{
		MaxContextTokens:    128000,
		SoftLimitRatio:      0.7,
		HardLimitRatio:      0.95,
		MinRecentMessages:   10,
		ReservedForResponse: 4096,
	}
	lower := strings.ToLower(modelID)
	switch {
	case strings.Contains(lower, "claude"):
		s.MaxContextTokens = 200000
	case strings.Contains(lower, "gpt-4o") || strings.Contains(lower, "gpt-4-turbo"):
		s.MaxContextTokens = 128000
	case strings.Contains(lower, "deepseek"):
		s.MaxContextTokens = 64000
	}
	return s
}

func (s Settings) SoftLimit() int {
	return int(float64(s.MaxContextTokens-s.ReservedForResponse) * s.SoftLimitRatio)
}

func (s Settings) HardLimit() int {
	return int(float64(s.MaxContextTokens-s.ReservedForResponse) * s.HardLimitRatio)
}

func EstimateTokens(messages []core.Message) int {
	total := 0
	for _, msg := range messages {
		total += estimateMessageTokens(msg)
	}
	return total
}

type TokenStats struct {
	mu           sync.RWMutex
	settings     Settings
	totalTokens  int
	messageCount int
	lastUpdate   time.Time
}

func NewTokenStats(settings Settings) *TokenStats {
	return &TokenStats{
		settings:   settings,
		lastUpdate: time.Now(),
	}
}

func (t *TokenStats) Add(msg core.Message) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalTokens += estimateMessageTokens(msg)
	t.messageCount++
	t.lastUpdate = time.Now()
}

func (t *TokenStats) AddMany(msgs []core.Message) {
	if len(msgs) == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, msg := range msgs {
		t.totalTokens += estimateMessageTokens(msg)
		t.messageCount++
	}
	t.lastUpdate = time.Now()
}

func (t *TokenStats) Recompute(messages []core.Message) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalTokens = EstimateTokens(messages)
	t.messageCount = len(messages)
	t.lastUpdate = time.Now()
}

func (t *TokenStats) Tokens() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.totalTokens
}

func (t *TokenStats) ShouldCompact() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.totalTokens > t.settings.SoftLimit()
}

func (t *TokenStats) ShouldTruncate() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.totalTokens > t.settings.HardLimit()
}

type Stats struct {
	MessageCount    int
	EstimatedTokens int
	SoftLimit       int
	HardLimit       int
	MaxContext      int
	UsageRatio      float64
}

func (t *TokenStats) Get() Stats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	ratio := 0.0
	if t.settings.MaxContextTokens > 0 {
		ratio = float64(t.totalTokens) / float64(t.settings.MaxContextTokens)
	}
	return Stats{
		MessageCount:    t.messageCount,
		EstimatedTokens: t.totalTokens,
		SoftLimit:       t.settings.SoftLimit(),
		HardLimit:       t.settings.HardLimit(),
		MaxContext:      t.settings.MaxContextTokens,
		UsageRatio:      ratio,
	}
}

func ComputeStats(messages []core.Message, settings Settings) Stats {
	tokens := EstimateTokens(messages)
	ratio := 0.0
	if settings.MaxContextTokens > 0 {
		ratio = float64(tokens) / float64(settings.MaxContextTokens)
	}
	return Stats{
		MessageCount:    len(messages),
		EstimatedTokens: tokens,
		SoftLimit:       settings.SoftLimit(),
		HardLimit:       settings.HardLimit(),
		MaxContext:      settings.MaxContextTokens,
		UsageRatio:      ratio,
	}
}

func FormatStats(s Stats) string {
	bar := renderUsageBar(s.UsageRatio, 20)
	return fmt.Sprintf(
		"上下文: %d/%d tokens (%.1f%%) | 消息: %d\n   软限制: %d | 硬限制: %d\n   %s",
		s.EstimatedTokens, s.MaxContext, s.UsageRatio*100,
		s.MessageCount, s.SoftLimit, s.HardLimit, bar,
	)
}

func renderUsageBar(ratio float64, width int) string {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(float64(width) * ratio)
	empty := width - filled
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", empty) + "]"
}

func estimateMessageTokens(msg core.Message) int {
	total := 4
	switch m := msg.(type) {
	case core.UserMessage:
		total += estimateAnyTokens(m.Content)
	case core.AssistantMessage:
		if m.Usage.Input > 0 {
			total = m.Usage.Input
		} else {
			total += estimateContentBlockTokens(m.Content)
		}
	case core.ToolResultMessage:
		total += estimateContentBlockTokens(m.Content)
	}
	return total
}

func estimateAnyTokens(v any) int {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case string:
		return estimateStringTokens(x)
	case []core.ContentBlock:
		return estimateContentBlockTokens(x)
	default:
		return estimateStringTokens(fmt.Sprintf("%v", v))
	}
}

func estimateContentBlockTokens(blocks []core.ContentBlock) int {
	total := 0
	for _, b := range blocks {
		switch c := b.(type) {
		case core.TextContent:
			total += estimateStringTokens(c.Text)
		case core.ThinkingContent:
			total += estimateStringTokens(c.Thinking)
		case core.ImageContent:
			total += 1000
		case core.ToolCall:
			total += estimateStringTokens(string(c.Arguments))
			total += 20
		}
	}
	return total
}

func estimateStringTokens(s string) int {
	if s == "" {
		return 0
	}
	cjkCount := 0
	otherCount := 0
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			cjkCount++
		} else {
			otherCount++
		}
	}
	return cjkCount*2/3 + otherCount/4
}

func Truncate(messages []core.Message, keepLast int) []core.Message {
	if len(messages) <= keepLast {
		return messages
	}
	// Preserve the first system message if present — it is typically a
	// globally important prompt that must not be dropped during truncation.
	var systemMsg core.Message
	hasSystem := false
	if len(messages) > 0 {
		if _, ok := messages[0].(core.SystemMessage); ok {
			systemMsg = messages[0]
			hasSystem = true
		} else if userMsg, ok := messages[0].(core.UserMessage); ok && userMsg.Role == "system" {
			systemMsg = messages[0]
			hasSystem = true
		}
	}

	result := make([]core.Message, 0, keepLast+1)
	if hasSystem {
		result = append(result, systemMsg)
	}
	result = append(result, messages[len(messages)-keepLast:]...)
	return result
}

type CompactionResult struct {
	Summary     string
	OldMessages []core.Message
	NewMessages []core.Message
	TokensSaved int
	Duration    time.Duration
}

func Compact(
	ctx context.Context,
	model core.Model,
	messages []core.Message,
	settings Settings,
	streamOpts ...core.SimpleStreamOptions,
) (*CompactionResult, error) {
	if len(messages) <= settings.MinRecentMessages {
		return nil, fmt.Errorf("消息数 %d <= 最小保留 %d，跳过压缩", len(messages), settings.MinRecentMessages)
	}

	start := time.Now()
	splitIdx := len(messages) - settings.MinRecentMessages
	if splitIdx <= 0 {
		splitIdx = 1
	}

	toSummarize := messages[:splitIdx]
	toKeep := messages[splitIdx:]

	maxSummarizeTokens := settings.MaxContextTokens / 2
	if maxSummarizeTokens <= 0 {
		maxSummarizeTokens = 60000
	}
	if maxSummarizeTokens > 100000 {
		maxSummarizeTokens = 100000
	}
	toSummarize = truncateMessagesByTokens(toSummarize, maxSummarizeTokens)

	type result struct {
		summary string
		err     error
	}
	resCh := make(chan result, 1)

	go func() {
		serialized := session.SerializeMessagesForSummary(toSummarize)
		prompt := fmt.Sprintf(`请简洁地总结以下对话历史，保留：
- 关键决策和结论
- 重要事实和上下文
- 文件操作（读/写）及其结果
- 工具调用的结果
- 用户的目标和当前进度

对话历史:
%s`, serialized)

		summary, err := summarizeWithLLM(ctx, model, prompt, streamOpts...)
		resCh <- result{summary: summary, err: err}
	}()

	res := <-resCh
	if res.err != nil {
		return nil, fmt.Errorf("LLM 摘要失败: %w", res.err)
	}
	summary := res.summary
	if summary == "" {
		return nil, fmt.Errorf("LLM 返回空摘要")
	}

	summaryUserMsg := core.UserMessage{
		Role:    "user",
		Content: fmt.Sprintf("[对话历史摘要]\n\n%s", summary),
	}
	assistantAck := core.AssistantMessage{
		Role: "assistant",
		Content: []core.ContentBlock{
			core.TextContent{
				Type: "text",
				Text: "好的，我已了解之前的对话历史。请继续。",
			},
		},
	}

	newMessages := make([]core.Message, 0, 2+len(toKeep))
	newMessages = append(newMessages, summaryUserMsg, assistantAck)
	newMessages = append(newMessages, toKeep...)

	tokensBefore := EstimateTokens(messages)
	tokensAfter := EstimateTokens(newMessages)

	return &CompactionResult{
		Summary:     summary,
		OldMessages: messages,
		NewMessages: newMessages,
		TokensSaved: tokensBefore - tokensAfter,
		Duration:    time.Since(start),
	}, nil
}

func truncateMessagesByTokens(messages []core.Message, maxTokens int) []core.Message {
	if maxTokens <= 0 || len(messages) == 0 {
		return messages
	}
	total := 0
	start := len(messages)
	for i := len(messages) - 1; i >= 0; i-- {
		t := estimateMessageTokens(messages[i])
		if total+t > maxTokens {
			break
		}
		total += t
		start = i
	}
	if start == len(messages) {
		return messages[len(messages)-1:]
	}
	if start == 0 {
		return messages
	}
	return messages[start:]
}

func summarizeWithLLM(ctx context.Context, model core.Model, prompt string, streamOpts ...core.SimpleStreamOptions) (string, error) {
	opts := streamOpts
	if len(opts) == 0 {
		opts = []core.SimpleStreamOptions{{}}
	}
	msg, err := llm.CompleteSimple(ctx, model, []core.Message{
		core.UserMessage{Content: prompt},
	}, opts...)
	if err != nil {
		return "", err
	}
	var text strings.Builder
	for _, b := range msg.Content {
		if c, ok := b.(core.TextContent); ok {
			text.WriteString(c.Text)
		}
	}
	return text.String(), nil
}
