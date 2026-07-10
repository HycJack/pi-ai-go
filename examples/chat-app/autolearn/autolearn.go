package autolearn

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"chat-app/memory"
	"pi-ai-go/core"
)

type TriggerSource string

const (
	SourceUserInput  TriggerSource = "user"
	SourceToolResult TriggerSource = "tool"
	SourceLLMExtract TriggerSource = "extract"
)

type Settings struct {
	AutoLearn     bool
	ExtractEveryN int
	MinConfidence float64
}

func DefaultSettings() Settings {
	return Settings{
		AutoLearn:     false,
		ExtractEveryN: 3,
		MinConfidence: 0.5,
	}
}

type Trigger struct {
	Source  TriggerSource
	Key     string
	Value   string
	Context string
	Time    time.Time
}

type Extractor interface {
	Extract(ctx context.Context, messages []core.Message) ([]Trigger, error)
}

type AutoLearner struct {
	settings    Settings
	mem         *memory.Memory
	mu          sync.Mutex
	counter     int
	WorkflowDir string
	// LLMExtract 直接调 LLM 从文本中提取记忆
	LLMExtract func(ctx context.Context, text string) (map[string]string, error)
}

func New(mem *memory.Memory, settings Settings) *AutoLearner {
	return &AutoLearner{
		settings: settings,
		mem:      mem,
	}
}

// ProcessUserInput 从用户输入中提取记忆，使用 LLM 进行智能提取。
func (a *AutoLearner) ProcessUserInput(text string) int {
	if a.mem == nil {
		return 0
	}
	if a.LLMExtract == nil {
		return 0
	}
	// 使用 LLM 提取
	triggers, err := a.LLMExtract(context.Background(), text)
	if err != nil || len(triggers) == 0 {
		return 0
	}
	var ts []Trigger
	now := time.Now()
	for k, v := range triggers {
		ts = append(ts, Trigger{Source: SourceUserInput, Key: k, Value: v, Time: now})
	}
	return a.apply(ts)
}

// ProcessToolResult 已弃用，统一走 LLM 提取路径，保留空方法避免编译错误。
func (a *AutoLearner) ProcessToolResult(text string) int {
	return 0
}

// ExtractFromToolResult 已弃用。
func ExtractFromToolResult(text string) []Trigger {
	return nil
}

// MaybeExtract 每 N 轮对话后批量提取记忆。
func (a *AutoLearner) MaybeExtract(ctx context.Context, messages []core.Message, extractor Extractor) int {
	if !a.settings.AutoLearn || extractor == nil || a.mem == nil {
		return 0
	}
	a.mu.Lock()
	a.counter++
	shouldExtract := a.counter%a.settings.ExtractEveryN == 0
	a.mu.Unlock()
	if !shouldExtract {
		return 0
	}
	triggers, err := extractor.Extract(ctx, messages)
	if err != nil {
		return 0
	}
	return a.apply(triggers)
}

// ─── LLM 提取器实现 ───

type LLMSimpleExtractor struct {
	SummarizeFunc func(ctx context.Context, prompt string) (string, error)
}

func (e *LLMSimpleExtractor) Extract(ctx context.Context, messages []core.Message) ([]Trigger, error) {
	if e.SummarizeFunc == nil {
		return nil, fmt.Errorf("SummarizeFunc not set")
	}
	var sb strings.Builder
	sb.WriteString("你是记忆提取助手。请从下面的对话中找出需要**长期记住**的事实。\n")
	sb.WriteString("【输出格式】每行一条 `KEY=VALUE`。\n")
	sb.WriteString("没有任何值得记住的内容 → 单独输出 `NONE`。\n\n")
	sb.WriteString("允许的 KEY 前缀：user., assistant., task., project., fact., decision., constraint., health., tool., goal.\n\n")

	for _, msg := range messages {
		switch m := msg.(type) {
		case core.UserMessage:
			fmt.Fprintf(&sb, "用户: %v\n", m.Content)
		case core.AssistantMessage:
			var text string
			for _, b := range m.Content {
				if c, ok := b.(core.TextContent); ok {
					text += c.Text
				}
			}
			fmt.Fprintf(&sb, "助手: %s\n", text)
		}
	}

	response, err := e.SummarizeFunc(ctx, sb.String())
	if err != nil {
		return nil, err
	}
	return parseExtractionResult(response, SourceLLMExtract), nil
}

var allowedKeyPrefixes = []string{
	"user.", "assistant.", "task.", "project.",
	"fact.", "decision.", "constraint.",
	"health.", "tool.", "goal.",
}

func allowedKeyPrefix(key string) bool {
	for _, p := range allowedKeyPrefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

// parseExtractionResult 解析 LLM 返回的 KEY=VALUE 格式。
func parseExtractionResult(response string, source TriggerSource) []Trigger {
	var triggers []Trigger
	now := time.Now()
	seen := make(map[string]bool)

	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "NONE" || strings.HasPrefix(line, "#") {
			continue
		}
		// 按 = 或 : 分割（只分割第一个分隔符）
		key, value, found := splitKV(line)
		if !found || key == "" || value == "" || seen[key] {
			continue
		}
		if source == SourceLLMExtract && !allowedKeyPrefix(key) {
			continue
		}
		if len(value) > 200 {
			value = value[:200]
		}
		seen[key] = true
		triggers = append(triggers, Trigger{
			Source: source,
			Key:    key,
			Value:  value,
			Time:   now,
		})
	}
	return triggers
}

// splitKV 将 line 按第一个 = 或 : 或 ： 分割为 key/value。
func splitKV(line string) (key, value string, found bool) {
	for _, sep := range []string{"=", ":", "："} {
		idx := strings.Index(line, sep)
		if idx > 0 {
			k := strings.TrimSpace(line[:idx])
			v := strings.Trim(strings.TrimSpace(line[idx+len(sep):]), "\"'「」『』")
			if k != "" && v != "" {
				return k, v, true
			}
		}
	}
	return "", "", false
}

func (a *AutoLearner) apply(triggers []Trigger) int {
	count := 0
	for _, t := range triggers {
		if t.Key == "" || t.Value == "" {
			continue
		}
		a.mem.SetWithCategory(t.Key, t.Value, string(t.Source))
		count++
	}
	if count > 0 {
		_ = a.mem.Save()
	}
	return count
}
