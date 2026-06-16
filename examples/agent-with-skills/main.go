// Example: Agent with skills + context management + memory persistence
//
// 演示功能：
//  1. 加载 skills (Trae-style SKILL.md)
//  2. 多轮对话历史持久化 (JSONL)
//  3. 上下文窗口管理 (token 估算 + 自动压缩)
//  4. 长期记忆 (KV 存储, 跨会话保留)
//  5. 交互命令 (:history, :context, :compact, :memory, :save, :load)
//
// 性能优化：
//  1. System prompt 缓存：skills/memory 不变时复用
//  2. 增量 token 统计：O(1) 添加 vs O(N) 重算
//  3. 并行化：load 与构建并发
//  4. 异步 session 写入：批量 sync 不阻塞主流程
//  5. 流式返回：边收边打印
//
// Usage:
//
//	go run . -skills ./skills -session ./session.jsonl -memory ./memory.json -query "your question"
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"pi-ai-go/agent"
	"pi-ai-go/agent/session"
	"pi-ai-go/agent/tools"
	"pi-ai-go/core"
	"pi-ai-go/llm"
	_ "pi-ai-go/providers"

	"examples/agent-with-skills/autolearn"
	"examples/agent-with-skills/contextmgr"
	"examples/agent-with-skills/memory"
)

func main() {
	loadEnv()

	skillsDir := flag.String("skills", os.Getenv("SKILLS"), "Directory containing SKILL.md files")
	modelID := flag.String("model", os.Getenv("MODEL"), "Model ID")
	provider := flag.String("provider", os.Getenv("PROVIDER"), "Provider name")
	baseURL := flag.String("base-url", os.Getenv("BASE_URL"), "Base URL")
	apiKeyFlag := flag.String("api-key", "", "API key")
	query := flag.String("query", "", "Query to run (interactive if empty)")
	verbose := flag.Bool("v", false, "Verbose mode")

	sessionPath := flag.String("session", os.Getenv("SESSION"), "JSONL session file (empty = memory only)")
	memoryPath := flag.String("memory", os.Getenv("MEMORY"), "Long-term memory file (empty = no memory)")
	autoCompact := flag.Bool("auto-compact", true, "Auto compact when context exceeds soft limit")
	flag.Parse()

	fmt.Fprintf(os.Stderr, "[config] Provider: %s\n", *provider)
	fmt.Fprintf(os.Stderr, "[config] Model: %s\n", *modelID)
	fmt.Fprintf(os.Stderr, "[config] Base URL: %s\n", *baseURL)
	fmt.Fprintf(os.Stderr, "[config] Skills dir: %s\n", *skillsDir)
	fmt.Fprintf(os.Stderr, "[config] Session: %s\n", *sessionPath)
	fmt.Fprintf(os.Stderr, "[config] Memory: %s\n", *memoryPath)
	fmt.Fprintf(os.Stderr, "[config] Auto-compact: %v\n", *autoCompact)

	if *modelID == "" {
		fmt.Fprintln(os.Stderr, "Error: MODEL not configured")
		os.Exit(1)
	}

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

	model := resolveModel(*provider, *modelID, *baseURL)
	if model.ID == "" {
		fmt.Fprintln(os.Stderr, "Error: Failed to resolve model")
		os.Exit(1)
	}

	// 并行加载：skills + memory + session
	type loadResult struct {
		skillsText string
		mem        *memory.Memory
		sess       *session.Session
		sessStore  *session.JSONLStorage
		err        error
	}
	resCh := make(chan loadResult, 1)
	go func() {
		// skills 加载
		skillsText := loadSkillsText(*skillsDir, *verbose)

		// 长期记忆
		mem := initMemory(*memoryPath, *verbose)

		// session
		sess, sessStore, err := initSession(*sessionPath)

		resCh <- loadResult{
			skillsText: skillsText,
			mem:        mem,
			sess:       sess,
			sessStore:  sessStore,
			err:        err,
		}
	}()

	loaded := <-resCh
	if loaded.err != nil {
		fmt.Fprintf(os.Stderr, "[session] Warning: %v\n", loaded.err)
	}
	skillsText := loaded.skillsText
	mem := loaded.mem
	sess := loaded.sess
	sessStore := loaded.sessStore

	defer func() {
		if sess != nil {
			sess.Close()
		}
		if sessStore != nil {
			sessStore.Close()
		}
	}()

	// 上下文管理
	ctxSettings := contextmgr.DefaultSettings(model.ID)
	if *verbose {
		fmt.Fprintf(os.Stderr, "[context] max=%d soft=%d hard=%d\n",
			ctxSettings.MaxContextTokens, ctxSettings.SoftLimit(), ctxSettings.HardLimit())
	}

	// System prompt 缓存
	promptCache := NewSystemPromptCache()
	systemPrompt := promptCache.Get(skillsText, mem)

	// Agent 配置
	config := agent.AgentLoopConfig{
		Model:        model,
		SystemPrompt: systemPrompt,
		Tools:        tools.All(),
	}
	config.SimpleStreamOptions.APIKey = apiKey
	config.SimpleStreamOptions.MaxRetries = 3
	config.SimpleStreamOptions.MaxRetryDelayMs = 30000

	// 从 session 加载历史消息（增量统计 token）
	var messages []core.Message
	tokenStats := contextmgr.NewTokenStats(ctxSettings)
	if sess != nil {
		ctx2 := sess.BuildContext()
		messages = ctx2.Messages
		tokenStats.Recompute(messages)
		if *verbose {
			fmt.Fprintf(os.Stderr, "[session] Loaded %d message(s), %d tokens\n",
				len(messages), tokenStats.Tokens())
		}
	}

	// 启动异步 session flusher
	var sessFlusher *SessionFlusher
	if sess != nil && sessStore != nil {
		sessFlusher = NewSessionFlusher(sess, sessStore, 500*time.Millisecond)
		sessFlusher.Start()
		defer sessFlusher.Stop()
	}

	// 自动学习器
	autoLearn := autolearn.New(mem, autolearn.DefaultSettings())

	if *query != "" {
		runSingleQuery(config, messages, *query, *verbose, sess, mem, tokenStats, ctxSettings, model, apiKey, *autoCompact, promptCache, sessFlusher, autoLearn)
	} else {
		runInteractive(config, messages, *verbose, sess, mem, tokenStats, ctxSettings, model, apiKey, *autoCompact, promptCache, sessFlusher, autoLearn)
	}
}

// SystemPromptCache 缓存 system prompt 组件。
//
// 当 skills / memory 哈希不变时复用上次拼接结果。
// 避免每轮都重新格式化大量 skills 和 memory。
type SystemPromptCache struct {
	mu        sync.RWMutex
	cached    string
	skillsKey string
	memKey    string
	memSize   int
}

func NewSystemPromptCache() *SystemPromptCache {
	return &SystemPromptCache{}
}

// Get 获取 system prompt，必要时重建。
func (c *SystemPromptCache) Get(skillsText string, mem *memory.Memory) string {
	skillsKey := hashString(skillsText)
	memKey := ""
	memSize := 0
	if mem != nil {
		memKey = mem.Hash()
		memSize = mem.Size()
	}

	c.mu.RLock()
	if c.cached != "" && c.skillsKey == skillsKey && c.memKey == memKey && c.memSize == memSize {
		defer c.mu.RUnlock()
		return c.cached
	}
	c.mu.RUnlock()

	// 重建
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cached = buildSystemPrompt(skillsText, mem)
	c.skillsKey = skillsKey
	c.memKey = memKey
	c.memSize = memSize
	return c.cached
}

// Invalidate 强制下次重新构建（如记忆被修改）。
func (c *SystemPromptCache) Invalidate() {
	c.mu.Lock()
	c.cached = ""
	c.skillsKey = ""
	c.memKey = ""
	c.mu.Unlock()
}

// SessionFlusher 异步批量写入 session。
//
// 设计目的：
// - 避免每条消息都 fsync（10-50ms 延迟）
// - 批量写：积攒 N 条 entries 或 T 时间后统一 Sync
// - 优雅关闭：退出时 flush 剩余 entries
type SessionFlusher struct {
	mu         sync.Mutex
	sess       *session.Session
	store      *session.JSONLStorage
	flushEvery time.Duration
	stopCh     chan struct{}
	doneCh     chan struct{}
	pending    []session.SessionTreeEntry
}

func NewSessionFlusher(sess *session.Session, store *session.JSONLStorage, flushEvery time.Duration) *SessionFlusher {
	return &SessionFlusher{
		sess:       sess,
		store:      store,
		flushEvery: flushEvery,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

// Start 启动后台 flush goroutine。
func (f *SessionFlusher) Start() {
	go f.run()
}

// Stop 停止后台 goroutine 并 flush 剩余数据。
func (f *SessionFlusher) Stop() {
	close(f.stopCh)
	<-f.doneCh
	f.flushNow()
}

func (f *SessionFlusher) run() {
	defer close(f.doneCh)
	ticker := time.NewTicker(f.flushEvery)
	defer ticker.Stop()
	for {
		select {
		case <-f.stopCh:
			return
		case <-ticker.C:
			f.flushNow()
		}
	}
}

// Add 添加要写入的 entries（非阻塞）。
func (f *SessionFlusher) Add(entries []session.SessionTreeEntry) {
	if len(entries) == 0 {
		return
	}
	f.mu.Lock()
	f.pending = append(f.pending, entries...)
	f.mu.Unlock()
}

// flushNow 立即 flush 所有 pending entries。
func (f *SessionFlusher) flushNow() {
	f.mu.Lock()
	if len(f.pending) == 0 {
		f.mu.Unlock()
		return
	}
	entries := f.pending
	f.pending = nil
	f.mu.Unlock()

	// 实际写入（Append 内部会 Sync）
	if err := f.sess.Append(entries...); err != nil {
		fmt.Fprintf(os.Stderr, "[session] flush error: %v\n", err)
	}
}

// loadSkillsText 加载 skills 并格式化为 system prompt 片段。
func loadSkillsText(skillsDir string, verbose bool) string {
	if skillsDir == "" {
		return ""
	}
	skills, diags := session.LoadSkills(skillsDir)
	for _, d := range diags {
		fmt.Fprintf(os.Stderr, "[skill] %s: %s\n", d.Path, d.Message)
	}
	if len(skills) == 0 {
		return ""
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "Loaded %d skill(s)\n", len(skills))
	}
	return session.FormatSkillsForSystemPrompt(skills)
}

// initMemory 加载或创建长期记忆。
func initMemory(path string, verbose bool) *memory.Memory {
	if path == "" {
		return nil
	}
	mem, err := memory.New(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[memory] Error: %v\n", err)
		return nil
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "[memory] Loaded %d entries from %s\n", mem.Size(), path)
	}
	return mem
}

// initSession 初始化 session。
func initSession(path string) (*session.Session, *session.JSONLStorage, error) {
	if path == "" {
		storage := session.NewMemoryStorage()
		sess, err := session.NewSession(storage)
		return sess, nil, err
	}
	storage, err := session.NewJSONLStorage(path)
	if err != nil {
		return nil, nil, err
	}
	sess, err := session.NewSession(storage)
	if err != nil {
		storage.Close()
		return nil, nil, err
	}
	return sess, storage, nil
}

// buildSystemPrompt 拼装最终 system prompt。
func buildSystemPrompt(skillsText string, mem *memory.Memory) string {
	var sb strings.Builder
	sb.WriteString("You are a helpful coding assistant. You have access to file system tools.")

	if skillsText != "" {
		sb.WriteString("\n\n")
		sb.WriteString(skillsText)
	}

	if mem != nil && mem.Size() > 0 {
		sb.WriteString("\n\n")
		sb.WriteString(mem.FormatForPrompt())
	}

	return sb.String()
}

// hashString 计算字符串的快速 hash。
func hashString(s string) string {
	if s == "" {
		return ""
	}
	// FNV-1a 64-bit
	const (
		offset = 14695981039346656037
		prime  = 1099511628211
	)
	h := uint64(offset)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime
	}
	return fmt.Sprintf("%x", h)
}

// runSingleQuery 运行单次查询。
func runSingleQuery(
	config agent.AgentLoopConfig,
	messages []core.Message,
	query string,
	verbose bool,
	sess *session.Session,
	mem *memory.Memory,
	tokenStats *contextmgr.TokenStats,
	ctxSettings contextmgr.Settings,
	model core.Model,
	apiKey string,
	autoCompact bool,
	promptCache *SystemPromptCache,
	sessFlusher *SessionFlusher,
	autoLearn *autolearn.AutoLearner,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// 添加用户消息
	userMsg := core.UserMessage{
		Role:      "user",
		Content:   query,
		Timestamp: time.Now(),
	}
	messages = append(messages, userMsg)
	tokenStats.Add(userMsg)

	// 自动压缩
	if autoCompact {
		var compacted bool
		messages, compacted = maybeCompact(ctx, messages, tokenStats, ctxSettings, model, apiKey, verbose)
		if compacted {
			tokenStats.Recompute(messages)
		}
	}

	stream, detailed := agent.AgentLoopDetailed(ctx, messages, config)

	_, err := stream.ForEach(ctx, func(evt agent.AgentEvent) error {
		return handleEvent(evt, verbose)
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
	}

	// 流结束后获取最终结果
	res, derr := detailed()
	if derr == nil {
		if len(res.Messages) > len(messages) {
			newEntries := res.Messages[len(messages):]
			if sessFlusher != nil {
				sessFlusher.Add(toTreeEntries(newEntries))
			} else if sess != nil {
				if err := sess.Append(toTreeEntries(newEntries)...); err != nil {
					fmt.Fprintf(os.Stderr, "[session] append error: %v\n", err)
				}
			}
		}
	} else {
		fmt.Fprintf(os.Stderr, "[summary] error: %v\n", derr)
	}

	fmt.Println()
	printSummary(detailed)
}

// runInteractive 交互模式。
func runInteractive(
	config agent.AgentLoopConfig,
	messages []core.Message,
	verbose bool,
	sess *session.Session,
	mem *memory.Memory,
	tokenStats *contextmgr.TokenStats,
	ctxSettings contextmgr.Settings,
	model core.Model,
	apiKey string,
	autoCompact bool,
	promptCache *SystemPromptCache,
	sessFlusher *SessionFlusher,
	autoLearn *autolearn.AutoLearner,
) {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Fprintln(os.Stderr, "\n🤖 Interactive mode")
	fmt.Fprintln(os.Stderr, "Commands: :history :context :compact :memory :save :load :remember :forget :stats :quit")
	fmt.Fprintln(os.Stderr, "Or type a question.")

	for {
		fmt.Fprint(os.Stderr, "\nquery> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// 命令以 : 开头
		if strings.HasPrefix(input, ":") {
			handleCommand(input, messages, sess, mem, tokenStats, ctxSettings, model, apiKey, verbose, promptCache)
			continue
		}

		// 添加用户消息
		userMsg := core.UserMessage{
			Role:      "user",
			Content:   input,
			Timestamp: time.Now(),
		}
		messages = append(messages, userMsg)
		tokenStats.Add(userMsg)

		// 自动学习：从用户输入提取记忆
		if count := autoLearn.ProcessUserInput(input); count > 0 {
			promptCache.Invalidate()
			fmt.Fprintf(os.Stderr, "[memory] auto-learned %d item(s)\n", count)
		}

		// 自动压缩
		if autoCompact {
			var compacted bool
			messages, compacted = maybeCompact(context.Background(), messages, tokenStats, ctxSettings, model, apiKey, verbose)
			if compacted {
				tokenStats.Recompute(messages)
			}
		}

		// 运行 agent
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		stream, detailed := agent.AgentLoopDetailed(ctx, messages, config)

		_, err := stream.ForEach(ctx, func(evt agent.AgentEvent) error {
			return handleEvent(evt, verbose)
		})
		cancel()

		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
			continue
		}

		// 获取最终结果
		res, derr := detailed()
		if derr == nil {
			if len(res.Messages) > len(messages) {
				newEntries := res.Messages[len(messages):]
				if sessFlusher != nil {
					sessFlusher.Add(toTreeEntries(newEntries))
				} else if sess != nil {
					if err := sess.Append(toTreeEntries(newEntries)...); err != nil {
						fmt.Fprintf(os.Stderr, "[session] append error: %v\n", err)
					}
				}
				messages = res.Messages
				// token 统计也用真实使用值更新
				if last := lastAssistantMessage(messages); last != nil && last.Usage.Input > 0 {
					tokenStats.Recompute(messages)
				} else {
					tokenStats.AddMany(newEntries)
				}
			}
		} else {
			fmt.Fprintf(os.Stderr, "[summary] error: %v\n", derr)
		}

		fmt.Println()
	}
}

// lastAssistantMessage 返回最后一条 assistant 消息。
func lastAssistantMessage(messages []core.Message) *core.AssistantMessage {
	for i := len(messages) - 1; i >= 0; i-- {
		if am, ok := messages[i].(core.AssistantMessage); ok {
			return &am
		}
	}
	return nil
}

// handleCommand 处理交互命令。
func handleCommand(
	cmd string,
	messages []core.Message,
	sess *session.Session,
	mem *memory.Memory,
	tokenStats *contextmgr.TokenStats,
	ctxSettings contextmgr.Settings,
	model core.Model,
	apiKey string,
	verbose bool,
	promptCache *SystemPromptCache,
) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch strings.ToLower(parts[0]) {
	case ":quit", ":exit":
		fmt.Fprintln(os.Stderr, "Bye!")
		os.Exit(0)

	case ":history":
		fmt.Fprintf(os.Stderr, "\n📜 对话历史 (%d 条):\n", len(messages))
		for i, msg := range messages {
			role := "?"
			content := ""
			switch m := msg.(type) {
			case core.UserMessage:
				role = "🧑"
				content = truncate(fmt.Sprintf("%v", m.Content), 80)
			case core.AssistantMessage:
				role = "🤖"
				content = extractAssistantText(m)
				content = truncate(content, 80)
			case core.ToolResultMessage:
				role = "🔧"
				content = m.ToolName
			}
			fmt.Fprintf(os.Stderr, "  %d. %s %s\n", i+1, role, content)
		}

	case ":context":
		stats := tokenStats.Get()
		fmt.Fprintln(os.Stderr, "\n"+contextmgr.FormatStats(stats))

	case ":compact":
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		newMsgs, err := doCompact(ctx, messages, ctxSettings, model, apiKey, verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "压缩失败: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "✓ 已压缩: %d → %d 消息\n", len(messages), len(newMsgs))
		}

	case ":memory":
		if mem == nil {
			fmt.Fprintln(os.Stderr, "未启用长期记忆（设置 -memory 参数）")
			return
		}
		fmt.Fprintf(os.Stderr, "\n🧠 长期记忆 (%d 条):\n", mem.Size())
		for _, key := range mem.Keys() {
			val, _ := mem.Get(key)
			fmt.Fprintf(os.Stderr, "  %s = %s\n", key, val)
		}

	case ":remember":
		if len(parts) < 2 {
			fmt.Fprintln(os.Stderr, "用法: :remember <key> = <value>")
			return
		}
		rest := strings.TrimPrefix(cmd, ":remember")
		rest = strings.TrimSpace(rest)
		kv := strings.SplitN(rest, "=", 2)
		if len(kv) != 2 {
			fmt.Fprintln(os.Stderr, "用法: :remember <key> = <value>")
			return
		}
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		if mem == nil {
			fmt.Fprintln(os.Stderr, "未启用长期记忆")
			return
		}
		mem.Set(key, value)
		if err := mem.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "保存失败: %v\n", err)
		} else {
			promptCache.Invalidate() // 记忆变更需要重建 prompt
			fmt.Fprintf(os.Stderr, "✓ 已记忆: %s = %s\n", key, value)
		}

	case ":forget":
		if mem == nil {
			fmt.Fprintln(os.Stderr, "未启用长期记忆")
			return
		}
		if len(parts) < 2 {
			fmt.Fprintln(os.Stderr, "用法: :forget <key>")
			return
		}
		key := parts[1]
		mem.Delete(key)
		if err := mem.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "保存失败: %v\n", err)
		} else {
			promptCache.Invalidate()
			fmt.Fprintf(os.Stderr, "✓ 已遗忘: %s\n", key)
		}

	case ":save":
		if mem != nil {
			if err := mem.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "保存记忆失败: %v\n", err)
			} else {
				fmt.Fprintln(os.Stderr, "✓ 记忆已保存")
			}
		}
		fmt.Fprintf(os.Stderr, "✓ Session 当前 %d 条记录\n", sessSize(sess))

	case ":load":
		if mem != nil {
			if err := mem.Load(); err != nil {
				fmt.Fprintf(os.Stderr, "重新加载记忆失败: %v\n", err)
			} else {
				promptCache.Invalidate()
				fmt.Fprintf(os.Stderr, "✓ 重新加载 %d 条记忆\n", mem.Size())
			}
		}
		fmt.Fprintf(os.Stderr, "✓ Session 当前 %d 条记录\n", sessSize(sess))

	case ":stats":
		stats := tokenStats.Get()
		fmt.Fprintln(os.Stderr, "\n"+contextmgr.FormatStats(stats))
		if mem != nil {
			fmt.Fprintf(os.Stderr, "🧠 长期记忆: %d 条\n", mem.Size())
		}
		fmt.Fprintf(os.Stderr, "💾 Session: %d 条记录\n", sessSize(sess))

	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", parts[0])
		fmt.Fprintln(os.Stderr, "可用: :history :context :compact :memory :remember :forget :save :load :stats :quit")
	}
}

// handleEvent 处理 agent 事件。
func handleEvent(evt agent.AgentEvent, verbose bool) error {
	switch e := evt.(type) {
	case agent.EventMessageUpdate:
		switch ae := e.AssistantEvent.(type) {
		case core.EventTextDelta:
			fmt.Print(ae.Delta)
		case core.EventThinkingDelta:
			if verbose {
				fmt.Fprintf(os.Stderr, "[thinking] %s", ae.Delta)
			}
		case core.EventToolCallStart:
			fmt.Fprintf(os.Stderr, "\n[tool] %s", ae.Name)
		case core.EventToolCallDelta:
			if len(ae.ArgumentsDelta) > 0 {
				fmt.Fprintf(os.Stderr, "%s", ae.ArgumentsDelta)
			}
		case core.EventToolCallEnd:
			fmt.Fprintf(os.Stderr, "\n")
		case core.EventError:
			fmt.Fprintf(os.Stderr, "\nError: %v\n", ae.Error)
		}
	case agent.EventMessageEnd:
		if e.Message.ErrorMessage != "" {
			fmt.Fprintf(os.Stderr, "\nError: %s\n", e.Message.ErrorMessage)
		}
	case agent.EventToolExecStart:
		fmt.Fprintf(os.Stderr, "\n[exec] %s\n", e.ToolName)
		if verbose && len(e.Args) > 0 {
			fmt.Fprintf(os.Stderr, "  args: %s\n", string(e.Args))
		}
	case agent.EventToolExecEnd:
		status := "ok"
		if e.IsError {
			status = "error"
		}
		fmt.Fprintf(os.Stderr, "[exec done] %s (%s)\n", e.ToolName, status)
	case agent.EventTurnEnd:
		if verbose {
			fmt.Fprintf(os.Stderr, "[turn end] ToolResults: %d\n", len(e.ToolResults))
		}
	case agent.EventAgentEnd:
		if verbose {
			fmt.Fprintf(os.Stderr, "[agent end]\n")
		}
	}
	return nil
}

// maybeCompact 在超出软限制时自动压缩。
// 返回 (new messages, true if compacted)
func maybeCompact(
	ctx context.Context,
	messages []core.Message,
	tokenStats *contextmgr.TokenStats,
	settings contextmgr.Settings,
	model core.Model,
	apiKey string,
	verbose bool,
) ([]core.Message, bool) {
	if !tokenStats.ShouldCompact() {
		return messages, false
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "\n⚠️  上下文达到 %d tokens, 触发自动压缩...\n", tokenStats.Tokens())
	}

	newMsgs, err := doCompact(ctx, messages, settings, model, apiKey, verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[compact] failed: %v\n", err)
		return messages, false
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "[compact] OK: %d → %d 消息\n", len(messages), len(newMsgs))
	}
	return newMsgs, true
}

// doCompact 执行压缩。
func doCompact(
	ctx context.Context,
	messages []core.Message,
	settings contextmgr.Settings,
	model core.Model,
	apiKey string,
	verbose bool,
) ([]core.Message, error) {
	opts := core.SimpleStreamOptions{}
	opts.APIKey = apiKey
	result, err := contextmgr.Compact(ctx, model, messages, settings, opts)
	if err != nil {
		return nil, err
	}
	return result.NewMessages, nil
}

// toTreeEntries 把 core.Message 转换为 session tree entries。
func toTreeEntries(msgs []core.Message) []session.SessionTreeEntry {
	entries := make([]session.SessionTreeEntry, 0, len(msgs))
	for _, msg := range msgs {
		entries = append(entries, session.SessionTreeEntry{
			ID:        session.GenerateID(),
			Type:      session.EntryMessage,
			Timestamp: msg.GetTimestamp(),
			Message:   msg,
		})
	}
	return entries
}

func sessSize(sess *session.Session) int {
	if sess == nil {
		return 0
	}
	return len(sess.Entries())
}

func extractAssistantText(m core.AssistantMessage) string {
	var parts []string
	for _, b := range m.Content {
		if c, ok := b.(core.TextContent); ok {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func printSummary(detailed func() (agent.AgentLoopDetailedResult, error)) {
	res, err := detailed()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Summary] Error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "\n[Summary] Steps: %d, Tools: %d, Cost: $%.4f, Tokens: %d in / %d out\n",
		res.Summary.StepCount,
		res.Summary.ToolCallCount,
		res.Summary.TotalCost,
		res.Summary.TotalUsage.Input,
		res.Summary.TotalUsage.Output,
	)
}

func resolveModel(provider, modelID, baseURL string) core.Model {
	provider = strings.ToLower(provider)

	if provider != "" && modelID != "" {
		if model, err := llm.GetModel(core.KnownProvider(provider), modelID); err == nil && model.ID != "" {
			if baseURL != "" {
				model.BaseURL = baseURL
			}
			return model
		}
	}

	api := core.KnownAPI("")
	switch core.KnownProvider(provider) {
	case core.ProviderOpenAI, core.ProviderDeepSeek, core.ProviderGroq, core.ProviderFireworks, core.ProviderTogether, core.ProviderCerebras:
		api = core.APIOpenAICompletions
	case core.ProviderAnthropic:
		api = core.APIAnthropicMessages
	case core.ProviderGoogle:
		api = core.APIGoogleGenerative
	case core.ProviderGoogleVertex:
		api = core.APIGoogleVertex
	case core.ProviderMistral:
		api = core.APIMistralConversations
	case core.ProviderAzureOpenAI:
		api = core.APIAzureOpenAIResponses
	case core.ProviderOpenRouter:
		api = core.OpenRouter
	case core.ProviderAmazonBedrock:
		api = core.APIBedrockConverse
	default:
		api = core.APIOpenAICompletions
	}

	return core.Model{
		ID:       modelID,
		Provider: core.KnownProvider(provider),
		API:      api,
		BaseURL:  baseURL,
	}
}

func loadEnv() {
	file, err := os.Open(".env")
	if err != nil {
		if exe, e := os.Executable(); e == nil {
			file, err = os.Open(filepath.Join(filepath.Dir(exe), ".env"))
		}
		if err != nil {
			return
		}
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
