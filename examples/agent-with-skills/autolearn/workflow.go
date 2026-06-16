package autolearn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"pi-ai-go/core"
)

// Skill 表示从对话中自动提取的可复用工作流。
//
// 渲染策略：完整 SKILL.md 由 LLM 直接生成（遵循 skill-writer 规范），
// 本结构体只保存结构化的中间结果。
type Skill struct {
	Name        string    // skill 名称（kebab-case）
	Trigger     string    // 触发场景描述
	Description string    // 简短描述（≤200 字符）
	Steps       []string  // 分步操作列表
	Output      string    // 预期输出
	Source      string    // 来源对话摘要
	CreatedAt   time.Time // 创建时间
}

// workflowBlockRegex 匹配 LLM 输出的 WORKFLOW 块。
//
// 格式：
//
//	WORKFLOW_START
//	NAME: test-driven-go
//	TRIGGER: 用户要求写 Go 函数时
//	DESCRIPTION: 强制 TDD 流程
//	STEP: 定义函数签名
//	STEP: 写失败测试
//	STEP: 实现函数直到测试通过
//	OUTPUT: 通过测试的 Go 代码
//	SOURCE: 用户在 3 次对话中都先写测试再实现
//	WORKFLOW_END
var workflowBlockRegex = regexp.MustCompile(`(?s)WORKFLOW_START\s*\n(.*?)\n\s*WORKFLOW_END`)

// parseWorkflowBlocks 从 LLM 输出解析 WORKFLOW 块。
func parseWorkflowBlocks(response string) []Skill {
	var skills []Skill
	matches := workflowBlockRegex.FindAllStringSubmatch(response, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		body := m[1]
		skill := Skill{
			CreatedAt: time.Now(),
		}
		var steps []string
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			switch {
			case strings.HasPrefix(line, "NAME:"):
				skill.Name = sanitizeName(strings.TrimSpace(strings.TrimPrefix(line, "NAME:")))
			case strings.HasPrefix(line, "TRIGGER:"):
				skill.Trigger = strings.TrimSpace(strings.TrimPrefix(line, "TRIGGER:"))
			case strings.HasPrefix(line, "DESCRIPTION:"):
				skill.Description = strings.TrimSpace(strings.TrimPrefix(line, "DESCRIPTION:"))
			case strings.HasPrefix(line, "STEP:"):
				step := strings.TrimSpace(strings.TrimPrefix(line, "STEP:"))
				if step != "" {
					steps = append(steps, step)
				}
			case strings.HasPrefix(line, "OUTPUT:"):
				skill.Output = strings.TrimSpace(strings.TrimPrefix(line, "OUTPUT:"))
			case strings.HasPrefix(line, "SOURCE:"):
				skill.Source = strings.TrimSpace(strings.TrimPrefix(line, "SOURCE:"))
			}
		}
		skill.Steps = steps
		// 校验：必须至少有 NAME、TRIGGER、1 个 STEP
		if skill.Name == "" || skill.Trigger == "" || len(skill.Steps) == 0 {
			continue
		}
		skills = append(skills, skill)
	}
	return skills
}

// sanitizeName 把 skill 名规范化为 kebab-case。
func sanitizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// 替换非法字符
	reg := regexp.MustCompile(`[^a-z0-9\-_]+`)
	s = reg.ReplaceAllString(s, "-")
	// 合并连续横线
	reg2 := regexp.MustCompile(`-+`)
	s = reg2.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-_")
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}

// RenderSKILLMd 把结构化 Skill 渲染为 SKILL.md 内容。
//
// 这是回退路径，模板简单但符合基本规范。
// 优先使用 ExtractSkillMd（让 LLM 按 skill-writer 规范直接生成完整 SKILL.md）。
func (s Skill) RenderSKILLMd() string {
	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", s.Name))
	sb.WriteString(fmt.Sprintf("description: %s\n", s.buildDescription()))
	sb.WriteString("---\n\n")

	sb.WriteString(fmt.Sprintf("# %s\n\n", s.Name))

	// 触发场景
	trigger := strings.TrimRight(s.Trigger, ".。")
	if trigger != "" {
		sb.WriteString(fmt.Sprintf("> **Use when** the user %s\n\n", trigger))
	}

	// 描述
	if s.Description != "" {
		sb.WriteString(fmt.Sprintf("## 概述\n\n%s\n\n", s.Description))
	}

	// 步骤（编号清单，祈使语气）
	if len(s.Steps) > 0 {
		sb.WriteString("## 步骤\n\n")
		for i, step := range s.Steps {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
		}
		sb.WriteString("\n")
	}

	// 输出
	if s.Output != "" {
		sb.WriteString(fmt.Sprintf("## 输出\n\n%s\n\n", s.Output))
	}

	// 来源
	if s.Source != "" {
		sb.WriteString(fmt.Sprintf("---\n\n> 来源：%s\n", s.Source))
	}

	return sb.String()
}

// buildDescription 构造 frontmatter 中的 trigger-rich description。
func (s Skill) buildDescription() string {
	core := s.Description
	if core == "" {
		core = s.Trigger
	}
	if core == "" {
		core = "Auto-extracted workflow"
	}
	core = strings.TrimSpace(core)
	trigger := strings.TrimRight(s.Trigger, ".。")
	if trigger == "" {
		trigger = "asks for this kind of task"
	}
	return escapeYamlString(fmt.Sprintf("%s. Use when the user %s.", core, trigger))
}

// escapeYamlString 把字符串安全地包成 YAML 双引号字符串。
func escapeYamlString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return "\"" + s + "\""
}

// WriteSKILLMd 把 skill 写入目录 baseDir/<name>/SKILL.md。
// 如果目录已存在同名 skill，默认覆盖（用户后续可手工调整）。
func (s Skill) WriteSKILLMd(baseDir string) (string, error) {
	if s.Name == "" {
		return "", fmt.Errorf("workflow: skill name is empty")
	}
	if baseDir == "" {
		return "", fmt.Errorf("workflow: baseDir is empty")
	}
	dir := filepath.Join(baseDir, s.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "SKILL.md")
	content := s.RenderSKILLMd()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// buildWorkflowExtractionPrompt 构造从对话中提取 workflow 的 prompt。
// 独立于事实提取，让 LLM 单独输出 WORKFLOW 块。
func buildWorkflowExtractionPrompt() string {
	return `你是工作流提取助手。请分析下面的对话，判断**本次对话是否体现了一个可复用的工作流**。

【什么样的对话需要提取 workflow】
- 用户在一次对话中执行了多步操作（≥3 步）形成完整流程
- 用户明确说"以后都这样做"、"记住这个流程"、"每次 X 都要 Y"等
- 用户在多个轮次中重复了类似的步骤序列
- 一次对话内解决了某种结构化问题（如部署、PR review、错误排查）

【输出格式】
- 如果**有**可提取的工作流，按下面格式输出（**只输出一次 WORKFLOW 块**）：
  WORKFLOW_START
  NAME: <kebab-case 名称，如 test-driven-go>
  TRIGGER: <触发场景，1 句话>
  DESCRIPTION: <简短描述，≤100 字符>
  STEP: <步骤 1>
  STEP: <步骤 2>
  STEP: <步骤 3>
  OUTPUT: <预期输出>
  SOURCE: <来源说明，如"用户在 3 轮对话中重复此流程">
  WORKFLOW_END

- 如果**没有**可提取的工作流 → 单独输出 NOWORKFLOW。

【严格要求】
- NAME 必须用 kebab-case（只含小写字母、数字、横线）
- 步骤数量 ≥ 3，否则不算工作流
- 不要输出 WORKFLOW_START 之外的内容
- 不要重复提取同名的 workflow

对话：
`
}

// buildSkillWriterPrompt 构造"按 skill-writer 规范生成完整 SKILL.md"的 prompt。
// 这是更高级的路径：先检测可复用 workflow，然后用 skill-writer 规范直接生成完整 SKILL.md。
//
// skillWriterDoc 参数是 skill-writer/SKILL.md 的完整内容，作为参考规范。
func buildSkillWriterPrompt(skillWriterDoc string) string {
	var sb strings.Builder
	sb.WriteString("你是 skill 自动生成器。请按 `skill-writer` 规范，从对话中识别可复用的工作流，\n")
	sb.WriteString("并直接生成符合规范的完整 SKILL.md 内容。\n\n")
	sb.WriteString("【参考规范：skill-writer 的核心要求】\n")
	sb.WriteString("```\n")
	sb.WriteString(skillWriterDoc)
	sb.WriteString("\n```\n\n")
	sb.WriteString("【你的任务】\n")
	sb.WriteString("1. 阅读下面的对话，判断是否包含可复用工作流（≥3 步）\n")
	sb.WriteString("2. 如果**没有** → 单独输出 NOWORKFLOW\n")
	sb.WriteString("3. 如果**有** → 按 skill-writer 规范生成**完整 SKILL.md 内容**，输出格式：\n")
	sb.WriteString("\n")
	sb.WriteString("   SKILL_START\n")
	sb.WriteString("   <完整的 SKILL.md 内容，包含 YAML frontmatter、imperative 步骤、trigger-rich description>\n")
	sb.WriteString("   SKILL_END\n")
	sb.WriteString("\n")
	sb.WriteString("【SKILL.md 必须包含】\n")
	sb.WriteString("- YAML frontmatter: name (kebab-case) + description (trigger-rich, 包含 \"Use when\" 子句)\n")
	sb.WriteString("- title (# name)\n")
	sb.WriteString("- 触发条件（blockquote 格式）\n")
	sb.WriteString("- 步骤（编号清单，祈使语气 imperative voice）\n")
	sb.WriteString("- 输出（预期产物）\n")
	sb.WriteString("- 在文末用一段引用文本标注 `> Auto-generated by autolearn from conversation`\n")
	sb.WriteString("\n")
	sb.WriteString("【严格要求】\n")
	sb.WriteString("- 不要修改或评论参考规范\n")
	sb.WriteString("- SKILL.md 内容必须自包含、可直接写入文件\n")
	sb.WriteString("- 步骤数量 ≥ 3\n")
	sb.WriteString("- 不要在 SKILL_START/END 之外输出解释\n")
	sb.WriteString("\n")
	sb.WriteString("对话：\n")
	return sb.String()
}

// skillMdBlockRegex 匹配 LLM 输出的完整 SKILL.md 块。
var skillMdBlockRegex = regexp.MustCompile(`(?s)SKILL_START\s*\n(.*?)\n\s*SKILL_END`)

// parseSkillMdBlocks 从 LLM 输出解析 SKILL_START...SKILL_END 块。
// 返回 0~N 个完整 SKILL.md 内容字符串。
func parseSkillMdBlocks(response string) []string {
	matches := skillMdBlockRegex.FindAllStringSubmatch(response, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		s := strings.TrimSpace(m[1])
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// WorkflowExtractor 独立的工作流提取器。
// 行为与 LLMSimpleExtractor 类似，但专注于识别可复用工作流而非事实。
type WorkflowExtractor struct {
	// SummarizeFunc 调用 LLM 同步获取响应。由调用方注入。
	SummarizeFunc func(ctx context.Context, prompt string) (string, error)

	// SkillWriterDoc skill-writer 的 SKILL.md 完整内容。
	// 加载后，ExtractSkillMd 会用它作为参考规范，让 LLM 直接输出符合 skill-writer 标准的 SKILL.md。
	// 如果为空，则回退到结构化提取（Extract → Skill.RenderSKILLMd）。
	SkillWriterDoc string
}

// Extract 提取 0~N 个结构化 Skill（回退路径，LLM 输出 WORKFLOW_START 块）。
func (e *WorkflowExtractor) Extract(ctx context.Context, messages []core.Message) ([]Skill, error) {
	if e.SummarizeFunc == nil {
		return nil, fmt.Errorf("workflow: SummarizeFunc not set")
	}

	var sb strings.Builder
	sb.WriteString(buildWorkflowExtractionPrompt())
	e.appendMessages(&sb, messages)

	response, err := e.SummarizeFunc(ctx, sb.String())
	if err != nil {
		return nil, err
	}

	return parseWorkflowBlocks(response), nil
}

// ExtractSkillMd 让 LLM 直接生成完整的 SKILL.md 内容（按 skill-writer 规范）。
//
// 工作流程：
//  1. 把对话消息附加到 prompt
//  2. 把 skill-writer/SKILL.md 作为参考规范告诉 LLM
//  3. LLM 输出 SKILL_START...SKILL_END 块，里面是完整 SKILL.md
//  4. 解析后返回字符串切片（每个元素是一个完整 SKILL.md）
//
// 返回的字符串可以直接 WriteFile 到 <dir>/<name>/SKILL.md。
func (e *WorkflowExtractor) ExtractSkillMd(ctx context.Context, messages []core.Message) ([]string, error) {
	if e.SummarizeFunc == nil {
		return nil, fmt.Errorf("workflow: SummarizeFunc not set")
	}
	if e.SkillWriterDoc == "" {
		return nil, fmt.Errorf("workflow: SkillWriterDoc not set; cannot use ExtractSkillMd")
	}

	var sb strings.Builder
	sb.WriteString(buildSkillWriterPrompt(e.SkillWriterDoc))
	e.appendMessages(&sb, messages)

	response, err := e.SummarizeFunc(ctx, sb.String())
	if err != nil {
		return nil, err
	}

	return parseSkillMdBlocks(response), nil
}

// appendMessages 把对话消息追加到 builder。
func (e *WorkflowExtractor) appendMessages(sb *strings.Builder, messages []core.Message) {
	for _, msg := range messages {
		switch m := msg.(type) {
		case core.UserMessage:
			fmt.Fprintf(sb, "用户: %v\n", m.Content)
		case core.AssistantMessage:
			var text string
			for _, b := range m.Content {
				if c, ok := b.(core.TextContent); ok {
					text += c.Text
				}
			}
			fmt.Fprintf(sb, "助手: %s\n", text)
		}
	}
}

// ExtractSkillName 从完整 SKILL.md 内容中提取 frontmatter 的 name 字段。
// 用于确定文件应该写到哪个子目录。
func ExtractSkillName(skillMd string) string {
	re := regexp.MustCompile(`(?m)^name:\s*([a-z0-9][a-z0-9\-_]*)\s*$`)
	m := re.FindStringSubmatch(skillMd)
	if len(m) >= 2 {
		return sanitizeName(m[1])
	}
	return ""
}
