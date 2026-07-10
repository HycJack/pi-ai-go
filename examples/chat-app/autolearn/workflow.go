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

type Skill struct {
	Name        string
	Trigger     string
	Description string
	Steps       []string
	Output      string
	Source      string
	CreatedAt   time.Time
}

var workflowBlockRegex = regexp.MustCompile(`(?s)WORKFLOW_START\s*\n(.*?)\n\s*WORKFLOW_END`)

func parseWorkflowBlocks(response string) []Skill {
	var skills []Skill
	matches := workflowBlockRegex.FindAllStringSubmatch(response, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		body := m[1]
		skill := Skill{CreatedAt: time.Now()}
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
		if skill.Name == "" || skill.Trigger == "" || len(skill.Steps) == 0 {
			continue
		}
		skills = append(skills, skill)
	}
	return skills
}

func sanitizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	reg := regexp.MustCompile(`[^a-z0-9\-_]+`)
	s = reg.ReplaceAllString(s, "-")
	reg2 := regexp.MustCompile(`-+`)
	s = reg2.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-_")
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}

func (s Skill) RenderSKILLMd() string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", s.Name))
	sb.WriteString(fmt.Sprintf("description: %s\n", s.buildDescription()))
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("# %s\n\n", s.Name))
	if s.Trigger != "" {
		sb.WriteString(fmt.Sprintf("> **Use when** the user %s\n\n", s.Trigger))
	}
	if s.Description != "" {
		sb.WriteString(fmt.Sprintf("## 概述\n\n%s\n\n", s.Description))
	}
	if len(s.Steps) > 0 {
		sb.WriteString("## 步骤\n\n")
		for i, step := range s.Steps {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
		}
		sb.WriteString("\n")
	}
	if s.Output != "" {
		sb.WriteString(fmt.Sprintf("## 输出\n\n%s\n\n", s.Output))
	}
	if s.Source != "" {
		sb.WriteString(fmt.Sprintf("---\n\n> 来源：%s\n", s.Source))
	}
	return sb.String()
}

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

func escapeYamlString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return "\"" + s + "\""
}

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
  NAME: <kebab-case 名称>
  TRIGGER: <触发场景，1 句话>
  DESCRIPTION: <简短描述，≤100 字符>
  STEP: <步骤 1>
  STEP: <步骤 2>
  STEP: <步骤 3>
  OUTPUT: <预期输出>
  SOURCE: <来源说明>
  WORKFLOW_END

- 如果**没有**可提取的工作流 → 单独输出 NOWORKFLOW。

对话：
`
}

func buildSkillWriterPrompt(skillWriterDoc string) string {
	var sb strings.Builder
	sb.WriteString("你是 skill 自动生成器。请按 `skill-writer` 规范，从对话中识别可复用的工作流，\n")
	sb.WriteString("并直接生成符合规范的完整 SKILL.md 内容。\n\n")
	if skillWriterDoc != "" {
		sb.WriteString("【参考规范：skill-writer 的核心要求】\n")
		sb.WriteString("```\n")
		sb.WriteString(skillWriterDoc)
		sb.WriteString("\n```\n\n")
	}
	sb.WriteString("【你的任务】\n")
	sb.WriteString("1. 阅读下面的对话，判断是否包含可复用工作流（≥3 步）\n")
	sb.WriteString("2. 如果**没有** → 单独输出 NOWORKFLOW\n")
	sb.WriteString("3. 如果**有** → 按 skill-writer 规范生成**完整 SKILL.md 内容**\n\n")
	sb.WriteString("输出格式：\n")
	sb.WriteString("   SKILL_START\n")
	sb.WriteString("   <完整的 SKILL.md 内容>\n")
	sb.WriteString("   SKILL_END\n\n")
	sb.WriteString("对话：\n")
	return sb.String()
}

var skillMdBlockRegex = regexp.MustCompile(`(?s)SKILL_START\s*\n(.*?)\n\s*SKILL_END`)

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

type WorkflowExtractor struct {
	SummarizeFunc  func(ctx context.Context, prompt string) (string, error)
	SkillWriterDoc string
}

func (e *WorkflowExtractor) Extract(ctx context.Context, messages []core.Message) ([]Skill, error) {
	if e.SummarizeFunc == nil {
		return nil, fmt.Errorf("workflow: SummarizeFunc not set")
	}
	var sb strings.Builder
	sb.WriteString(buildWorkflowExtractionPrompt())
	appendMessages(&sb, messages)
	response, err := e.SummarizeFunc(ctx, sb.String())
	if err != nil {
		return nil, err
	}
	return parseWorkflowBlocks(response), nil
}

func (e *WorkflowExtractor) ExtractSkillMd(ctx context.Context, messages []core.Message) ([]string, error) {
	if e.SummarizeFunc == nil {
		return nil, fmt.Errorf("workflow: SummarizeFunc not set")
	}
	if e.SkillWriterDoc == "" {
		return nil, fmt.Errorf("workflow: SkillWriterDoc not set")
	}
	var sb strings.Builder
	sb.WriteString(buildSkillWriterPrompt(e.SkillWriterDoc))
	appendMessages(&sb, messages)
	response, err := e.SummarizeFunc(ctx, sb.String())
	if err != nil {
		return nil, err
	}
	return parseSkillMdBlocks(response), nil
}

func appendMessages(sb *strings.Builder, messages []core.Message) {
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

func ExtractSkillName(skillMd string) string {
	re := regexp.MustCompile(`(?m)^name:\s*([a-z0-9][a-z0-9\-_]*)\s*$`)
	m := re.FindStringSubmatch(skillMd)
	if len(m) >= 2 {
		return sanitizeName(m[1])
	}
	return ""
}
