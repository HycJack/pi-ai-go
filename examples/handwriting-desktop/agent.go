package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"pi-ai-go/agent"
	"pi-ai-go/core"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================================
// Agent 流程：AI 对话使用 pi-ai-go 的 agent loop
// agent 可以帮助用户：撰写/润色/翻译文本，然后直接生成手写图片。
// ============================================================================

// AgentMessage 处理一个 agent 模式的流式对话请求。
func (a *App) AgentMessage(jsonStr string) error {
	var req AgentRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		LogError("[agent] parse error: %v", err)
		runtime.EventsEmit(a.ctx, "agent-error", fmt.Sprintf("parse error: %v", err))
		return err
	}

	LogInfo("[agent] request: provider=%s model=%s msgLen=%d history=%d",
		req.Provider, req.Model, len(req.Message), len(req.Messages))

	model, _, opts := a.providerFromRequest(req.Provider, req.APIKey, req.BaseURL, req.Model)
	if req.MaxTokens > 0 {
		t := req.MaxTokens
		opts.MaxTokens = &t
	}
	if req.Temperature > 0 {
		t := req.Temperature
		opts.Temperature = &t
	}

	systemPrompt := a.buildSystemPrompt()
	tools := a.handwritingTools()

	config := agent.AgentLoopConfig{
		Model:              model,
		SystemPrompt:       systemPrompt,
		Tools:              tools,
		SimpleStreamOptions: opts,
	}

	messages := a.buildAgentMessages(req)

	streamCtx, cancelFn := context.WithCancel(a.ctx)
	a.cancelFn = cancelFn

	eventStream, detailed := agent.AgentLoopDetailed(streamCtx, messages, config)

	LogInfo("[agent] loop started, messages=%d tools=%d", len(messages), len(config.Tools))

	go func() {
		defer func() {
			a.cancelFn = nil
		}()
		textLen := 0
		_, forEachErr := eventStream.ForEach(streamCtx, func(evt agent.AgentEvent) error {
			switch e := evt.(type) {
			case agent.EventMessageUpdate:
				if e.AssistantEvent != nil {
					switch ae := e.AssistantEvent.(type) {
					case core.EventTextDelta:
						textLen += len(ae.Delta)
						runtime.EventsEmit(a.ctx, "agent-text-delta", ae.Delta)
					case core.EventThinkingDelta:
						runtime.EventsEmit(a.ctx, "agent-thinking-delta", ae.Delta)
					case core.EventToolCallStart:
						data, _ := json.Marshal(map[string]interface{}{"id": ae.ID, "name": ae.Name, "arguments": ""})
						runtime.EventsEmit(a.ctx, "agent-tool-call-start", string(data))
					case core.EventToolCallDelta:
						runtime.EventsEmit(a.ctx, "agent-tool-call-delta", ae.ArgumentsDelta)
					case core.EventToolCallEnd:
						argsStr := string(ae.Arguments)
						safeArgs, _ := json.Marshal(argsStr)
						runtime.EventsEmit(a.ctx, "agent-tool-call-end", string(safeArgs))
					}
				}
			case agent.EventTurnEnd:
				LogInfo("[agent] turn end, text length so far=%d", textLen)
			case agent.EventToolExecStart:
				data, _ := json.Marshal(map[string]interface{}{
					"toolCallID": e.ToolCallID,
					"toolName":   e.ToolName,
				})
				runtime.EventsEmit(a.ctx, "agent-tool-exec-start", string(data))
			case agent.EventToolExecEnd:
				result, _ := json.Marshal(map[string]interface{}{"success": !e.IsError})
				if !e.IsError && len(e.Result) > 0 {
					var toolResult core.AgentToolResult
					if err := json.Unmarshal(e.Result, &toolResult); err == nil {
						var textParts []string
						for _, block := range toolResult.Content {
							if tc, ok := block.(core.TextContent); ok {
								textParts = append(textParts, tc.Text)
							}
						}
						if len(textParts) > 0 {
							runtime.EventsEmit(a.ctx, "agent-tool-result", strings.Join(textParts, "\n"))
						} else {
							runtime.EventsEmit(a.ctx, "agent-tool-result", string(e.Result))
						}
					} else {
						runtime.EventsEmit(a.ctx, "agent-tool-result", string(e.Result))
					}
				}
				runtime.EventsEmit(a.ctx, "agent-tool-exec-end", string(result))
			case agent.EventAgentEnd:
				// agent 完成
			}
			return nil
		})
		if forEachErr != nil {
			LogError("[agent] ForEach error: %v (textLen=%d)", forEachErr, textLen)
			runtime.EventsEmit(a.ctx, "agent-error", fmt.Sprintf("agent error: %v", forEachErr))
			runtime.EventsEmit(a.ctx, "agent-done", "")
			return
		}

		result, err := detailed()
		if err != nil {
			LogError("[agent] detailed() error: %v (textLen=%d)", err, textLen)
			runtime.EventsEmit(a.ctx, "agent-error", fmt.Sprintf("agent error: %v", err))
			runtime.EventsEmit(a.ctx, "agent-done", "")
			return
		}

		LogInfo("[agent] done, total text length=%d, messages=%d", textLen, len(result.Messages))
		if textLen == 0 {
			LogWarn("[agent] done but no text was emitted — agent may have only used tools or returned empty content")
		}

		runtime.EventsEmit(a.ctx, "agent-done", "")
	}()

	return nil
}

// buildSystemPrompt 构建 agent 的系统提示词。
func (a *App) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("你是「手写文字生成器」的智能助手。\n")
	sb.WriteString("你可以帮助用户撰写、润色、翻译文本，并通过工具直接将文本渲染成手写风格的图片。\n\n")

	sb.WriteString("## 你的能力\n")
	sb.WriteString("1. 帮助用户撰写各类文本：作文、论文、信件、笔记、摘抄等\n")
	sb.WriteString("2. 润色和优化用户提供的文本\n")
	sb.WriteString("3. 中英文互译\n")
	sb.WriteString("4. 调整文本格式（分段、添加署名、设置分页）\n\n")

	sb.WriteString("## 排版标记\n")
	sb.WriteString("在文本中可以使用以下独占一行的标记控制排版：\n")
	sb.WriteString("- `---`（三个或更多短横线）：从下一段开始强制分页\n")
	sb.WriteString("- `>>>` 放在行首：该行靠右排版，适合署名和日期\n\n")

	sb.WriteString("## 工具说明\n")
	sb.WriteString("- `generate_handwriting`：将指定文本渲染成手写图片。调用后图片会显示在预览区。\n")
	sb.WriteString("  参数：text（要渲染的文本）、fontSize（字号，默认100）、preview（是否预览，默认true）\n\n")

	sb.WriteString("## 工作流程\n")
	sb.WriteString("1. 理解用户需求，撰写或修改文本\n")
	sb.WriteString("2. 用 `generate_handwriting` 工具渲染成手写图片\n")
	sb.WriteString("3. 用自然语言向用户说明结果\n\n")

	sb.WriteString("## 输出规范\n")
	sb.WriteString("- 使用简洁、专业的中文回复\n")
	sb.WriteString("- 长文本使用 markdown 格式\n")
	sb.WriteString("- 不要在回复中使用 emoji\n")
	return sb.String()
}

// handwritingTools 返回手写生成相关的工具集。
func (a *App) handwritingTools() []agent.AgentTool {
	return []agent.AgentTool{
		{
			Name:        "generate_handwriting",
			Label:       "生成手写",
			Description: "将指定文本渲染成手写风格的图片，并显示在预览区。支持分页标记（---）和右对齐标记（>>>）。",
			Parameters:  json.RawMessage(schemaGenerateHandwriting),
			Execute:     a.executeGenerateHandwriting,
		},
	}
}

const schemaGenerateHandwriting = `{
	"type": "object",
	"properties": {
		"text": {
			"type": "string",
			"description": "要渲染成手写图片的文本内容。可使用 --- 独占一行强制分页，>>> 放在行首使该行右对齐。"
		},
		"fontSize": {
			"type": "integer",
			"description": "字号，默认 100。建议 70-150。"
		},
		"preview": {
			"type": "boolean",
			"description": "是否预览模式（只生成第一页），默认 true。"
		}
	},
	"required": ["text"]
}`

// executeGenerateHandwriting 执行生成手写图片的工具。
func (a *App) executeGenerateHandwriting(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	var args struct {
		Text     string `json:"text"`
		FontSize int    `json:"fontSize"`
		Preview  *bool  `json:"preview"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return errResult("invalid arguments: " + err.Error()), nil
	}
	if strings.TrimSpace(args.Text) == "" {
		return errResult("text 不能为空"), nil
	}

	// 使用当前设置构建渲染参数
	preview := true
	if args.Preview != nil {
		preview = *args.Preview
	}
	fontSize := args.FontSize
	if fontSize <= 0 {
		fontSize = 100
	}

	// 构建渲染参数（使用默认值，前端用户可后续微调）
	renderParams := HandwritingParams{
		Text:           args.Text,
		FontSize:       fontSize,
		LineSpacing:    int(float64(fontSize) * 1.5),
		WordSpacing:    1,
		Fill:           "0,0,0,255",
		Width:          2481,
		Height:         3507,
		MarginTop:      150,
		MarginBottom:   150,
		MarginLeft:     150,
		MarginRight:    150,
		LineSpacingSigma: 1,
		FontSizeSigma:    1,
		WordSpacingSigma: 2,
		PerturbXSigma:    3,
		PerturbYSigma:    3,
		PerturbThetaSigma: 0.05,
		InkDepthSigma:    20,
		StrikethroughProbability: 0,
		StrikethroughWidth: 8,
		IsUnderlined:     true,
		EnableEnglishSpacing: false,
		Preview:          preview,
		FullPreview:      false,
		ExportPDF:        false,
		FontOption:       "GoRegular.ttf",
	}

	renderer, err := NewRenderer(renderParams)
	if err != nil {
		return errResult("创建渲染器失败: " + err.Error()), nil
	}

	pages, err := renderer.Render()
	if err != nil {
		return errResult("渲染失败: " + err.Error()), nil
	}

	if len(pages) == 0 {
		return errResult("未生成任何页面"), nil
	}

	// 通过事件把图片 base64 发射到前端，前端负责显示
	images := pagesToBase64(pages)
	payload, _ := json.Marshal(map[string]interface{}{
		"images":   images,
		"fontSize":  fontSize,
		"preview":   preview,
		"pageCount": len(images),
	})
	runtime.EventsEmit(a.ctx, "handwriting-generated", string(payload))

	summary := fmt.Sprintf("已生成手写图片：%d 页，字号 %d。图片已显示在预览区。", len(images), fontSize)
	return okResult(summary), nil
}

// buildAgentMessages 从 AgentRequest 构建消息历史。
func (a *App) buildAgentMessages(req AgentRequest) []core.Message {
	var messages []core.Message
	for _, m := range req.Messages {
		role, _ := m["role"].(string)
		if role == "" {
			continue
		}
		content, _ := m["content"].(string)
		if role == "user" {
			messages = append(messages, core.UserMessage{Content: content})
		} else if role == "assistant" {
			messages = append(messages, core.AssistantMessage{
				Content: []core.ContentBlock{core.TextContent{Text: content}},
			})
		}
	}
	messages = append(messages, core.UserMessage{Content: req.Message})
	return messages
}

// --- 工具结果辅助函数 ---

func okResult(text string) core.AgentToolResult {
	return core.AgentToolResult{
		Content: []core.ContentBlock{core.TextContent{Type: "text", Text: text}},
	}
}

func errResult(text string) core.AgentToolResult {
	return core.AgentToolResult{
		Content:  []core.ContentBlock{core.TextContent{Type: "text", Text: text}},
		IsError:  true,
	}
}
