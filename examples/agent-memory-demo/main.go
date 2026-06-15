// Demo: Agent 上下文管理与记忆存储
//
// 展示功能:
//   1. 会话管理 (MemoryStorage / JSONLStorage)
//   2. 多轮对话与上下文保持
//   3. 上下文压缩 (Compaction)
//   4. 分支摘要 (Branch Summary)
//   5. 自定义消息与会话条目
//   6. 从会话历史重建上下文
//
// Usage:
//
//	go run . [-persist session.jsonl]
package main

import (
	"bufio"
	"context"
	
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"pi-ai-go/agent"
	"pi-ai-go/agent/session"
	"pi-ai-go/agent/tools"
	"pi-ai-go/core"
	"pi-ai-go/llm"
	_ "pi-ai-go/providers"
)

func main() {
	loadEnv()

	persistFile := flag.String("persist", "", "JSONL file for session persistence (empty = in-memory only)")
	modelID := flag.String("model", envOr("MODEL", "deepseek-chat"), "Model ID")
	provider := flag.String("provider", envOr("PROVIDER", "deepseek"), "Provider name")
	apiKeyFlag := flag.String("api-key", "", "API key")
	flag.Parse()

	apiKey := *apiKeyFlag
	if apiKey == "" {
		apiKey = os.Getenv("API_KEY")
		if apiKey == "" && *provider != "" {
			apiKey = os.Getenv(strings.ToUpper(*provider) + "_API_KEY")
		}
	}
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: API key not configured")
		os.Exit(1)
	}

	model := resolveModel(*provider, *modelID)

	// ──────────────────────────────────────────────
	// 1. 创建会话存储
	// ──────────────────────────────────────────────
	var storage session.SessionStorage
	if *persistFile != "" {
		s, err := session.NewJSONLStorage(*persistFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer s.Close()
		storage = s
		fmt.Fprintf(os.Stderr, "[session] 持久化存储: %s\n", *persistFile)
	} else {
		storage = session.NewMemoryStorage()
		fmt.Fprintf(os.Stderr, "[session] 内存存储 (重启后丢失)\n")
	}

	// ──────────────────────────────────────────────
	// 2. 创建会话
	// ──────────────────────────────────────────────
	sess, err := session.NewSession(storage)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer sess.Close()

	// 写入会话元信息
	_ = sess.Append(session.SessionTreeEntry{
		ID:        session.GenerateID(),
		Type:      session.EntrySessionInfo,
		Timestamp: time.Now(),
		SessionID: "demo-" + time.Now().Format("20060102-150405"),
		Description: "Agent 上下文管理演示",
	})
	fmt.Fprintf(os.Stderr, "[session] 会话 ID: %s\n", sess.ID())

	// ──────────────────────────────────────────────
	// 3. 配置 Agent
	// ──────────────────────────────────────────────
	systemPrompt := `你是一个有帮助的编程助手。你可以使用文件系统工具来读写文件。
请用中文回答用户的问题。记住之前的对话内容，保持上下文连贯。`

	config := agent.AgentLoopConfig{
		Model:        model,
		SystemPrompt: systemPrompt,
		Tools:        tools.All(),
		// 上下文压缩策略：超过 80% 时触发滑动窗口压缩
		ContextPolicy: &agent.ContextPolicy{
			SoftLimit:       0.80,
			HardLimit:       0.95,
			ReservedOutput:  2048,
			MinTailMessages: 6,
			Strategy:        agent.CompactionStrategySlidingWindow,
		},
		OnCompaction: func(e agent.CompactionEvent) {
			fmt.Fprintf(os.Stderr, "\n[compaction] 策略=%s 触发=%s 丢弃=%d token %d→%d\n",
				e.Strategy, e.TriggeredBy, e.Dropped, e.TokensBefore, e.TokensAfter)
			// 记录压缩事件到会话
			_ = sess.Append(session.SessionTreeEntry{
				ID:        session.GenerateID(),
				Type:      session.EntryCompaction,
				Timestamp: time.Now(),
				CompactionSummary: fmt.Sprintf("压缩了 %d 条消息, token %d→%d", e.Dropped, e.TokensBefore, e.TokensAfter),
				TokensBefore: e.TokensBefore,
			})
		},
	}
	config.SimpleStreamOptions.APIKey = apiKey

	// ──────────────────────────────────────────────
	// 4. 交互式多轮对话
	// ──────────────────────────────────────────────
	scanner := bufio.NewScanner(os.Stdin)
	var messages []core.Message

	fmt.Fprintln(os.Stderr, "\n═══════════════════════════════════════════")
	fmt.Fprintln(os.Stderr, "  Agent 上下文管理与记忆存储 Demo")
	fmt.Fprintln(os.Stderr, "  命令: :quit :history :context :compact :branch :save :load")
	fmt.Fprintln(os.Stderr, "═══════════════════════════════════════════")

	for {
		fmt.Fprint(os.Stderr, "\n🧑 ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// ── 内置命令 ──
		switch strings.ToLower(input) {
		case ":quit", ":exit":
			fmt.Fprintln(os.Stderr, "再见!")
			return

		case ":history":
			printSessionHistory(sess)
			continue

		case ":context":
			ctx := sess.BuildContext()
			printContextInfo(ctx)
			continue

		case ":compact":
			performCompaction(context.Background(), sess, model, config)
			continue

		case ":branch":
			performBranchSummary(context.Background(), sess, model, messages)
			continue

		case ":save":
			saveSession(sess, messages)
			continue
		}

		// ── 普通对话 ──
		// 记录用户消息到会话
		userMsg := core.UserMessage{
			Role:      "user",
			Content:   input,
			Timestamp: time.Now(),
		}
		_ = sess.Append(session.SessionTreeEntry{
			ID:        session.GenerateID(),
			Type:      session.EntryMessage,
			Timestamp: time.Now(),
			Message:   userMsg,
		})
		messages = append(messages, userMsg)

		// 从会话历史重建上下文 (确保压缩后的上下文也能正确恢复)
		ctx := sess.BuildContext()
		if len(ctx.Messages) > 0 {
			messages = ctx.Messages
			// 追加当前用户消息 (BuildContext 可能不包含最新的)
			if len(messages) == 0 || !isSameMessage(messages[len(messages)-1], userMsg) {
				messages = append(messages, userMsg)
			}
		}

		// 运行 Agent
		runCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		stream, _ := agent.AgentLoopDetailed(runCtx, messages, config)

		// 处理事件流
		var lastAssistant core.AssistantMessage
		_, _ = stream.ForEach(runCtx, func(evt agent.AgentEvent) error {
			switch e := evt.(type) {
			case agent.EventMessageUpdate:
				switch ae := e.AssistantEvent.(type) {
				case core.EventTextDelta:
					fmt.Print(ae.Delta)
				case core.EventToolCallStart:
					fmt.Fprintf(os.Stderr, "\n  🔧 %s", ae.Name)
				case core.EventToolCallDelta:
					if len(ae.ArgumentsDelta) > 0 {
						fmt.Fprintf(os.Stderr, "%s", ae.ArgumentsDelta)
					}
				case core.EventToolCallEnd:
					fmt.Fprintf(os.Stderr, "\n")
				}
			case agent.EventMessageEnd:
				lastAssistant = e.Message
			case agent.EventToolExecStart:
				fmt.Fprintf(os.Stderr, "  ⚡ 执行: %s\n", e.ToolName)
			case agent.EventToolExecEnd:
				status := "✅"
				if e.IsError {
					status = "❌"
				}
				fmt.Fprintf(os.Stderr, "  %s 完成: %s\n", status, e.ToolName)
			case agent.EventCompaction:
				fmt.Fprintf(os.Stderr, "\n[压缩] %s: %d tokens → %d tokens (丢弃 %d 条)\n",
					e.Strategy, e.TokensBefore, e.TokensAfter, e.Dropped)
			}
			return nil
		})
		cancel()

		fmt.Println()

		// 记录 assistant 回复到会话
		if lastAssistant.Role == "assistant" {
			_ = sess.Append(session.SessionTreeEntry{
				ID:        session.GenerateID(),
				Type:      session.EntryMessage,
				Timestamp: time.Now(),
				Message:   lastAssistant,
			})
			messages = append(messages, lastAssistant)
		}

		// 记录工具结果到会话
		// (AgentLoopDetailed 内部已经处理了工具结果的消息追加)
		// 从 stream 结果获取完整消息历史
		result, err := stream.Result()
		if err == nil {
			messages = result
		}

		// 显示 token 使用情况
		printUsageInfo(lastAssistant.Usage)
	}
}

// ──────────────────────────────────────────────
// 辅助函数
// ──────────────────────────────────────────────

func printSessionHistory(sess *session.Session) {
	entries := sess.Entries()
	fmt.Fprintf(os.Stderr, "\n📜 会话历史 (%d 条记录):\n", len(entries))
	fmt.Fprintln(os.Stderr, "─────────────────────────────────────")
	for i, e := range entries {
		switch e.Type {
		case session.EntryMessage:
			role := "?"
			content := ""
			if e.Message != nil {
				switch m := e.Message.(type) {
				case core.UserMessage:
					role = "🧑"
					content = truncate(fmt.Sprintf("%v", m.Content), 60)
				case core.AssistantMessage:
					role = "🤖"
					for _, b := range m.Content {
						if tc, ok := b.(core.TextContent); ok {
							content = truncate(tc.Text, 60)
							break
						}
					}
				case core.ToolResultMessage:
					role = "🔧"
					content = fmt.Sprintf("[%s] %s", m.ToolName, truncate(extractToolText(m), 40))
				}
			}
			fmt.Fprintf(os.Stderr, "  %d. %s %s\n", i+1, role, content)
		case session.EntryCompaction:
			fmt.Fprintf(os.Stderr, "  %d. 📦 压缩: %s\n", i+1, e.CompactionSummary)
		case session.EntryBranchSummary:
			fmt.Fprintf(os.Stderr, "  %d. 🌿 分支摘要: %s\n", i+1, truncate(e.Summary, 60))
		case session.EntrySessionInfo:
			fmt.Fprintf(os.Stderr, "  %d. ℹ️  会话: %s\n", i+1, e.SessionID)
		case session.EntryModelChange:
			fmt.Fprintf(os.Stderr, "  %d. 🔄 模型变更: %s/%s\n", i+1, e.Provider, e.ModelID)
		}
	}
}

func printContextInfo(ctx session.SessionContext) {
	fmt.Fprintf(os.Stderr, "\n📊 上下文信息:\n")
	fmt.Fprintln(os.Stderr, "─────────────────────────────────────")
	fmt.Fprintf(os.Stderr, "  消息数量: %d\n", len(ctx.Messages))
	if ctx.Model != nil {
		fmt.Fprintf(os.Stderr, "  当前模型: %s/%s\n", ctx.Model.Provider, ctx.Model.ModelID)
	}
	if ctx.ThinkingLevel != "" {
		fmt.Fprintf(os.Stderr, "  思考级别: %s\n", ctx.ThinkingLevel)
	}

	// 统计各类消息
	var user, assistant, tool int
	for _, m := range ctx.Messages {
		switch m.(type) {
		case core.UserMessage:
			user++
		case core.AssistantMessage:
			assistant++
		case core.ToolResultMessage:
			tool++
		}
	}
	fmt.Fprintf(os.Stderr, "  用户消息: %d, 助手消息: %d, 工具结果: %d\n", user, assistant, tool)

	// 估算 token
	totalChars := 0
	for _, m := range ctx.Messages {
		switch mm := m.(type) {
		case core.UserMessage:
			if s, ok := mm.Content.(string); ok {
				totalChars += len(s)
			}
		case core.AssistantMessage:
			for _, b := range mm.Content {
				if tc, ok := b.(core.TextContent); ok {
					totalChars += len(tc.Text)
				}
			}
		}
	}
	fmt.Fprintf(os.Stderr, "  估算 token: ~%d\n", totalChars/4)
}

func performCompaction(ctx context.Context, sess *session.Session, model core.Model, config agent.AgentLoopConfig) {
	msgCtx := sess.BuildContext()
	messages := msgCtx.Messages

	fmt.Fprintf(os.Stderr, "\n🗜️  执行手动压缩 (%d 条消息)...\n", len(messages))

	settings := session.DefaultCompactionSettings()
	settings.MaxTokensBeforeCompaction = 100 // 低阈值，方便演示

	opts := core.SimpleStreamOptions{StreamOptions: core.StreamOptions{APIKey: config.SimpleStreamOptions.APIKey}}
	result, err := session.CompactSession(ctx, sess, model, settings, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "压缩失败: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "✅ 压缩完成:\n")
	fmt.Fprintf(os.Stderr, "  摘要: %s\n", truncate(result.Summary, 100))
	fmt.Fprintf(os.Stderr, "  节省 token: %d\n", result.TokensSaved)
	fmt.Fprintf(os.Stderr, "  保留消息: %d\n", result.EntriesKept)
}

func performBranchSummary(ctx context.Context, sess *session.Session, model core.Model, messages []core.Message) {
	if len(messages) == 0 {
		fmt.Fprintln(os.Stderr, "没有消息可以生成分支摘要")
		return
	}

	fmt.Fprintf(os.Stderr, "\n🌿 生成分支摘要 (%d 条消息)...\n", len(messages))

	opts := core.SimpleStreamOptions{StreamOptions: core.StreamOptions{APIKey: os.Getenv("DEEPSEEK_API_KEY")}}
	result, err := session.SummarizeBranchSession(ctx, sess, model, messages, "branch-1", opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "分支摘要失败: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "✅ 分支摘要:\n%s\n", result.Summary)
}

func saveSession(sess *session.Session, messages []core.Message) {
	fmt.Fprintf(os.Stderr, "\n💾 会话状态:\n")
	fmt.Fprintf(os.Stderr, "  会话 ID: %s\n", sess.ID())
	fmt.Fprintf(os.Stderr, "  条目数: %d\n", len(sess.Entries()))
	fmt.Fprintf(os.Stderr, "  消息数: %d\n", len(messages))
	fmt.Fprintln(os.Stderr, "  (使用 -persist 标志启动以启用 JSONL 持久化)")
}

func printUsageInfo(usage core.Usage) {
	if usage.Input > 0 || usage.Output > 0 {
		fmt.Fprintf(os.Stderr, "\n📊 Token: 输入=%d 输出=%d", usage.Input, usage.Output)
		if usage.CacheRead > 0 {
			fmt.Fprintf(os.Stderr, " 缓存读=%d", usage.CacheRead)
		}
		if usage.Cost.Total > 0 {
			fmt.Fprintf(os.Stderr, " 费用=$%.4f", usage.Cost.Total)
		}
		fmt.Fprintln(os.Stderr)
	}
}

func isSameMessage(a, b core.Message) bool {
	// 简单比较：如果都是 UserMessage 且内容相同则认为相同
	ua, okA := a.(core.UserMessage)
	ub, okB := b.(core.UserMessage)
	if okA && okB {
		return fmt.Sprintf("%v", ua.Content) == fmt.Sprintf("%v", ub.Content)
	}
	return false
}

func extractToolText(msg core.ToolResultMessage) string {
	for _, b := range msg.Content {
		if tc, ok := b.(core.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func resolveModel(provider, modelID string) core.Model {
	provider = strings.ToLower(provider)
	if m, err := llm.GetModel(core.KnownProvider(provider), modelID); err == nil && m.ID != "" {
		return m
	}
	api := core.APIOpenAICompletions
	return core.Model{
		ID:       modelID,
		Provider: core.KnownProvider(provider),
		API:      api,
	}
}

func loadEnv() {
	file, err := os.Open(".env")
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}
