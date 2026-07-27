package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"pi-ai-go/core"
	"pi-ai-go/llm"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// GenerateCode takes a natural language prompt and returns generated Python
// matplotlib code via the configured LLM provider (non-streaming).
func (a *App) GenerateCode(jsonStr string) (string, error) {
	var req CodeGenRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		return "", fmt.Errorf("parse error: %v", err)
	}

	model := a.resolveModel(req.Provider, req.Model, req.BaseURL)
	apiKey := a.resolveAPIKey(req, model)

	llmCtx := buildLLMContext(req)

	opts := core.SimpleStreamOptions{
		StreamOptions: core.StreamOptions{
			APIKey: apiKey,
		},
	}
	if req.MaxTokens > 0 {
		t := req.MaxTokens
		opts.MaxTokens = &t
	}
	if req.Temperature > 0 {
		opts.Temperature = &req.Temperature
	}

	streamCtx, cancelFn := context.WithCancel(context.Background())
	a.cancelFn = cancelFn

	msg, err := llm.CompleteSimple(streamCtx, model, llmCtx.Messages, opts)
	if err != nil {
		return "", fmt.Errorf("generation error: %v", err)
	}

	var code strings.Builder
	for _, block := range msg.Content {
		if c, ok := block.(core.TextContent); ok {
			code.WriteString(c.Text)
		}
	}

	generatedCode := cleanCode(code.String())
	if generatedCode != "" {
		a.saveConversation(req.Prompt, generatedCode, req.ConvID)
	}

	return generatedCode, nil
}

// StreamGenerateCode streams generated Python code to the frontend.
func (a *App) StreamGenerateCode(jsonStr string) error {
	var req CodeGenRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		runtime.EventsEmit(a.ctx, "codegen-error", fmt.Sprintf("parse error: %v", err))
		return err
	}

	model := a.resolveModel(req.Provider, req.Model, req.BaseURL)
	apiKey := a.resolveAPIKey(req, model)

	llmCtx := buildLLMContext(req)

	opts := core.SimpleStreamOptions{
		StreamOptions: core.StreamOptions{
			APIKey: apiKey,
		},
	}
	if req.MaxTokens > 0 {
		t := req.MaxTokens
		opts.MaxTokens = &t
	}
	if req.Temperature > 0 {
		opts.Temperature = &req.Temperature
	}

	streamCtx, cancelFn := context.WithCancel(context.Background())
	a.cancelFn = cancelFn

	stream, err := llm.StreamSimple(streamCtx, model, llmCtx.Messages, opts)
	if err != nil {
		runtime.EventsEmit(a.ctx, "codegen-error", fmt.Sprintf("stream error: %v", err))
		return err
	}

	var fullText strings.Builder
	inCodeBlock := false
	codeBuf := strings.Builder{}

	safeGo(func() {
		defer func() {
			a.cancelFn = nil
		}()

		_, forEachErr := stream.ForEach(streamCtx, func(event core.AssistantMessageEvent) error {
			switch e := event.(type) {
			case core.EventTextDelta:
				fullText.WriteString(e.Delta)

				// Track code block boundaries
				codeBuf.WriteString(e.Delta)
				content := codeBuf.String()

				if strings.Contains(content, "```python") {
					inCodeBlock = true
					codeBuf.Reset()
					// Emit everything after ```python
					after := content[strings.Index(content, "```python")+10:]
					if after != "" {
						runtime.EventsEmit(a.ctx, "codegen-delta", after)
					}
				} else if inCodeBlock {
					// Check if we hit the closing ```
					if strings.Contains(e.Delta, "```") {
						before := e.Delta[:strings.Index(e.Delta, "```")]
						if before != "" {
							runtime.EventsEmit(a.ctx, "codegen-delta", before)
						}
						inCodeBlock = false
					} else {
						runtime.EventsEmit(a.ctx, "codegen-delta", e.Delta)
					}
				}
			}
			return nil
		})

		if forEachErr != nil {
			runtime.EventsEmit(a.ctx, "codegen-error", fmt.Sprintf("error: %v", forEachErr))
			runtime.EventsEmit(a.ctx, "codegen-done", "")
			return
		}

		generatedCode := cleanCode(fullText.String())
		if generatedCode != "" {
			a.saveConversation(req.Prompt, generatedCode, req.ConvID)
		}
		runtime.EventsEmit(a.ctx, "codegen-done", generatedCode)
	})

	return nil
}

// sysPrompt returns the system prompt for matplotlib code generation.
func sysPrompt() string {
	lines := []string{
		"你是一个 Python 数据可视化专家。根据用户需求生成完整的 Python matplotlib 代码。",
		"",
		"【要求】",
		"1. 使用 matplotlib + PyQt5 后端",
		"2. 代码开头：",
		"   import matplotlib",
		"   matplotlib.use('Qt5Agg')",
		"3. 包含 import matplotlib.pyplot as plt",
		"4. 图表必须有标题、x轴标签、y轴标签",
		"5. 使用 plt.show() 显示图表",
		"6. 如果用户没有指定具体数据，使用合理的示例数据",
		"7. 如果图表包含中文，必须设置：",
		"   plt.rcParams['font.sans-serif'] = ['SimHei']",
		"   plt.rcParams['axes.unicode_minus'] = False",
		"8. 配色必须美观专业：",
		"   - 使用 plt.style.use('seaborn-v0_8') 或类似现代样式",
		"   - 或手动指定美观调色板",
		"   - 折线图线条加粗(线宽2-3)，添加标记点",
		"   - 散点图使用带透明度的颜色",
		"9. 只返回 ```python 代码块，不要包含任何解释说明",
		"10. 代码必须可以独立运行",
	}
	return strings.Join(lines, "\n") + "\n"
}

// buildLLMContext builds the message array with system prompt inlined into
// the first user message, followed by conversation history and the current
// prompt. This ensures the system instructions reach all providers (some
// drop SystemMessage during conversion).
func buildLLMContext(req CodeGenRequest) core.Context {
	sp := sysPrompt()

	var msgs []core.Message

	if len(req.Messages) == 0 {
		// First turn: inline system prompt + user prompt
		msgs = append(msgs, core.UserMessage{Content: sp + "\n用户需求：\n" + req.Prompt})
	} else {
		// Subsequent turns: start with system prompt alone (first user message
		// from history will follow)
		msgs = append(msgs, core.UserMessage{Content: sp})

		// Add all conversation history as-is
		for _, m := range req.Messages {
			switch m.Role {
			case "user":
				msgs = append(msgs, core.UserMessage{Content: m.Content})
			case "assistant":
				msgs = append(msgs, core.AssistantMessage{
					Content: []core.ContentBlock{core.TextContent{Type: "text", Text: m.Content}},
				})
			}
		}

		// Append the current prompt as the new user message
		userContent := req.Prompt
		if req.CurrentCode != "" {
			userContent += fmt.Sprintf("\n\n当前已生成的代码（请基于此修改）：\n%s", req.CurrentCode)
		}
		msgs = append(msgs, core.UserMessage{Content: userContent})
	}

	return core.Context{Messages: msgs}
}

// resolveAPIKey resolves the API key from request or settings.
func (a *App) resolveAPIKey(req CodeGenRequest, model core.Model) string {
	if req.APIKey != "" {
		return req.APIKey
	}
	cp := a.settings.Current()
	if cp != nil && cp.APIKey != "" {
		return cp.APIKey
	}
	return core.ResolveAPIKey(model.Provider, "")
}

// cleanCode attempts to extract just the Python code from LLM output.
func cleanCode(code string) string {
	code = strings.TrimSpace(code)

	// Remove markdown code fences
	if strings.HasPrefix(code, "```python") {
		code = strings.TrimPrefix(code, "```python")
	} else if strings.HasPrefix(code, "```") {
		code = strings.TrimPrefix(code, "```")
	}
	if strings.HasSuffix(code, "```") {
		code = strings.TrimSuffix(code, "```")
	}

	code = strings.TrimSpace(code)
	return code
}

// saveConversation persists a code generation result as a conversation.
// If convID is non-empty and the conversation already exists, it appends
// the new messages instead of overwriting.
func (a *App) saveConversation(prompt, code, convID string) {
	a.convMu.Lock()
	defer a.convMu.Unlock()

	id := convID
	if id == "" {
		id = fmt.Sprintf("conv_%d", makeTimestamp())
	} else {
		// Check if conversation already exists
		existing, err := a.GetConversation(id)
		if err == nil && existing != "" && existing != "null" {
			var existingConv Conversation
			if json.Unmarshal([]byte(existing), &existingConv) == nil {
				// Append new messages
				existingConv.Messages = append(existingConv.Messages,
					Message{Role: "user", Content: prompt},
					Message{Role: "assistant", Content: code},
				)
				existingConv.Code = code
				existingConv.Prompt = prompt
				existingConv.Timestamp = formatTimestamp()
				data, _ := json.Marshal(existingConv)
				if err := a.SaveConversation(id, string(data)); err != nil {
					LogError("[conversation] save error: %v", err)
				}
				return
			}
		}
	}

	title := prompt
	if idx := strings.Index(prompt, "\n"); idx > 0 {
		title = prompt[:idx]
	}
	if len(title) > 60 {
		title = title[:60] + "..."
	}

	conv := Conversation{
		ID:        id,
		Title:     title,
		Prompt:    prompt,
		Code:      code,
		Timestamp: formatTimestamp(),
		Messages: []Message{
			{Role: "user", Content: prompt},
			{Role: "assistant", Content: code},
		},
	}

	data, err := json.Marshal(conv)
	if err != nil {
		LogError("[conversation] marshal error: %v", err)
		return
	}

	if err := a.SaveConversation(id, string(data)); err != nil {
		LogError("[conversation] save error: %v", err)
	}
}

// makeTimestamp returns a Unix timestamp in milliseconds.
func makeTimestamp() int64 {
	return time.Now().UnixMilli()
}

// formatTimestamp returns a human-readable timestamp.
func formatTimestamp() string {
	return fmt.Sprintf("%s", time.Now().Format("2006-01-02 15:04:05"))
}
