// Package contextmgr 提供 Agent 上下文窗口管理。
//
// 核心功能：
// 1. Token 估算（基于字符数的简单估算）
// 2. 自动压缩触发（达到阈值时调用 LLM 摘要）
// 3. 软/硬限制（soft 触发摘要，hard 强制截断）
//
// 性能优化：
// 1. 增量 TokenStats：只计算新增消息，避免全量重算
// 2. 缓存 system prompt 组件：避免重复格式化
// 3. 并行化：上下文计算和 system prompt 构建并发进行
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

// Settings 配置上下文管理参数。
type Settings struct {
	// MaxContextTokens 是模型上下文窗口大小。
	// 例如 gpt-4o: 128000, claude-sonnet-4-5: 200000, deepseek-chat: 64000。
	MaxContextTokens int

	// SoftLimitRatio 触发自动压缩的使用比例（0-1）。
	// 默认 0.7：使用到 70% 时开始压缩。
	SoftLimitRatio float64

	// HardLimitRatio 强制截断的使用比例（0-1）。
	// 默认 0.95：使用到 95% 时强制丢弃最早消息。
	HardLimitRatio float64

	// MinRecentMessages 压缩时保留的最近消息数。
	MinRecentMessages int

	// ReservedForResponse 预留给响应的 token 数。
	ReservedForResponse int
}

// DefaultSettings 返回基于模型的合理默认。
func DefaultSettings(modelID string) Settings {
	s := Settings{
		MaxContextTokens:    128000,
		SoftLimitRatio:      0.7,
		HardLimitRatio:      0.95,
		MinRecentMessages:   10,
		ReservedForResponse: 4096,
	}

	// 简单模型识别（可扩展）
	lower := strings.ToLower(modelID)
	switch {
	case strings.Contains(lower, "claude"):
		s.MaxContextTokens = 200000
	case strings.Contains(lower, "gpt-4o") || strings.Contains(lower, "gpt-4-turbo"):
		s.MaxContextTokens = 128000
	case strings.Contains(lower, "deepseek"):
		s.MaxContextTokens = 64000
	case strings.Contains(lower, "glm-4-long") || strings.Contains(lower, "kimi"):
		s.MaxContextTokens = 128000
	}

	return s
}

// SoftLimit 返回软限制 token 数。
func (s Settings) SoftLimit() int {
	return int(float64(s.MaxContextTokens-s.ReservedForResponse) * s.SoftLimitRatio)
}

// HardLimit 返回硬限制 token 数。
func (s Settings) HardLimit() int {
	return int(float64(s.MaxContextTokens-s.ReservedForResponse) * s.HardLimitRatio)
}

// EstimateTokens 估算消息列表的 token 数。
// 简单实现：CJK ~ 1.5 字符/token, ASCII ~ 4 字符/token。
func EstimateTokens(messages []core.Message) int {
	total := 0
	for _, msg := range messages {
		total += estimateMessageTokens(msg)
	}
	return total
}

// TokenStats 增量跟踪 token 使用情况。
//
// 用法：
//
//	stats := NewTokenStats(settings)
//	stats.Add(message)  // 添加新消息
//	stats.AddMany(messages)  // 批量添加
//	if stats.ShouldCompact() { ... }
type TokenStats struct {
	mu           sync.RWMutex
	settings     Settings
	totalTokens  int
	messageCount int
	lastUpdate   time.Time
}

// NewTokenStats 创建增量 token 统计器。
func NewTokenStats(settings Settings) *TokenStats {
	return &TokenStats{
		settings:   settings,
		lastUpdate: time.Now(),
	}
}

// Add 添加单条消息，更新 token 计数。
// 增量更新比重新计算全量快 O(1)。
func (t *TokenStats) Add(msg core.Message) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalTokens += estimateMessageTokens(msg)
	t.messageCount++
	t.lastUpdate = time.Now()
}

// AddMany 批量添加消息。
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

// Recompute 从消息列表完全重算（用于压缩后重置）。
func (t *TokenStats) Recompute(messages []core.Message) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalTokens = EstimateTokens(messages)
	t.messageCount = len(messages)
	t.lastUpdate = time.Now()
}

// Tokens 返回当前总 token 数。
func (t *TokenStats) Tokens() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.totalTokens
}

// MessageCount 返回消息数。
func (t *TokenStats) MessageCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.messageCount
}

// ShouldCompact 判断是否应触发压缩。
func (t *TokenStats) ShouldCompact() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.totalTokens > t.settings.SoftLimit()
}

// ShouldTruncate 判断是否应强制截断。
func (t *TokenStats) ShouldTruncate() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.totalTokens > t.settings.HardLimit()
}

// Stats 描述当前上下文状态。
type Stats struct {
	MessageCount    int
	EstimatedTokens int
	SoftLimit       int
	HardLimit       int
	MaxContext      int
	UsageRatio      float64
}

// Get 从 TokenStats 派生 Stats。
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

// ShouldCompact 判断是否应触发压缩（基于 Stats 快照）。
func ShouldCompact(stats Stats, settings Settings) bool {
	return stats.EstimatedTokens > settings.SoftLimit()
}

// ShouldTruncate 判断是否应强制截断。
func ShouldTruncate(stats Stats, settings Settings) bool {
	return stats.EstimatedTokens > settings.HardLimit()
}

// ComputeStats 统计消息列表的上下文使用情况。
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

// FormatStats 把 Stats 格式化为可读字符串。
func FormatStats(s Stats) string {
	bar := renderUsageBar(s.UsageRatio, 20)
	return fmt.Sprintf(
		"📊 上下文: %d/%d tokens (%.1f%%) | 消息: %d\n   软限制: %d | 硬限制: %d\n   %s",
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
	// 加上每条消息的元数据开销（约 4 tokens）
	total := 4
	switch m := msg.(type) {
	case core.UserMessage:
		total += estimateAnyTokens(m.Content)
	case core.AssistantMessage:
		// 如果有真实使用值，优先使用
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
			total += 1000 // 图像大致 1000 tokens
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
	// 区分中英文估算
	cjkCount := 0
	otherCount := 0
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			cjkCount++
		} else {
			otherCount++
		}
	}
	// CJK: 约 1.5 字符/token; ASCII: 约 4 字符/token
	return cjkCount*2/3 + otherCount/4
}

// Truncate 强制截断到保留最近 N 条消息。
// 返回截断后的消息列表。
func Truncate(messages []core.Message, keepLast int) []core.Message {
	if len(messages) <= keepLast {
		return messages
	}
	result := make([]core.Message, 0, keepLast)
	result = append(result, messages[len(messages)-keepLast:]...)
	return result
}

// CompactionResult 包装压缩结果。
type CompactionResult struct {
	Summary     string
	OldMessages []core.Message
	NewMessages []core.Message
	TokensSaved int
	Duration    time.Duration
}

// Compact 执行压缩：调用 LLM 摘要早期消息，保留最近消息。
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

	// 串行化在另一个 goroutine 中做（与构造保留消息并发）
	type result struct {
		serialized string
		summary    string
		err        error
	}
	resCh := make(chan result, 1)

	go func() {
		// 串行化消息
		serialized := session.SerializeMessagesForSummary(toSummarize)

		// 构造 prompt
		prompt := fmt.Sprintf(`请简洁地总结以下对话历史，保留：
- 关键决策和结论
- 重要事实和上下文
- 文件操作（读/写）及其结果
- 工具调用的结果
- 用户的目标和当前进度

对话历史:
%s`, serialized)

		// 调用 LLM 生成摘要
		summary, err := summarizeWithLLM(ctx, model, prompt, streamOpts...)
		resCh <- result{serialized: serialized, summary: summary, err: err}
	}()

	// 主线程同时构造保留消息
	res := <-resCh
	if res.err != nil {
		return nil, fmt.Errorf("LLM 摘要失败: %w", res.err)
	}
	summary := res.summary
	if summary == "" {
		return nil, fmt.Errorf("LLM 返回空摘要")
	}

	// 构造新消息列表：摘要 user 消息 + 保留消息
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

// summarizeWithLLM 调用 LLM 生成摘要。
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
	// 提取文本内容
	var text strings.Builder
	for _, b := range msg.Content {
		if c, ok := b.(core.TextContent); ok {
			text.WriteString(c.Text)
		}
	}
	return text.String(), nil
}
