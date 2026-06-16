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
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"examples/agent-with-skills/memory"
	"pi-ai-go/core"
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
		AutoLearn:     false, // 默认关闭 LLM 提取（避免开销）
		ExtractEveryN: 5,     // 每 5 轮提取一次
		MinConfidence: 0.7,
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

	// 可选：workflow 输出目录（不为空时自动把提取的 skill 写入）
	WorkflowDir string
}

// New 创建自动学习器。
func New(mem *memory.Memory, settings Settings) *AutoLearner {
	return &AutoLearner{
		settings: settings,
		mem:      mem,
	}
}

// Settings 返回当前 settings。
func (a *AutoLearner) Settings() Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings
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
//
//	"请记住：user.name=小明"
//	"记住：user.name=小明"
//	"[remember:user.name=小明]"
//	"[memorize:user.name=小明]"
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
//
//	"REMEMBER:user.name=小明"
//	"SAVE_MEMORY:preference.language=zh-CN"
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

// ExtractFromNaturalLanguage 从自然语言中提取常见记忆事实。
// 不依赖 LLM，覆盖最常见的命名/自我介绍/偏好场景。
//
// 匹配示例：
//
//	"你叫小七"                          → assistant.name=小七
//	"你的名字叫小明"                      → assistant.name=小明
//	"从现在开始你是小七"                   → assistant.name=小七
//	"我叫张三"                            → user.name=张三
//	"我来自杭州"                          → user.city=杭州
//	"我喜欢 Python"                       → user.preferred_language=Python
var (
	namingAssistantRegex = regexp.MustCompile(`(?i)(?:你|助手|机器人|AI)[的]?名字(?:叫|是)\s*[` + "`" + `"'「]?([^` + "`" + `"'」\s，。,.!?！？]{1,20})`)
	beingCalledRegex     = regexp.MustCompile(`(?i)(?:你|助手|机器人|AI)(?:就是|叫|是)\s*[` + "`" + `"'「]?([^` + "`" + `"'」\s，。,.!?！？]{1,20})`)
	namingUserRegex      = regexp.MustCompile(`(?i)(?:我)[的]?名字(?:叫|是)\s*[` + "`" + `"'「]?([^` + "`" + `"'」\s，。,.!?！？]{1,20})`)
	introUserNameRegex   = regexp.MustCompile(`(?i)^(?:我(?:叫|是))\s*[` + "`" + `"'「]?([^` + "`" + `"'」\s，。,.!?！？\n]{1,20})`)
	introCityRegex       = regexp.MustCompile(`(?i)(?:我)?(?:来自|在|住|是)\s*([\p{Han}A-Za-z]{2,15}(?:市|省|区|县|国|州)?)`)
	preferredLangRegex   = regexp.MustCompile(`(?i)(?:用|请用|使用|讲|说)\s*([\p{Han}A-Za-z]+)\s*(?:回答|交流|沟通|回复)`)
)

func ExtractFromNaturalLanguage(text string) []Trigger {
	triggers := []Trigger{}
	now := time.Now()
	seen := make(map[string]bool) // 同 key 只保留第一次

	add := func(key, value string) {
		if value == "" || seen[key] {
			return
		}
		seen[key] = true
		triggers = append(triggers, Trigger{
			Source: SourceUserInput,
			Key:    key,
			Value:  value,
			Time:   now,
		})
	}

	for _, m := range namingAssistantRegex.FindAllStringSubmatch(text, -1) {
		if len(m) >= 2 {
			add("assistant.name", strings.TrimSpace(m[1]))
		}
	}
	for _, m := range beingCalledRegex.FindAllStringSubmatch(text, -1) {
		if len(m) >= 2 {
			add("assistant.name", strings.TrimSpace(m[1]))
		}
	}
	for _, m := range namingUserRegex.FindAllStringSubmatch(text, -1) {
		if len(m) >= 2 {
			add("user.name", strings.TrimSpace(m[1]))
		}
	}
	for _, m := range introUserNameRegex.FindAllStringSubmatch(text, -1) {
		if len(m) >= 2 {
			v := strings.TrimSpace(m[1])
			// 过滤掉一些无效值
			if !isCommonVerb(v) {
				add("user.name", v)
			}
		}
	}
	for _, m := range introCityRegex.FindAllStringSubmatch(text, -1) {
		if len(m) >= 2 {
			add("user.location", strings.TrimSpace(m[1]))
		}
	}
	for _, m := range preferredLangRegex.FindAllStringSubmatch(text, -1) {
		if len(m) >= 2 {
			add("user.preferred_language", strings.TrimSpace(m[1]))
		}
	}

	return triggers
}

func isCommonVerb(s string) bool {
	common := []string{"是", "的", "你", "我", "他", "她", "它", "要", "有", "在", "从", "到", "叫", "了", "吗", "吧"}
	for _, c := range common {
		if s == c {
			return true
		}
	}
	return false
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
	sb.WriteString("你是记忆提取助手。请从下面的对话中找出需要**长期记住**的事实。\n")
	sb.WriteString("\n")
	sb.WriteString("【输出格式】每行一条 `KEY=VALUE`（或 `KEY: VALUE`），单独成行。\n")
	sb.WriteString("  - KEY 必须是下方白名单内的 key（不允许自定义 key）。\n")
	sb.WriteString("  - VALUE 是简短的事实值（不要带「是」「了」「啊」等修饰词）。\n")
	sb.WriteString("  - 没有任何值得记住的内容 → 单独输出 `NONE`。\n")
	sb.WriteString("\n")
	sb.WriteString("【允许的 KEY 白名单】\n")
	sb.WriteString("A. 用户身份（user.*）\n")
	sb.WriteString("  - user.name: 用户姓名（如「我叫张三」→ user.name=张三）\n")
	sb.WriteString("  - user.age: 年龄（如「我30岁」→ user.age=30）\n")
	sb.WriteString("  - user.gender: 性别（如「我是男生」→ user.gender=男）\n")
	sb.WriteString("  - user.role: 职业身份（如「我是一名Python开发者」→ user.role=Python开发者）\n")
	sb.WriteString("  - user.company: 公司/单位（如「我在阿里工作」→ user.company=阿里）\n")
	sb.WriteString("  - user.location: 所在地（如「我来自杭州」→ user.location=杭州）\n")
	sb.WriteString("\n")
	sb.WriteString("B. 用户偏好（user.preference_*）\n")
	sb.WriteString("  - user.preferred_language: 语言偏好（如「用中文回答」→ user.preferred_language=中文）\n")
	sb.WriteString("  - user.preferred_response_style: 回答风格（如「回答简短点」→ user.preferred_response_style=简短）\n")
	sb.WriteString("  - user.dislike_format: 不喜欢的格式（如「不要用表格」→ user.dislike_format=表格）\n")
	sb.WriteString("\n")
	sb.WriteString("C. AI 自身信息（assistant.*）\n")
	sb.WriteString("  - assistant.name: AI 名字（如「你叫小七」→ assistant.name=小七）\n")
	sb.WriteString("  - assistant.location: AI 来源地（如「你是杭州的AI」→ assistant.location=杭州）\n")
	sb.WriteString("  - assistant.personality: 人设风格（如「你风格幽默」→ assistant.personality=幽默）\n")
	sb.WriteString("\n")
	sb.WriteString("D. 当前任务/项目（task.* / project.*）\n")
	sb.WriteString("  - task.current: 当前任务（如「我在开发电商网站」→ task.current=开发电商网站）\n")
	sb.WriteString("  - project.tech_stack: 技术栈（如「我们用React」→ project.tech_stack=React）\n")
	sb.WriteString("  - project.client: 项目客户（如「客户是XXX公司」→ project.client=XXX公司）\n")
	sb.WriteString("\n")
	sb.WriteString("E. 关键事实/决策（fact.* / decision.*）\n")
	sb.WriteString("  - fact.<具体>: 关键事实（如「这个bug是空指针」→ fact.bug_root_cause=空指针）\n")
	sb.WriteString("  - decision.<具体>: 已做决策（如「决定用PostgreSQL」→ decision.database=PostgreSQL）\n")
	sb.WriteString("  - constraint.<具体>: 约束条件（如「API限速100次/天」→ constraint.api_rate_limit=100次/天）\n")
	sb.WriteString("\n")
	sb.WriteString("F. 关系/家庭/宠物\n")
	sb.WriteString("  - relation.spouse: 配偶（如「我老婆叫小丽」→ relation.spouse=小丽）\n")
	sb.WriteString("  - family.<角色>.<属性>: 家庭成员（如「我儿子5岁」→ family.son.age=5）\n")
	sb.WriteString("  - pet.<角色>.name: 宠物名（如「我家狗叫旺财」→ pet.dog.name=旺财）\n")
	sb.WriteString("\n")
	sb.WriteString("G. 健康/饮食\n")
	sb.WriteString("  - health.allergy: 过敏源（如「我对花生过敏」→ health.allergy=花生）\n")
	sb.WriteString("  - diet.preference: 饮食习惯（如「我不吃辣」→ diet.preference=不吃辣）\n")
	sb.WriteString("\n")
	sb.WriteString("H. 重要日期\n")
	sb.WriteString("  - date.birthday: 生日（如「我生日是3月15日」→ date.birthday=03-15）\n")
	sb.WriteString("  - date.<其他>: 其他重要日期（如纪念日、deadline 等）\n")
	sb.WriteString("\n")
	sb.WriteString("I. 资产/设备\n")
	sb.WriteString("  - asset.laptop / asset.phone: 设备（如「我电脑是MacBook Pro」→ asset.laptop=MacBook Pro）\n")
	sb.WriteString("\n")
	sb.WriteString("J. 沟通风格\n")
	sb.WriteString("  - style.verbosity: 详细度（如「别太啰嗦」→ style.verbosity=低）\n")
	sb.WriteString("  - style.formality: 正式度（如「随便聊聊」→ style.formality=随意）\n")
	sb.WriteString("\n")
	sb.WriteString("K. 常用工具\n")
	sb.WriteString("  - tool.editor: 编辑器（如「我常用VSCode」→ tool.editor=VSCode）\n")
	sb.WriteString("  - tool.<其他>: 常用工具/命令\n")
	sb.WriteString("\n")
	sb.WriteString("L. 用户目标\n")
	sb.WriteString("  - goal.learn: 学习目标（如「我想学Go」→ goal.learn=Go）\n")
	sb.WriteString("  - goal.<其他>: 其他目标\n")
	sb.WriteString("\n")
	sb.WriteString("M. 痛点/抱怨\n")
	sb.WriteString("  - pain.<具体>: 痛点（如「Python打包太慢」→ pain.build_speed=Python打包慢）\n")
	sb.WriteString("\n")
	sb.WriteString("【严格禁止】\n")
	sb.WriteString("- 不要输出 KEY 白名单之外的自定义 key\n")
	sb.WriteString("- 不要输出「好的，记住了」等客套话\n")
	sb.WriteString("- 不要输出 markdown 代码块（```）或编号列表\n")
	sb.WriteString("- 不要在 VALUE 里加解释或前后缀（如「assistant.name=好的，记住了」是错的）\n")
	sb.WriteString("- 不要重复输出同一 KEY\n")
	sb.WriteString("\n")
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

// allowedKeyPrefixes 是 LLM 提取允许的 key 前缀白名单。
// 不在白名单内的 key 一律丢弃，防止 LLM 自由发挥产生噪声。
// 顺序无关：只要有一个前缀匹配即可。
var allowedKeyPrefixes = []string{
	// A. 用户
	"user.",
	// C. AI 自身
	"assistant.",
	// D. 任务/项目
	"task.",
	"project.",
	// E. 事实/决策/约束
	"fact.",
	"decision.",
	"constraint.",
	// F. 关系/家庭/宠物
	"relation.",
	"family.",
	"pet.",
	// G. 健康/饮食
	"health.",
	"diet.",
	// H. 重要日期
	"date.",
	// I. 资产/设备
	"asset.",
	// J. 沟通风格
	"style.",
	// K. 常用工具
	"tool.",
	// L. 用户目标
	"goal.",
	// M. 痛点/抱怨
	"pain.",
}

// allowedKeyPrefix 检查 key 是否在白名单前缀内。
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
	triggers := []Trigger{}
	now := time.Now()
	seen := make(map[string]bool) // 去重

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
		value := strings.TrimSpace(m[2])
		// 去掉首尾的引号/书名号
		value = strings.Trim(value, "\"'「」『』")

		// 过滤：空白 key/value
		if key == "" || value == "" {
			continue
		}
		// 过滤：不在白名单内的 key（仅对 LLM 来源生效）
		if source == SourceLLMExtract && !allowedKeyPrefix(key) {
			continue
		}
		// 过滤：VALUE 太长（>200 字符）通常说明 LLM 把整段话塞进去了
		if len(value) > 200 {
			value = value[:200]
		}
		// 去重
		if seen[key] {
			continue
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

// --- 3. 主流程 ---

// ProcessUserInput 处理用户输入，提取并保存记忆。
// 返回触发的记忆数。
func (a *AutoLearner) ProcessUserInput(text string) int {
	if a.mem == nil {
		return 0
	}
	triggers := append(ExtractFromUserInput(text), ExtractFromNaturalLanguage(text)...)
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

// MaybeExtractWorkflow 异步检查是否需要 LLM 提取工作流。
// 优先调用 ExtractSkillMd（按 skill-writer 规范直接生成完整 SKILL.md），
// 回退到 Extract（结构化提取 → 模板渲染）。
// 返回实际写入的 skill 数。
func (a *AutoLearner) MaybeExtractWorkflow(ctx context.Context, messages []core.Message, extractor *WorkflowExtractor) int {
	if !a.settings.AutoLearn || extractor == nil || a.WorkflowDir == "" {
		return 0
	}

	// 路径 1：直接生成符合 skill-writer 规范的完整 SKILL.md
	if extractor.SkillWriterDoc != "" {
		contents, err := extractor.ExtractSkillMd(ctx, messages)
		if err == nil && len(contents) > 0 {
			count := 0
			for _, content := range contents {
				name := ExtractSkillName(content)
				if name == "" {
					continue
				}
				dir := filepath.Join(a.WorkflowDir, name)
				if err := os.MkdirAll(dir, 0755); err != nil {
					continue
				}
				path := filepath.Join(dir, "SKILL.md")
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					continue
				}
				count++
			}
			return count
		}
	}

	// 路径 2（回退）：结构化提取 → 模板渲染
	skills, err := extractor.Extract(ctx, messages)
	if err != nil || len(skills) == 0 {
		return 0
	}

	count := 0
	for _, s := range skills {
		if _, err := s.WriteSKILLMd(a.WorkflowDir); err != nil {
			continue
		}
		count++
	}
	return count
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
