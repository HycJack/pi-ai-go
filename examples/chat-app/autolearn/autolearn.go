package autolearn

import (
	"context"
	"fmt"
	"regexp"
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
		ExtractEveryN: 5,
		MinConfidence: 0.7,
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
}

func New(mem *memory.Memory, settings Settings) *AutoLearner {
	return &AutoLearner{
		settings: settings,
		mem:      mem,
	}
}

var (
	rememberRegex   = regexp.MustCompile(`(?:请记住[：:]?|记住[：:]\s*|\[remember:\s*)([^\s=]+)\s*=\s*([^\n\]]+?)(?:\]|$|\n)`)
	memorizeRegex   = regexp.MustCompile(`\[memorize:\s*([^\s=]+)\s*=\s*([^\]]+?)\]`)
	namingUserRegex = regexp.MustCompile(`(?i)(?:我)[的]?名字(?:叫|是)\s*[` + "`" + `"'「]?([^` + "`" + `"'」\s，。,.!?！？]{1,20})`)
)

func ExtractFromUserInput(text string) []Trigger {
	var triggers []Trigger
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

func ExtractFromNaturalLanguage(text string) []Trigger {
	var triggers []Trigger
	now := time.Now()
	seen := make(map[string]bool)
	add := func(key, value string) {
		if value == "" || seen[key] {
			return
		}
		seen[key] = true
		triggers = append(triggers, Trigger{Source: SourceUserInput, Key: key, Value: value, Time: now})
	}
	for _, m := range namingUserRegex.FindAllStringSubmatch(text, -1) {
		if len(m) >= 2 {
			add("user.name", strings.TrimSpace(m[1]))
		}
	}
	return triggers
}

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

var extractionRegex = regexp.MustCompile(`^([^\s=:：]+)\s*[=:：]\s*(.+)$`)

func parseExtractionResult(response string, source TriggerSource) []Trigger {
	var triggers []Trigger
	now := time.Now()
	seen := make(map[string]bool)

	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "NONE" || strings.HasPrefix(line, "#") {
			continue
		}
		m := extractionRegex.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}
		key := strings.TrimSpace(m[1])
		value := strings.Trim(strings.TrimSpace(m[2]), "\"'「」『』")
		if key == "" || value == "" || seen[key] {
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

func (a *AutoLearner) ProcessUserInput(text string) int {
	if a.mem == nil {
		return 0
	}
	triggers := append(ExtractFromUserInput(text), ExtractFromNaturalLanguage(text)...)
	return a.apply(triggers)
}

func (a *AutoLearner) ProcessToolResult(text string) int {
	if a.mem == nil {
		return 0
	}
	triggers := ExtractFromToolResult(text)
	return a.apply(triggers)
}

func ExtractFromToolResult(text string) []Trigger {
	var triggers []Trigger
	now := time.Now()
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
