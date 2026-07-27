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

	prompt := buildCodeGenPrompt(req)

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

	msg, err := llm.CompleteSimple(streamCtx, model, []core.Message{
		core.UserMessage{Content: prompt},
	}, opts)
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

	prompt := buildCodeGenPrompt(req)

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

	stream, err := llm.StreamSimple(streamCtx, model, []core.Message{
		core.UserMessage{Content: prompt},
	}, opts)
	if err != nil {
		runtime.EventsEmit(a.ctx, "codegen-error", fmt.Sprintf("stream error: %v", err))
		return err
	}

	var fullText strings.Builder
	inCodeBlock := false
	codeBuf := strings.Builder{}

	go func() {
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
	}()

	return nil
}

// buildCodeGenPrompt constructs the system prompt for code generation.
func buildCodeGenPrompt(req CodeGenRequest) string {
	var sb strings.Builder
	sb.WriteString("你是一个 Python 数据可视化专家。根据用户的需求，生成完整的 Python 代码。\n")
	sb.WriteString("\n")
	sb.WriteString("【要求】\n")
	sb.WriteString("1. 使用 matplotlib 绘制图表，必须使用 PyQt5 后端\n")
	sb.WriteString("2. 代码开头添加以下两行：\n")
	sb.WriteString("   import matplotlib\n")
	sb.WriteString("   matplotlib.use('Qt5Agg')\n")
	sb.WriteString("3. 必须包含 import matplotlib.pyplot as plt\n")
	sb.WriteString("4. 图表必须有标题、x轴标签、y轴标签\n")
	sb.WriteString("5. 使用 plt.show() 显示图表\n")
	sb.WriteString("6. 如果用户没有指定具体数据，使用合理的示例数据\n")
	sb.WriteString("7. 只返回 Python 代码（放在 ```python 代码块中），不要包含任何解释说明\n")
	sb.WriteString("8. 代码必须可以独立运行（包含所有必要的 import）\n")
	sb.WriteString("\n")
	sb.WriteString("用户需求：\n")
	sb.WriteString(req.Prompt)

	if req.CurrentCode != "" {
		sb.WriteString(fmt.Sprintf("\n\n当前已生成的代码（请基于此修改）：\n%s", req.CurrentCode))
	}

	return sb.String()
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
func (a *App) saveConversation(prompt, code, convID string) {
	id := convID
	if id == "" {
		// Generate an ID if not provided by frontend
		id = fmt.Sprintf("conv_%d", makeTimestamp())
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
