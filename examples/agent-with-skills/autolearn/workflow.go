package autolearn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"pi-ai-go/core"
	"regexp"
	"strings"
	"time"
)

// Skill 表示从对话中自动提取的可复用工作流。
type Skill struct {
	Name        string   // skill 名称（kebab-case）
	Trigger     string   // 触发场景描述
	Description string   // 简短描述（≤200 字符）
	Steps       []string // 分步操作列表
	Output      string   // 预期输出
	Source      string   // 来源对话摘要
	CreatedAt   time.Time
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

// RenderSKILLMd 渲染 SKILL.md 内容（带 frontmatter）。
func (s Skill) RenderSKILLMd() string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", s.Name))
	desc := s.Description
	if desc == "" {
		desc = s.Trigger
	}
	if desc != "" {
		sb.WriteString(fmt.Sprintf("description: %s\n", escapeYaml(desc)))
	}
	sb.WriteString("source: auto-extracted\n")
	sb.WriteString(fmt.Sprintf("extractedAt: %s\n", s.CreatedAt.Format(time.RFC3339)))
	sb.WriteString("---\n\n")

	sb.WriteString(fmt.Sprintf("# %s\n\n", s.Name))
	if s.Trigger != "" {
		sb.WriteString(fmt.Sprintf("> **触发场景**：%s\n\n", s.Trigger))
	}
	if s.Description != "" {
		sb.WriteString(fmt.Sprintf("**描述**：%s\n\n", s.Description))
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
		sb.WriteString(fmt.Sprintf("## 来源\n\n%s\n", s.Source))
	}
	return sb.String()
}

// escapeYaml 简单转义 YAML 字符串中的特殊字符。
func escapeYaml(s string) string {
	s = strings.ReplaceAll(s, "\"", "\\\"")
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

- 如果**没有**可提取的工作流 → 单独输出 ` + "`NOWORKFLOW`" + `。

【严格要求】
- NAME 必须用 kebab-case（只含小写字母、数字、横线）
- 步骤数量 ≥ 3，否则不算工作流
- 不要输出 ` + "`WORKFLOW_START`" + ` 之外的内容
- 不要重复提取同名的 workflow

对话：
`
}

// WorkflowExtractor 独立的工作流提取器。
// 行为与 LLMSimpleExtractor 类似，但专注于识别可复用工作流而非事实。
type WorkflowExtractor struct {
	// SummarizeFunc 调用 LLM 同步获取响应。由调用方注入。
	SummarizeFunc func(ctx context.Context, prompt string) (string, error)
}

// Extract 提取 0~N 个 Skill。
func (e *WorkflowExtractor) Extract(ctx context.Context, messages []core.Message) ([]Skill, error) {
	if e.SummarizeFunc == nil {
		return nil, fmt.Errorf("workflow: SummarizeFunc not set")
	}

	// 构造 prompt
	var sb strings.Builder
	sb.WriteString(buildWorkflowExtractionPrompt())
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

	skills := parseWorkflowBlocks(response)
	return skills, nil
}
