package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pi-ai-go/agent/session"
	"pi-ai-go/core"
	"pi-ai-go/llm"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// providerFromRequest resolves the provider, API key, base URL, model from a request
// and falls back to app settings if the request fields are empty.
func (a *App) providerFromRequest(provider, apiKey, baseURL, modelID string) (core.Model, string, core.SimpleStreamOptions) {
	cp := a.settings.Current()
	if provider == "" && cp != nil {
		provider = cp.Type
	}
	if apiKey == "" && cp != nil {
		apiKey = cp.APIKey
	}
	if apiKey == "" {
		apiKey = core.ResolveAPIKey(core.ProviderOpenAI, "")
	}
	if baseURL == "" && cp != nil {
		baseURL = cp.BaseURL
	}
	if modelID == "" {
		modelID = a.settings.Model
	}

	model := a.resolveModel(provider, modelID, baseURL)
	opts := core.SimpleStreamOptions{
		StreamOptions: core.StreamOptions{
			APIKey: apiKey,
		},
	}
	return model, provider, opts
}

// streamLLM performs the common streaming LLM call pattern:
//  1. Creates a cancellable context
//  2. Calls llm.StreamSimpleWithContext
//  3. Emits "geogebra-text-delta" for each text chunk
//  4. On completion emits "geogebra-done" with parsed result JSON
//  5. On error emits "geogebra-error" and a zero-result "geogebra-done"
//
// Returns an error only if the initial stream creation fails.
func (a *App) streamLLM(model core.Model, llmCtx core.Context, opts core.SimpleStreamOptions, streamID string) error {
	streamCtx, cancelFn := context.WithCancel(a.ctx)
	a.cancelFn = cancelFn

	stream, err := llm.StreamSimpleWithContext(streamCtx, model, llmCtx, opts)
	if err != nil {
		LogError("[%s] StreamSimple error: %v", streamID, err)
		runtime.EventsEmit(a.ctx, "geogebra-error", fmt.Sprintf("Error: %v", err))
		return err
	}

	LogInfo("[%s] stream started", streamID)

	go func() {
		defer func() {
			a.cancelFn = nil
		}()
		textLen := 0
		var fullContent strings.Builder

		_, forEachErr := stream.ForEach(streamCtx, func(event core.AssistantMessageEvent) error {
			switch e := event.(type) {
			case core.EventTextDelta:
				textLen += len(e.Delta)
				fullContent.WriteString(e.Delta)
				runtime.EventsEmit(a.ctx, "geogebra-text-delta", e.Delta)
			case core.EventDone:
				LogInfo("[%s] done, total text length=%d", streamID, textLen)
				result := extractGeogebraResult(fullContent.String())
				resultJSON, _ := json.Marshal(result)
				runtime.EventsEmit(a.ctx, "geogebra-done", string(resultJSON))
				return nil
			}
			return nil
		})

		if forEachErr != nil {
			LogError("[%s] ForEach error: %v (textLen=%d)", streamID, forEachErr, textLen)
			runtime.EventsEmit(a.ctx, "geogebra-error", fmt.Sprintf("Error: %v", forEachErr))
			runtime.EventsEmit(a.ctx, "geogebra-done", `{"text":"","ggbCode":"","html":""}`)
		}
	}()

	return nil
}

// GeogebraMessage handles a streaming request to generate GeoGebra
// commands and embeddable HTML. It:
//  1. Builds a specialized system prompt for GeoGebra command generation
//  2. Calls the LLM with streaming
//  3. Emits text deltas to the frontend
//  4. On completion, emits a "geogebra-done" event with GeoGebra XML/HTML
func (a *App) GeogebraMessage(jsonStr string) error {
	var req GeogebraRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		LogError("[geogebra] parse error: %v", err)
		runtime.EventsEmit(a.ctx, "geogebra-error", fmt.Sprintf("parse error: %v", err))
		return err
	}

	LogInfo("[geogebra] request: message=%q", req.Message)

	model, _, opts := a.providerFromRequest(req.Provider, req.APIKey, req.BaseURL, req.Model)

	if req.MaxTokens > 0 {
		t := req.MaxTokens
		opts.MaxTokens = &t
	}
	if req.Temperature > 0 {
		t := req.Temperature
		opts.Temperature = &t
	}

	systemPrompt := a.buildGeogebraSystemPrompt()

	// Add perspective context to system prompt
	if req.Perspective != "" {
		viewNames := map[string]string{
			"1": "2D 函数/代数视图",
			"2": "2D 平面几何视图",
			"5": "3D 立体几何视图",
		}
		if name, ok := viewNames[req.Perspective]; ok {
			systemPrompt += "\n\n用户当前处于 " + name + "，生成的 GeoGebra 命令应使用对应的坐标系和命令。"
		}
	}

	// Build messages: history + current user message (system prompt goes
	// into core.Context.SystemPrompt, not the messages array, so all
	// providers — including Anthropic — receive it correctly).
	var messages []core.Message
	for _, h := range req.HistoryMessages {
		switch h.Role {
		case "user":
			messages = append(messages, core.UserMessage{Content: h.Content})
		case "assistant":
			messages = append(messages, core.AssistantMessage{Content: []core.ContentBlock{core.TextContent{Text: h.Content}}})
		}
	}
	messages = append(messages, core.UserMessage{Content: req.Message})

	llmCtx := core.Context{
		SystemPrompt: systemPrompt,
		Messages:     messages,
	}

	return a.streamLLM(model, llmCtx, opts, "geogebra")
}

// GeogebraValidateAndRegenerate validates GGB code and if there are errors,
// sends them back to the LLM to regenerate a corrected version.
func (a *App) GeogebraValidateAndRegenerate(jsonStr string) error {
	var req GeogebraRegenRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		LogError("[geogebra-regen] parse error: %v", err)
		runtime.EventsEmit(a.ctx, "geogebra-error", fmt.Sprintf("parse error: %v", err))
		return err
	}

	LogInfo("[geogebra-regen] original=%q, errors=%q", req.OriginalMessage, req.ValidationErrors)

	model, _, opts := a.providerFromRequest(req.Provider, req.APIKey, req.BaseURL, req.Model)

	if req.MaxTokens > 0 {
		t := req.MaxTokens
		opts.MaxTokens = &t
	}
	if req.Temperature > 0 {
		t := req.Temperature
		opts.Temperature = &t
	}

	systemPrompt := a.buildGeogebraSystemPrompt()

	// Build a correction prompt that includes the original request, generated code, and validation errors
	fixPrompt := "我需要你修正之前生成的 GeoGebra 命令。\n\n"
	fixPrompt += "## 原始需求\n%s\n\n"
	fixPrompt += "## 之前生成的 GeoGebra 命令\n%s\n\n"
	fixPrompt += "## 校验错误\n%s\n\n"
	fixPrompt += "请根据以上校验错误修正 GeoGebra 命令，重新输出修正后的版本。\n"
	fixPrompt += "注意：\n"
	fixPrompt += "1. 修正所有报告的语法错误和未知命令\n"
	fixPrompt += "2. 确保命令名称拼写正确\n"
	fixPrompt += "3. 确保参数数量和类型正确\n"
	fixPrompt += "4. 必须返回完整的 GeoGebra 指令，不要只输出修改的部分\n"
	fixPrompt += "5. 一行一条指令，不要将多条命令写在同一行\n"
	fixPrompt += "6. 代码块中不包含任何注释（不要出现 // 或 # 开头的注释行），只输出纯命令\n"
	fixPrompt += "7. 输出格式与之前相同：```geogebra 代码块放命令，```html 代码块放 HTML"
	correctionPrompt := fmt.Sprintf(fixPrompt, req.OriginalMessage, req.GgbCode, req.ValidationErrors)

	messages := []core.Message{
		core.UserMessage{Content: correctionPrompt},
	}

	llmCtx := core.Context{
		SystemPrompt: systemPrompt,
		Messages:     messages,
	}

	return a.streamLLM(model, llmCtx, opts, "geogebra-regen")
}

// GeogebraResult holds the parsed result from the LLM.
type GeogebraResult struct {
	Text    string `json:"text"`
	GgbCode string `json:"ggbCode"`
	HTML    string `json:"html"`
	SVG     string `json:"svg"`
}

// extractGeogebraResult parses the LLM response, extracting GeoGebra
// commands, HTML content, and SVG from code blocks.
func extractGeogebraResult(fullText string) GeogebraResult {
	result := GeogebraResult{
		Text: fullText,
	}

	lines := strings.Split(fullText, "\n")
	var inGgbBlock, inHTMLBlock, inSVGBlock bool
	var ggbLines, htmlLines, svgLines []string
	var textLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```geogebra") || strings.HasPrefix(trimmed, "```ggb") {
			inGgbBlock = true
			continue
		}
		if strings.HasPrefix(trimmed, "```html") {
			inHTMLBlock = true
			continue
		}
		if strings.HasPrefix(trimmed, "```svg") {
			inSVGBlock = true
			continue
		}
		if trimmed == "```" {
			if inGgbBlock {
				inGgbBlock = false
				continue
			}
			if inHTMLBlock {
				inHTMLBlock = false
				continue
			}
			if inSVGBlock {
				inSVGBlock = false
				continue
			}
		}

		switch {
		case inGgbBlock:
			ggbLines = append(ggbLines, line)
		case inHTMLBlock:
			htmlLines = append(htmlLines, line)
		case inSVGBlock:
			svgLines = append(svgLines, line)
		default:
			textLines = append(textLines, line)
		}
	}

	if len(ggbLines) > 0 {
		result.GgbCode = strings.Join(ggbLines, "\n")
	}
	if len(htmlLines) > 0 {
		result.HTML = strings.Join(htmlLines, "\n")
	}
	if len(svgLines) > 0 {
		result.SVG = strings.Join(svgLines, "\n")
	}
	if len(textLines) > 0 && result.GgbCode == "" && result.HTML == "" {
		result.Text = strings.Join(textLines, "\n")
		// If there's only one code block without a label, try to detect it
		result.GgbCode = detectGeogebraCommands(fullText)
	}

	return result
}

// detectGeogebraCommands tries to find GeoGebra commands in plain text.
func detectGeogebraCommands(text string) string {
	var commands []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// GeoGebra commands typically start with a command name like
		// "Polygon", "Line", "Circle", "Segment", etc.
		geogebraKeywords := []string{
			"Polygon(", "Line(", "Circle(", "Segment(", "Point(", "Midpoint(",
			"Intersect(", "Angle(", "Distance(", "Area(", "Perimeter(",
			"Function(", "Curve(", "Surface(", "Sequence(", "Sum(",
			"If(", "Slider(", "Text(", "ShowLabel(", "SetColor(",
			"SetDynamicColor(", "SetLineStyle(", "SetLineThickness(",
			"A=(", "B=(", "C=(", "D=(", "E=(", "F=(", "O=(", "M=(", "N=(",
			"Polyline(", "Ray(", "Tangent(", "AngularBisector(",
			"PerpendicularLine(", "ParallelLine(", "Reflect(", "Rotate(",
			"Translate(", "Dilate(", "Expand(", "Factor(", "Simplify(",
		}
		for _, kw := range geogebraKeywords {
			if strings.Contains(trimmed, kw) {
				commands = append(commands, trimmed)
				break
			}
		}
	}
	return strings.Join(commands, "\n")
}

// buildGeogebraSystemPrompt loads skills from the skills/ directory and
// builds the system prompt using session.BuildSystemPrompt. Skills are
// loaded once and cached for subsequent calls.
func (a *App) buildGeogebraSystemPrompt() string {
	a.skillsOnce.Do(func() {
		skillsDir := filepath.Join("skills")
		if abs, err := filepath.Abs(skillsDir); err == nil {
			skillsDir = abs
		}
		if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
			LogWarn("[skills] directory not found: %s", skillsDir)
			return
		}
		skills, diags := session.LoadSkills(skillsDir)
		for _, d := range diags {
			LogWarn("[skills] %s: %s", d.Path, d.Message)
		}
		if len(skills) == 0 {
			LogWarn("[skills] no skills loaded from %s", skillsDir)
			return
		}
		LogInfo("[skills] loaded %d skill(s) from %s", len(skills), skillsDir)
		a.cachedSkills = skills
	})

	return session.BuildSystemPrompt(session.SystemPromptConfig{
		BasePrompt: "你是 GeoGebra 指令生成专家。你的任务是根据用户的描述，只生成 GeoGebra 命令和 GeoGebra 网页版课件。",
		Skills:     a.cachedSkills,
	})
}
