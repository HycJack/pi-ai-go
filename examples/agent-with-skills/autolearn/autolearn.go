// Package autolearn 提供自动记忆触发机制。
//
// 触发来源：
// 1. 显式标记：用户输入中包含 "[remember:key=value]" 或 "请记住：key=value"
// 2. 工具结果标记：bash 输出包含 "REMEMBER:key=value"
// 3. LLM 提取：每 N 轮异步调用 LLM 提取可记忆的事实
//
// 设计原则：
// - 异步执行：不影响主对话流程
// - 增量去重：同一 key 多次出现只更新一次
// - 可禁用：通过 settings.AutoLearn = false 关闭
package autolearn

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"pi-ai-go/core"
	"examples/agent-with-skills/memory"
)

// TriggerSource 标记触发来源。
type TriggerSource string

const (
	SourceUserInput  TriggerSource = "user"    // 用户输入
	SourceToolResult TriggerSource = "tool"    // 工具结果
	SourceLLMExtract TriggerSource = "extract" // LLM 提取
)

// Settings 配置自动记忆。
type Settings struct {
	// AutoLearn 启用自动学习（LLM 提取）
	AutoLearn bool

	// ExtractEveryN 多少轮对话后触发一次 LLM 提取
	ExtractEveryN int

	// MinConfidence 提取置信度阈值（0-1）
	MinConfidence float64
}

// DefaultSettings 返回默认设置。
func DefaultSettings() Settings {
	return Settings{
		AutoLearn:      false, // 默认关闭 LLM 提取（避免开销）
		ExtractEveryN:  5,     // 每 5 轮提取一次
		MinConfidence:  0.7,
	}
}

// Trigger 是一次自动记忆触发事件。
type Trigger struct {
	Source  TriggerSource
	Key     string
	Value   string
	Context string // 触发的上下文（用于 LLM 提取时的来源）
	Time    time.Time
}

// Extractor 提取器接口。
// 抽象为接口方便测试和自定义。
type Extractor interface {
	// Extract 从对话历史中提取可记忆的事实
	// 返回 []Trigger 列表，调用者负责写入 memory
	Extract(ctx context.Context, messages []core.Message) ([]Trigger, error)
}

// AutoLearner 协调各种触发源。
type AutoLearner struct {
	settings Settings
	mem      *memory.Memory
	mu       sync.Mutex
	counter  int
}

// New 创建自动学习器。
func New(mem *memory.Memory, settings Settings) *AutoLearner {
	return &AutoLearner{
		settings: settings,
		mem:      mem,
	}
}

// --- 1. 显式标记触发 ---

var (
	// 匹配 "请记住：xxx=yyy" 或 "记住 xxx=yyy" 或 "[remember:xxx=yyy]"
	rememberRegex = regexp.MustCompile(`(?:请记住[：:]?|记住[：:]\s*|\[remember:\s*)([^\s=]+)\s*=\s*([^\n\]]+?)(?:\]|$|\n)`)
	// 匹配 "[memorize:key=value]"
	memorizeRegex = regexp.MustCompile(`\[memorize:\s*([^\s=]+)\s*=\s*([^\]]+?)\]`)
)

// ExtractFromUserInput 从用户输入中提取记忆标记。
//
// 匹配模式：
//   "请记住：user.name=小明"
//   "记住：user.name=小明"
//   "[remember:user.name=小明]"
//   "[memorize:user.name=小明]"
func ExtractFromUserInput(text string) []Trigger {
	triggers := []Trigger{}
	now := time.Now()

	for _, m := range rememberRegex.FindAllStringSubmatch(text, -1) {
		if len(m) >= 3 {
			triggers = append(triggers, Trigger{
				Source: SourceUserInput,
				Key:    strings.TrimSpace(m[1]),
				Value:  strings.TrimSpace(m[2]),
				Time:   now,
			})
		}
	}
	for _, m := range memorizeRegex.FindAllStringSubmatch(text, -1) {
		if len(m) >= 3 {
			triggers = append(triggers, Trigger{
				Source: SourceUserInput,
				Key:    strings.TrimSpace(m[1]),
				Value:  strings.TrimSpace(m[2]),
				Time:   now,
			})
		}
	}

	return triggers
}

// ExtractFromToolResult 从工具结果中提取记忆标记。
//
// 匹配模式（出现在工具输出里）：
//   "REMEMBER:user.name=小明"
//   "SAVE_MEMORY:preference.language=zh-CN"
func ExtractFromToolResult(text string) []Trigger {
	triggers := []Trigger{}
	now := time.Now()

	// 复用 rememberRegex
	for _, m := range rememberRegex.FindAllStringSubmatch(text, -1) {
		if len(m) >= 3 {
			triggers = append(triggers, Trigger{
				Source: SourceToolResult,
				Key:    strings.TrimSpace(m[1]),
				Value:  strings.TrimSpace(m[2]),
				Time:   now,
			})
		}
	}

	// REMEMBER: 前缀
	remRegex := regexp.MustCompile(`REMEMBER:\s*([^\s=]+)\s*=\s*([^\n]+)`)
	for _, m := range remRegex.FindAllStringSubmatch(text, -1) {
		if len(m) >= 3 {
			triggers = append(triggers, Trigger{
				Source: SourceToolResult,
				Key:    strings.TrimSpace(m[1]),
				Value:  strings.TrimSpace(m[2]),
				Time:   now,
			})
		}
	}

	return triggers
}

// --- 2. LLM 提取触发（异步） ---

// LLMSimpleExtractor 简单 LLM 提取器实现。
// 调用 LLM 让其分析对话历史，提取可记忆的事实。
type LLMSimpleExtractor struct {
	// SummarizeFunc 调用 LLM 同步获取响应。
	// 由调用方注入（避免循环依赖）。
	SummarizeFunc func(ctx context.Context, prompt string) (string, error)
}

// Extract 实现 Extractor 接口。
func (e *LLMSimpleExtractor) Extract(ctx context.Context, messages []core.Message) ([]Trigger, error) {
	if e.SummarizeFunc == nil {
		return nil, fmt.Errorf("SummarizeFunc not set")
	}

	// 构造 prompt：让 LLM 提取可记忆的事实
	var sb strings.Builder
	sb.WriteString("请分析以下对话，提取需要长期记忆的关键信息（如用户偏好、个人信息、重要事实）。\n")
	sb.WriteString("按格式输出：KEY=VALUE 每行一个。如果没有可记忆的内容，输出 NONE。\n\n")
	sb.WriteString("对话：\n")
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

	// 解析 LLM 输出
	return parseExtractionResult(response, SourceLLMExtract), nil
}

var extractionRegex = regexp.MustCompile(`^([a-zA-Z0-9._-]+)\s*=\s*(.+)$`)

func parseExtractionResult(response string, source TriggerSource) []Trigger {
	triggers := []Trigger{}
	now := time.Now()
	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "NONE" {
			continue
		}
		m := extractionRegex.FindStringSubmatch(line)
		if len(m) >= 3 {
			triggers = append(triggers, Trigger{
				Source: source,
				Key:    strings.TrimSpace(m[1]),
				Value:  strings.TrimSpace(m[2]),
				Time:   now,
			})
		}
	}
	return triggers
}

// --- 3. 主流程 ---

// ProcessUserInput 处理用户输入，提取并保存记忆。
// 返回触发的记忆数。
func (a *AutoLearner) ProcessUserInput(text string) int {
	if a.mem == nil {
		return 0
	}
	triggers := ExtractFromUserInput(text)
	return a.apply(triggers)
}

// ProcessToolResult 处理工具结果，提取并保存记忆。
func (a *AutoLearner) ProcessToolResult(text string) int {
	if a.mem == nil {
		return 0
	}
	triggers := ExtractFromToolResult(text)
	return a.apply(triggers)
}

// MaybeExtract 异步检查是否需要 LLM 提取。
// 每 ExtractEveryN 轮触发一次。返回是否实际触发了提取。
func (a *AutoLearner) MaybeExtract(ctx context.Context, messages []core.Message, extractor Extractor) bool {
	if !a.settings.AutoLearn || extractor == nil || a.mem == nil {
		return false
	}

	a.mu.Lock()
	a.counter++
	shouldExtract := a.counter%a.settings.ExtractEveryN == 0
	a.mu.Unlock()

	if !shouldExtract {
		return false
	}

	triggers, err := extractor.Extract(ctx, messages)
	if err != nil {
		return false
	}

	a.apply(triggers)
	return true
}

// apply 应用 triggers 到 memory（去重 + 持久化）。
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
		_ = a.mem.Save() // 异步保存不阻塞
	}
	return count
}
