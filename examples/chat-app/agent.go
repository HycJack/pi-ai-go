package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pi-ai-go/agent"
	"pi-ai-go/agent/session"
	agenttools "pi-ai-go/agent/tools"
	"pi-ai-go/core"
	"pi-ai-go/llm"

	"chat-app/autolearn"
	"chat-app/contextmgr"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AgentMessage handles an agent-mode streaming chat request. It builds
// the message history, loads skills, runs the agent loop, and emits
// events to the frontend.
func (a *App) AgentMessage(jsonStr string) error {
	var req AgentRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		LogError("[agent] parse error: %v", err)
		runtime.EventsEmit(a.ctx, "agent-error", fmt.Sprintf("parse error: %v", err))
		return err
	}

	LogInfo("[agent] request: provider=%s model=%s msgLen=%d history=%d images=%d",
		req.Provider, req.Model, len(req.Message), len(req.Messages), len(req.Images))

	model := a.resolveModel(req.Provider, req.Model, req.BaseURL)
	apiKey := req.APIKey
	if apiKey == "" {
		apiKey = a.selectAPIKey()
	}
	if apiKey == "" {
		apiKey = core.ResolveAPIKey(model.Provider, "")
	}

	systemPrompt := a.buildSystemPrompt()

	// Load skills from ~/.agent/skills/ and the configured SkillsDir.
	var skills []session.Skill
	var skillDirs []string
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		skillDirs = append(skillDirs, filepath.Join(homeDir, ".agent", "skills"))
	}
	skillsDir := a.settings.SkillsDir
	if skillsDir == "" {
		skillsDir = filepath.Join(a.dataDir, "skills")
	}
	if skillsDir != "" {
		skillDirs = append(skillDirs, skillsDir)
	}
	for _, dir := range skillDirs {
		if loaded, diags := session.LoadSkills(dir); len(loaded) > 0 {
			skills = append(skills, loaded...)
			for _, d := range diags {
				LogInfo("[skill] %s: %s", d.Path, d.Message)
			}
			LogInfo("[skills] loaded %d skill(s) from %s", len(loaded), dir)
		}
	}

	config := agent.AgentLoopConfig{
		Model:        model,
		SystemPrompt: systemPrompt,
		Tools:        agenttools.All(),
		Skills:       skills,
		ExecEnv:      a.makeExecEnv(),
		SimpleStreamOptions: core.SimpleStreamOptions{
			StreamOptions: core.StreamOptions{
				APIKey: apiKey,
			},
		},
	}
	if req.MaxTokens > 0 {
		t := req.MaxTokens
		config.SimpleStreamOptions.MaxTokens = &t
	}
	if req.Temperature > 0 {
		config.SimpleStreamOptions.Temperature = &req.Temperature
	}
	if req.Reasoning != "" {
		config.SimpleStreamOptions.Reasoning = core.ThinkingLevel(req.Reasoning)
	}

	messages := a.buildAgentMessages(req)

	// autolearn — process input extraction if enabled.
	if a.mem != nil && a.settings.AutoLearn {
		learned := a.autoLearn().ProcessUserInput(req.Message)
		if learned > 0 {
			config.SystemPrompt = a.buildSystemPrompt()
			_ = a.mem.Save()
		}
	}

	streamCtx, cancelFn := context.WithCancel(a.ctx)
	a.cancelFn = cancelFn

	// Initialize per-conversation token stats.
	convID := req.ConversationID
	if convID == "" {
		convID = "_default"
	}
	a.currentConvID = convID
	tokenStats := a.getOrCreateTokenStats(convID, req.Model)

	eventStream, detailed := agent.AgentLoopDetailed(streamCtx, messages, config)

	LogInfo("[agent] loop started, messages=%d tools=%d skills=%d",
		len(messages), len(config.Tools), len(config.Skills))

	go func() {
		textLen := 0
		eventStream.ForEach(streamCtx, func(evt agent.AgentEvent) error {
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
			case agent.EventAgentEnd:
				// Agent 完成
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
			}
			return nil
		})

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

		// Extract memory if AutoLearn is enabled.
		if a.mem != nil && a.settings.AutoLearn && len(result.Messages) > 0 {
			_ = a.autoLearn().MaybeExtract(streamCtx, result.Messages, a.newLLMExtractor(model, apiKey))
			_ = a.mem.Save()
			tokenStats.Recompute(result.Messages)
		}

		// Context compaction.
		if a.settings.AutoCompact && tokenStats.ShouldCompact() {
			compactCtx, compactCancel := context.WithTimeout(context.Background(), 30*time.Second)
			streamOpts := []core.SimpleStreamOptions{
				{StreamOptions: core.StreamOptions{APIKey: apiKey}},
			}
			if cr, err := contextmgr.Compact(compactCtx, model, result.Messages, a.ctxSettings, streamOpts...); err == nil {
				LogInfo("[compact] saved %d tokens in %v", cr.TokensSaved, cr.Duration)
				for _, msg := range cr.NewMessages {
					result.Messages = append(result.Messages, msg)
				}
				tokenStats.Recompute(result.Messages)
			} else {
				LogWarn("[compact] skipped: %v", err)
			}
			compactCancel()
		}

		runtime.EventsEmit(a.ctx, "agent-done", "")
	}()

	return nil
}

// resolveModel maps a provider string + model ID to a core.Model, using
// a fallback if the model is not in the registry.
func (a *App) resolveModel(providerStr, modelID, baseURL string) core.Model {
	var provider core.KnownProvider
	var api core.KnownAPI
	if providerStr == "anthropic" {
		provider = core.ProviderAnthropic
		api = core.APIAnthropicMessages
	} else {
		provider = core.ProviderOpenAI
		api = core.APIOpenAICompletions
	}
	if modelID == "" {
		modelID = "auto"
	}
	model, err := llm.GetModel(provider, modelID)
	if err != nil {
		LogWarn("[model] %v, using fallback model %s/%s", err, provider, modelID)
		model = core.Model{
			ID:            modelID,
			Provider:      provider,
			API:           api,
			ContextWindow: 8192,
		}
	}
	if baseURL != "" {
		model.BaseURL = baseURL
	}
	LogDebug("[model] resolved: id=%s provider=%s api=%s baseURL=%s ctxWindow=%d",
		model.ID, model.Provider, model.API, model.BaseURL, model.ContextWindow)
	return model
}

// buildSystemPrompt constructs the system prompt for the agent, including
// tool instructions and memory entries.
func (a *App) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("你是一个有帮助的 AI 助手，可以访问文件系统工具。\n")
	sb.WriteString("你可以读取文件、写入文件、追加写入文件、编辑文件、列出目录、执行命令和搜索内容。\n")
	sb.WriteString("当用户提出涉及文件操作的任务时，请使用可用的工具。\n")
	sb.WriteString("在执行工具之前，请清楚地解释你的操作。\n")
	if a.settings.WorkingDir != "" {
		sb.WriteString(fmt.Sprintf("\n当前工作目录是：%s\n", a.settings.WorkingDir))
	}
	sb.WriteString("\n## 输出规范\n")
	sb.WriteString("- 尽量不要在回复中使用 emoji（表情符号）\n")
	sb.WriteString("- 使用简洁、专业的语言回复\n")
	sb.WriteString("- 代码块使用正确的 markdown 格式\n")
	sb.WriteString("\n## 生成大文件的策略\n")
	sb.WriteString("当你需要生成一个**完整文件**（HTML 页面、长文档、大段代码）且预估内容可能超过单次输出上限时：\n")
	sb.WriteString("1. 先规划文件结构（开头/正文/结尾），把内容切成几个明确的部分\n")
	sb.WriteString("2. 第一段使用 `write_file` 写入（创建文件）\n")
	sb.WriteString("3. 后续每一段使用 `append_file` 追加到文件末尾\n")
	sb.WriteString("4. 每一段控制在合理大小（例如几百行代码），避免单次工具调用过大导致失败\n")
	sb.WriteString("5. 如果上一次写入失败或被截断，使用 `read_file` 查看文件当前末尾内容，再用 `append_file` 从断点继续\n")
	sb.WriteString("6. 写完后用 `read_file` 或 `bash` 简单校验文件是否完整\n")
	if a.mem != nil && a.mem.Size() > 0 {
		memText := a.mem.FormatForPrompt()
		if memText != "" {
			sb.WriteString("\n---\n")
			sb.WriteString(memText)
			sb.WriteString("\n---\n")
		}
	}
	return sb.String()
}

// autoLearn returns an AutoLearner configured with the current settings.
func (a *App) autoLearn() *autolearn.AutoLearner {
	settings := a.settings
	skillsDir := settings.SkillsDir
	if skillsDir == "" {
		skillsDir = filepath.Join(a.dataDir, "skills")
	}
	al := autolearn.New(a.mem, autolearn.Settings{
		AutoLearn:     settings.AutoLearn,
		ExtractEveryN: 3,
		MinConfidence: 0.5,
	})
	al.WorkflowDir = filepath.Join(skillsDir, "auto-extracted")
	al.LLMExtract = a.newInputLLMExtractor
	return al
}

// newLLMExtractor creates an LLMSimpleExtractor for memory extraction.
func (a *App) newLLMExtractor(model core.Model, apiKey string) *autolearn.LLMSimpleExtractor {
	return &autolearn.LLMSimpleExtractor{
		SummarizeFunc: func(ctx context.Context, prompt string) (string, error) {
			msg, err := llm.CompleteSimple(ctx, model, []core.Message{
				core.UserMessage{Content: prompt},
			}, core.SimpleStreamOptions{
				StreamOptions: core.StreamOptions{APIKey: apiKey},
			})
			if err != nil {
				return "", err
			}
			var text strings.Builder
			for _, b := range msg.Content {
				if c, ok := b.(core.TextContent); ok {
					text.WriteString(c.Text)
				}
			}
			return text.String(), nil
		},
	}
}

// newInputLLMExtractor creates an LLM extractor that parses a single user
// input into KEY=VALUE memory pairs.
func (a *App) newInputLLMExtractor(ctx context.Context, text string) (map[string]string, error) {
	if text == "" {
		return nil, nil
	}
	settings := a.settings
	modelID := settings.Model
	if modelID == "" {
		modelID = "gpt-4o-mini"
	}
	cp := settings.Current()
	var apiKey, providerType, baseURL string
	if cp != nil {
		apiKey = cp.APIKey
		providerType = cp.Type
		baseURL = cp.BaseURL
	}
	if apiKey == "" {
		apiKey = core.ResolveAPIKey(core.ProviderOpenAI, "")
	}
	model := a.resolveModel(providerType, modelID, baseURL)

	prompt := fmt.Sprintf(`你是记忆提取助手。从用户的输入中提取需要**长期记住**的事实。
【规则】
- 只提取明确的、可验证的事实（名字、偏好、身份信息、项目名等）
- 每条输出一行 KEY=VALUE
- 允许的 KEY 前缀: user. , project. , fact.
- 没有值得记的，只输出 NONE
- 不要编造，不确定就不提取

用户说: %s`, text)

	msg, err := llm.CompleteSimple(ctx, model, []core.Message{
		core.UserMessage{Content: prompt},
	}, core.SimpleStreamOptions{
		StreamOptions: core.StreamOptions{APIKey: apiKey},
	})
	if err != nil {
		return nil, err
	}
	var response strings.Builder
	for _, b := range msg.Content {
		if c, ok := b.(core.TextContent); ok {
			response.WriteString(c.Text)
		}
	}

	result := make(map[string]string)
	for _, line := range strings.Split(response.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "NONE" {
			continue
		}
		key, value, found := splitKV(line)
		if found && key != "" && value != "" {
			result[key] = value
		}
	}
	return result, nil
}

// splitKV splits a line on the first = or : separator.
func splitKV(line string) (key, value string, found bool) {
	for _, sep := range []string{"=", ":", "："} {
		idx := strings.Index(line, sep)
		if idx > 0 {
			k := strings.TrimSpace(line[:idx])
			v := strings.Trim(strings.TrimSpace(line[idx+len(sep):]), "\"'「」『』")
			if k != "" && v != "" {
				return k, v, true
			}
		}
	}
	return "", "", false
}

// makeExecEnv creates an ExecutionEnv rooted at the configured working
// directory, or uses the default process CWD if none is set.
func (a *App) makeExecEnv() core.ExecutionEnv {
	wd := a.settings.WorkingDir
	if wd != "" {
		return core.NewDefaultExecutionEnvWithDir(wd)
	}
	return core.NewDefaultExecutionEnv()
}

// ListDirectory returns a JSON array of file/directory entries inside dir.
func (a *App) ListDirectory(dir string) (string, error) {
	if dir == "" {
		dir = a.settings.WorkingDir
		if dir == "" {
			wd, err := os.Getwd()
			if err != nil {
				return "[]", err
			}
			dir = wd
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "[]", err
	}
	type entry struct {
		Name  string `json:"name"`
		IsDir bool   `json:"isDir"`
		Size  int64  `json:"size,omitempty"`
	}
	out := make([]entry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		sz := int64(0)
		if err == nil {
			sz = info.Size()
		}
		out = append(out, entry{Name: e.Name(), IsDir: e.IsDir(), Size: sz})
	}
	data, _ := json.Marshal(out)
	return string(data), nil
}

// ReadTextFile reads a text file's content and returns it as a string.
func (a *App) ReadTextFile(path string) (string, error) {
	wd := a.settings.WorkingDir
	if wd == "" {
		wd, _ = os.Getwd()
	}
	if wd != "" {
		rel, err := filepath.Rel(wd, path)
		if err != nil {
			return "", fmt.Errorf("access denied: cannot resolve path relative to working directory: %w", err)
		}
		if len(rel) >= 2 && rel[:2] == ".." {
			return "", fmt.Errorf("access denied: path is outside working directory")
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	const maxBytes = 1 * 1024 * 1024
	lr := io.LimitReader(f, maxBytes+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return "", err
	}
	truncated := len(data) > maxBytes
	if truncated {
		data = data[:maxBytes]
	}
	result := string(data)
	if truncated {
		result += "\n\n... (truncated, file > 1MB)"
	}
	return result, nil
}
