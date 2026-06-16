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
	"runtime"
	"sort"
	"strconv"
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
	"examples/agent-with-skills/keypool"
	"examples/agent-with-skills/memory"
)

func main() {
	loadEnv()

	skillsDir := flag.String("skills", os.Getenv("SKILLS"), "Directory containing SKILL.md files")
	modelID := flag.String("model", os.Getenv("MODEL"), "Model ID")
	provider := flag.String("provider", os.Getenv("PROVIDER"), "Provider name")
	baseURL := flag.String("base-url", os.Getenv("BASE_URL"), "Base URL")
	apiKeyFlag := flag.String("api-key", "", "API key (single, or use -api-keys for rotation)")
	apiKeysFlag := flag.String("api-keys", os.Getenv("API_KEYS"), "Comma-separated API keys for rotation (highest priority)")
	query := flag.String("query", "", "Query to run (interactive if empty)")
	verbose := flag.Bool("v", false, "Verbose mode")

	sessionPath := flag.String("session", os.Getenv("SESSION"), "JSONL session file (empty = memory only)")
	memoryPath := flag.String("memory", os.Getenv("MEMORY"), "Long-term memory file (empty = no memory)")
	autoCompact := flag.Bool("auto-compact", true, "Auto compact when context exceeds soft limit")
	autoLearnFlag := flag.Bool("auto-learn", envBool("AUTO_LEARN", false), "Auto-extract memories from LLM every N turns")
	extractEveryFlag := flag.Int("extract-every", envInt("EXTRACT_EVERY_N", 5), "Trigger LLM memory extraction every N turns")
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

	// 收集 API key 列表（多 key 轮询）
	keys := collectAPIKeys(*apiKeysFlag, *apiKeyFlag, *provider, *verbose)
	if len(keys) == 0 {
		fmt.Fprintln(os.Stderr, "Error: API key(s) not configured (use -api-keys or API_KEYS env)")
		os.Exit(1)
	}

	keyPool := keypool.New(keys, keypool.DefaultSettings())
	if *verbose {
		fmt.Fprintf(os.Stderr, "[keypool] %d key(s) loaded\n", keyPool.Size())
		for _, s := range keyPool.Status() {
			fmt.Fprintf(os.Stderr, "[keypool] %s\n", s.String())
		}
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
		sessRef    *SessionRef
		err        error
	}
	resCh := make(chan loadResult, 1)
	go func() {
		skillsText := loadSkillsText(*skillsDir, *verbose)
		mem := initMemory(*memoryPath, *verbose)

		// Session 目录：
		//   - -session 指定的是单文件路径：取其所在目录
		//   - 未指定：使用 ./sessions/
		sessionsDir := "./sessions"
		defaultName := "main"
		if *sessionPath != "" {
			abs, _ := filepath.Abs(*sessionPath)
			dir := filepath.Dir(abs)
			if dir != "." {
				sessionsDir = dir
			}
			defaultName = strings.TrimSuffix(filepath.Base(abs), ".jsonl")
			if defaultName == "" || defaultName == filepath.Base(abs) {
				defaultName = "main"
			}
		}

		sessRef, err := NewSessionRef(defaultName, sessionsDir, 500*time.Millisecond)
		resCh <- loadResult{
			skillsText: skillsText,
			mem:        mem,
			sessRef:    sessRef,
			err:        err,
		}
	}()

	loaded := <-resCh
	if loaded.err != nil {
		fmt.Fprintf(os.Stderr, "[session] Warning: %v\n", loaded.err)
	}
	skillsText := loaded.skillsText
	mem := loaded.mem
	sessRef := loaded.sessRef
	defer func() {
		if sessRef != nil {
			sessRef.Close()
		}
	}()

	if *verbose && sessRef != nil {
		fmt.Fprintf(os.Stderr, "[session] dir=%s, current=%s\n", sessRef.dir, sessRef.Name())
	}

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
	// 初始 key（每次 AgentLoopDetailed 调用前会重新从 keyPool 取）
	if firstKey, err := keyPool.Next(); err == nil {
		config.SimpleStreamOptions.APIKey = firstKey
	} else {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	config.SimpleStreamOptions.MaxRetries = 3
	config.SimpleStreamOptions.MaxRetryDelayMs = 30000

	// 从 session 加载历史消息（增量统计 token）
	tokenStats := contextmgr.NewTokenStats(ctxSettings)
	if sessRef != nil && sessRef.Session() != nil {
		ctx2 := sessRef.Session().BuildContext()
		sessRef.Messages = ctx2.Messages
		tokenStats.Recompute(sessRef.Messages)
		if *verbose {
			fmt.Fprintf(os.Stderr, "[session] Loaded %d message(s), %d tokens\n",
				len(sessRef.Messages), tokenStats.Tokens())
		}
	}

	// 自动学习器（含 LLM 提取）
	var extractor autolearn.Extractor
	var wfExtractor *autolearn.WorkflowExtractor
	if *autoLearnFlag {
		extractor = newLLMExtractor(model, keyPool, *verbose)
		wfExtractor = newWorkflowExtractor(model, keyPool, *verbose)

		// 加载 skill-writer/SKILL.md 作为工作流生成的参考规范。
		// 如果加载成功，wfExtractor 就能直接让 LLM 按 skill-writer 标准输出完整 SKILL.md。
		if doc, err := loadSkillWriterDoc(*skillsDir); err == nil && doc != "" {
			wfExtractor.SkillWriterDoc = doc
			if *verbose {
				fmt.Fprintf(os.Stderr, "[workflow] 已加载 skill-writer 参考规范（%d 字符）\n", len(doc))
			}
		} else if *verbose && err != nil {
			fmt.Fprintf(os.Stderr, "[workflow] 未找到 skill-writer/SKILL.md，使用回退路径: %v\n", err)
		}
	}
	autoLearn := autolearn.New(mem, autolearn.Settings{
		AutoLearn:     *autoLearnFlag,
		ExtractEveryN: *extractEveryFlag,
		MinConfidence: 0.7,
	})
	// 设置 workflow 输出目录：<skillsDir>/auto-extracted
	if *skillsDir != "" {
		autoLearn.WorkflowDir = filepath.Join(*skillsDir, "auto-extracted")
		_ = os.MkdirAll(autoLearn.WorkflowDir, 0755)
	}

	if *query != "" {
		runSingleQuery(config, *query, *verbose, sessRef, mem, tokenStats, ctxSettings, model, keyPool, *autoCompact, promptCache, autoLearn, extractor)
	} else {
		runInteractive(config, *verbose, sessRef, mem, tokenStats, ctxSettings, model, keyPool, *autoCompact, promptCache, autoLearn, extractor, wfExtractor)
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

// Flush 立即同步写一次（不停止后台 goroutine）。
func (f *SessionFlusher) Flush() {
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

// loadSkillWriterDoc 加载 skill-writer/SKILL.md 的完整内容。
// 用于 wfExtractor 作为参考规范让 LLM 按 skill-writer 标准生成 SKILL.md。
// 找不到时返回空字符串和 nil 错误（不视为错误，使用回退路径）。
func loadSkillWriterDoc(skillsDir string) (string, error) {
	if skillsDir == "" {
		return "", nil
	}
	candidates := []string{
		filepath.Join(skillsDir, "skill-writer", "SKILL.md"),
		filepath.Join(skillsDir, "skill-writer", "skill.md"),
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err == nil {
			return string(data), nil
		}
	}
	return "", nil
}

// loadSkillsText 加载 skills 并格式化为 system prompt 片段。
// 自动合并用户手写 skills（skillsDir）和自动提取的 skills（skillsDir/auto-extracted）。
func loadSkillsText(skillsDir string, verbose bool) string {
	if skillsDir == "" {
		return ""
	}
	// 同时加载用户手写和 LLM 自动提取的 skills
	dirs := []string{skillsDir, filepath.Join(skillsDir, "auto-extracted")}
	skills, diags := session.LoadSkills(dirs...)
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
	recordSystemInfo(mem, verbose)
	return mem
}

// recordSystemInfo 记录操作系统信息到长期记忆。
// 只在首次启动时写入，后续启动不会覆盖用户修改的值。
func recordSystemInfo(mem *memory.Memory, verbose bool) {
	if mem == nil {
		return
	}
	const osKey = "system.os"
	if mem.Has(osKey) {
		return
	}
	goos := runtime.GOOS
	var osName string
	switch goos {
	case "windows":
		osName = "windows"
	case "darwin":
		osName = "mac"
	case "linux":
		osName = "linux"
	default:
		osName = goos
	}
	mem.SetWithCategory(osKey, osName, "system")
	if err := mem.Save(); err != nil && verbose {
		fmt.Fprintf(os.Stderr, "[memory] 保存系统信息失败: %v\n", err)
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "[memory] 系统信息: %s = %s\n", osKey, osName)
	}
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

// SessionRef 持有当前活跃 session 及其状态。
// 通过该引用可以在运行时切换到不同的 session 文件。
type SessionRef struct {
	mu       sync.Mutex
	name     string // 当前 session 名称
	dir      string // sessions 目录
	sess     *session.Session
	store    *session.JSONLStorage
	flusher  *SessionFlusher
	Messages []core.Message
}

// NewSessionRef 创建一个 session 引用。
// name: session 名称（不含 .jsonl 后缀）
// dir: session 文件存放目录（自动创建）
// flushEvery: 后台 flush 间隔；<=0 表示不启动 flusher
func NewSessionRef(name, dir string, flushEvery time.Duration) (*SessionRef, error) {
	r := &SessionRef{name: name, dir: dir}
	if err := r.openLocked(name); err != nil {
		return nil, err
	}
	if flushEvery > 0 {
		r.flusher = NewSessionFlusher(r.sess, r.store, flushEvery)
		r.flusher.Start()
	}
	return r, nil
}

// openLocked 打开一个 session（调用方需持锁）。
func (r *SessionRef) openLocked(name string) error {
	if err := os.MkdirAll(r.dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(r.dir, name+".jsonl")
	store, err := session.NewJSONLStorage(path)
	if err != nil {
		return err
	}
	sess, err := session.NewSession(store)
	if err != nil {
		store.Close()
		return err
	}
	r.sess = sess
	r.store = store
	r.name = name
	r.Messages = nil
	return nil
}

// Session 访问当前 Session。
func (r *SessionRef) Session() *session.Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sess
}

// Name 返回当前 session 名称。
func (r *SessionRef) Name() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.name
}

// Switch 切换到指定 name 的 session。会自动 flush 当前 session。
func (r *SessionRef) Switch(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.flusher != nil {
		r.flusher.Flush()
		r.flusher.Stop()
		r.flusher = nil
	}
	if r.store != nil {
		_ = r.store.Close()
	}
	return r.openLocked(name)
}

// AppendTreeEntries 把 entries 加入后台 flusher。
func (r *SessionRef) AppendTreeEntries(entries []session.SessionTreeEntry) {
	r.mu.Lock()
	flusher := r.flusher
	r.mu.Unlock()
	if flusher != nil {
		flusher.Add(entries)
	}
}

// Close 关闭当前 session。
func (r *SessionRef) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.flusher != nil {
		r.flusher.Stop()
		r.flusher = nil
	}
	if r.store != nil {
		_ = r.store.Close()
		r.store = nil
	}
}

// sessionInfo 描述一个 session 文件。
type sessionInfo struct {
	Name     string
	Path     string
	Size     int64
	Modified time.Time
}

// ListSessions 扫描目录，列出所有 session 文件，按修改时间倒序。
func ListSessions(dir string) ([]sessionInfo, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	out := make([]sessionInfo, 0, len(files))
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(f), ".jsonl")
		out = append(out, sessionInfo{
			Name:     name,
			Path:     f,
			Size:     info.Size(),
			Modified: info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Modified.After(out[j].Modified)
	})
	return out, nil
}

// LoadSessionMessages 加载指定 session 文件中的所有消息（不打开文件持有）。
// 用于 :view 命令只读查看。
func LoadSessionMessages(path string) ([]core.Message, error) {
	store, err := session.NewJSONLStorage(path)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	entries, err := store.ReadAll()
	if err != nil {
		return nil, err
	}
	var msgs []core.Message
	for _, e := range entries {
		if e.Type == session.EntryMessage {
			if m, ok := e.Message.(core.Message); ok {
				msgs = append(msgs, m)
			}
		}
	}
	return msgs, nil
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
	query string,
	verbose bool,
	sessRef *SessionRef,
	mem *memory.Memory,
	tokenStats *contextmgr.TokenStats,
	ctxSettings contextmgr.Settings,
	model core.Model,
	keyPool *keypool.Pool,
	autoCompact bool,
	promptCache *SystemPromptCache,
	autoLearn *autolearn.AutoLearner,
	extractor autolearn.Extractor,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// 添加用户消息
	userMsg := core.UserMessage{
		Role:      "user",
		Content:   query,
		Timestamp: time.Now(),
	}
	sessRef.Messages = append(sessRef.Messages, userMsg)
	sessRef.AppendTreeEntries(toTreeEntries([]core.Message{userMsg}))
	tokenStats.Add(userMsg)

	// 自动压缩
	if autoCompact {
		var compacted bool
		sessRef.Messages, compacted = maybeCompact(ctx, sessRef.Messages, tokenStats, ctxSettings, model, keyPool, verbose)
		if compacted {
			tokenStats.Recompute(sessRef.Messages)
		}
	}

	// 从 keyPool 取本次请求的 key
	key, kerr := keyPool.Next()
	if kerr != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", kerr)
		return
	}
	config.SimpleStreamOptions.APIKey = key

	stream, detailed := agent.AgentLoopDetailed(ctx, sessRef.Messages, config)

	_, err := stream.ForEach(ctx, func(evt agent.AgentEvent) error {
		return handleEvent(evt, verbose)
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		keyPool.MarkFailedByKey(key, keypool.CategorizeError(err))
	} else {
		keyPool.MarkSuccessByKey(key)
	}

	// 流结束后获取最终结果
	res, derr := detailed()
	if derr == nil {
		if len(res.Messages) > len(sessRef.Messages) {
			newEntries := res.Messages[len(sessRef.Messages):]
			sessRef.AppendTreeEntries(toTreeEntries(newEntries))
			sessRef.Messages = append(sessRef.Messages, newEntries...)
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
	verbose bool,
	sessRef *SessionRef,
	mem *memory.Memory,
	tokenStats *contextmgr.TokenStats,
	ctxSettings contextmgr.Settings,
	model core.Model,
	keyPool *keypool.Pool,
	autoCompact bool,
	promptCache *SystemPromptCache,
	autoLearn *autolearn.AutoLearner,
	extractor autolearn.Extractor,
	wfExtractor *autolearn.WorkflowExtractor,
) {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Fprintln(os.Stderr, "\n🤖 Interactive mode")
	fmt.Fprintln(os.Stderr, "Commands: :history :context :compact :memory :save :load :sessions :open :view :new :current :remember :forget :stats :keys :quit")
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
			handleCommand(input, sessRef, mem, tokenStats, ctxSettings, model, keyPool, verbose, promptCache)
			continue
		}

		// 添加用户消息
		userMsg := core.UserMessage{
			Role:      "user",
			Content:   input,
			Timestamp: time.Now(),
		}
		sessRef.Messages = append(sessRef.Messages, userMsg)
		sessRef.AppendTreeEntries(toTreeEntries([]core.Message{userMsg}))
		tokenStats.Add(userMsg)

		// 自动学习：从用户输入提取记忆
		if count := autoLearn.ProcessUserInput(input); count > 0 {
			promptCache.Invalidate()
			fmt.Fprintf(os.Stderr, "[memory] auto-learned %d item(s)\n", count)
		}

		// 自动压缩
		if autoCompact {
			var compacted bool
			sessRef.Messages, compacted = maybeCompact(context.Background(), sessRef.Messages, tokenStats, ctxSettings, model, keyPool, verbose)
			if compacted {
				tokenStats.Recompute(sessRef.Messages)
			}
		}

		// 从 keyPool 取本次请求的 key
		key, kerr := keyPool.Next()
		if kerr != nil {
			fmt.Fprintf(os.Stderr, "\n❌ %v\n", kerr)
			continue
		}
		config.SimpleStreamOptions.APIKey = key

		// 运行 agent
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		stream, detailed := agent.AgentLoopDetailed(ctx, sessRef.Messages, config)

		_, err := stream.ForEach(ctx, func(evt agent.AgentEvent) error {
			return handleEvent(evt, verbose)
		})
		cancel()

		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
			keyPool.MarkFailedByKey(key, keypool.CategorizeError(err))
			continue
		}

		// 获取最终结果
		res, derr := detailed()
		if derr == nil {
			if len(res.Messages) > len(sessRef.Messages) {
				newEntries := res.Messages[len(sessRef.Messages):]
				sessRef.AppendTreeEntries(toTreeEntries(newEntries))
				sessRef.Messages = res.Messages
				// token 统计也用真实使用值更新
				if last := lastAssistantMessage(sessRef.Messages); last != nil && last.Usage.Input > 0 {
					tokenStats.Recompute(sessRef.Messages)
				} else {
					tokenStats.AddMany(newEntries)
				}
			}
		} else {
			fmt.Fprintf(os.Stderr, "[summary] error: %v\n", derr)
		}

		// 异步 LLM 提取记忆（不阻塞主循环）
		if extractor != nil && autoLearn.Settings().AutoLearn {
			msgsCopy := append([]core.Message(nil), sessRef.Messages...)
			go func() {
				bgCtx, bgCancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer bgCancel()
				if autoLearn.MaybeExtract(bgCtx, msgsCopy, extractor) {
					promptCache.Invalidate()
					fmt.Fprintln(os.Stderr, "[memory] LLM 提取了新的记忆")
				}
			}()
		}

		// 异步 LLM 提取工作流 → 自动生成 SKILL.md
		if wfExtractor != nil && autoLearn.Settings().AutoLearn && autoLearn.WorkflowDir != "" {
			msgsCopy := append([]core.Message(nil), sessRef.Messages...)
			go func() {
				bgCtx, bgCancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer bgCancel()
				if n := autoLearn.MaybeExtractWorkflow(bgCtx, msgsCopy, wfExtractor); n > 0 {
					promptCache.Invalidate()
					fmt.Fprintf(os.Stderr, "[workflow] LLM 提取了 %d 个新 skill\n", n)
				}
			}()
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
	sessRef *SessionRef,
	mem *memory.Memory,
	tokenStats *contextmgr.TokenStats,
	ctxSettings contextmgr.Settings,
	model core.Model,
	keyPool *keypool.Pool,
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
		fmt.Fprintf(os.Stderr, "\n📜 对话历史 [%s] (%d 条):\n", sessRef.Name(), len(sessRef.Messages))
		for i, msg := range sessRef.Messages {
			role := "?"
			content := ""
			marker := ""
			switch m := msg.(type) {
			case core.UserMessage:
				role = "🧑"
				if s, ok := m.Content.(string); ok && strings.HasPrefix(s, "[对话历史摘要]") {
					marker = " [摘要]"
				}
				content = truncate(fmt.Sprintf("%v", m.Content), 80)
			case core.AssistantMessage:
				role = "🤖"
				if text := extractAssistantText(m); text == "好的，我已了解之前的对话历史。请继续。" {
					marker = " [摘要]"
				}
				parts := []string{}
				if text := extractAssistantText(m); text != "" {
					parts = append(parts, text)
				}
				for _, call := range extractToolCalls(m) {
					parts = append(parts, fmt.Sprintf("🔧 %s(%s)", call.Name, truncate(call.Args, 40)))
				}
				content = strings.Join(parts, " | ")
				if content == "" {
					content = "(无内容)"
				}
				content = truncate(content, 80)
			case core.ToolResultMessage:
				role = "🔧"
				result := formatContentBlocks(m.Content)
				content = fmt.Sprintf("%s: %s", m.ToolName, truncate(result, 60))
			}
			fmt.Fprintf(os.Stderr, "  %d. %s %s%s\n", i+1, role, marker, content)
		}

	case ":context":
		stats := tokenStats.Get()
		fmt.Fprintln(os.Stderr, "\n"+contextmgr.FormatStats(stats))

	case ":compact":
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		newMsgs, err := doCompact(ctx, sessRef.Messages, ctxSettings, model, keyPool, verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "压缩失败: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "✓ 已压缩: %d → %d 消息\n", len(sessRef.Messages), len(newMsgs))
			sessRef.Messages = newMsgs
			tokenStats.Recompute(sessRef.Messages)
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
		fmt.Fprintf(os.Stderr, "✓ Session [%s] 当前 %d 条记录\n", sessRef.Name(), len(sessRef.Messages))

	case ":load":
		if mem != nil {
			if err := mem.Load(); err != nil {
				fmt.Fprintf(os.Stderr, "重新加载记忆失败: %v\n", err)
			} else {
				promptCache.Invalidate()
				fmt.Fprintf(os.Stderr, "✓ 重新加载 %d 条记忆\n", mem.Size())
			}
		}
		fmt.Fprintf(os.Stderr, "✓ Session [%s] 当前 %d 条记录\n", sessRef.Name(), len(sessRef.Messages))

	case ":stats":
		stats := tokenStats.Get()
		fmt.Fprintln(os.Stderr, "\n"+contextmgr.FormatStats(stats))
		if mem != nil {
			fmt.Fprintf(os.Stderr, "🧠 长期记忆: %d 条\n", mem.Size())
		}
		fmt.Fprintf(os.Stderr, "💾 Session [%s]: %d 条记录\n", sessRef.Name(), len(sessRef.Messages))

	case ":keys":
		fmt.Fprintln(os.Stderr, "\n🔑 API Key 池:")
		for _, s := range keyPool.Status() {
			fmt.Fprintf(os.Stderr, "  %s\n", s.String())
		}

	case ":sessions", ":list":
		list, err := ListSessions(sessRef.dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "列出 session 失败: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "\n📁 Sessions (%s) 共 %d 个:\n", sessRef.dir, len(list))
		current := sessRef.Name()
		for _, s := range list {
			marker := ""
			if s.Name == current {
				marker = " ← 当前"
			}
			fmt.Fprintf(os.Stderr, "  %-30s  %6d 字节  %s%s\n",
				s.Name, s.Size, s.Modified.Format("2006-01-02 15:04:05"), marker)
		}

	case ":current":
		fmt.Fprintf(os.Stderr, "当前 session: %s\n", sessRef.Name())
		fmt.Fprintf(os.Stderr, "消息数: %d\n", len(sessRef.Messages))

	case ":open":
		if len(parts) < 2 {
			fmt.Fprintln(os.Stderr, "用法: :open <session 名>")
			return
		}
		newName := parts[1]
		if newName == sessRef.Name() {
			fmt.Fprintf(os.Stderr, "已经在 %s 中\n", newName)
			return
		}
		if err := sessRef.Switch(newName); err != nil {
			fmt.Fprintf(os.Stderr, "切换失败: %v\n", err)
			return
		}
		newSess := sessRef.Session()
		if newSess != nil {
			ctx2 := newSess.BuildContext()
			sessRef.Messages = ctx2.Messages
		}
		tokenStats.Recompute(sessRef.Messages)
		fmt.Fprintf(os.Stderr, "✓ 已切换到 [%s]，消息数: %d\n", sessRef.Name(), len(sessRef.Messages))

	case ":new":
		var newName string
		if len(parts) >= 2 {
			newName = parts[1]
		} else {
			newName = fmt.Sprintf("session-%s", time.Now().Format("20060102-150405"))
		}
		if err := sessRef.Switch(newName); err != nil {
			fmt.Fprintf(os.Stderr, "创建失败: %v\n", err)
			return
		}
		sessRef.Messages = nil
		tokenStats.Recompute(sessRef.Messages)
		fmt.Fprintf(os.Stderr, "✓ 新建并切换到 [%s]\n", sessRef.Name())

	case ":view":
		if len(parts) < 2 {
			fmt.Fprintln(os.Stderr, "用法: :view <session 名>")
			return
		}
		viewName := parts[1]
		viewPath := filepath.Join(sessRef.dir, viewName+".jsonl")
		msgs, err := LoadSessionMessages(viewPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取 %s 失败: %v\n", viewName, err)
			return
		}
		fmt.Fprintf(os.Stderr, "\n👀 查看 session [%s] (%d 条消息):\n", viewName, len(msgs))
		for i, msg := range msgs {
			role := "?"
			content := ""
			switch m := msg.(type) {
			case core.UserMessage:
				role = "🧑"
				content = truncate(fmt.Sprintf("%v", m.Content), 80)
			case core.AssistantMessage:
				role = "🤖"
				content = truncate(extractAssistantText(m), 80)
			case core.ToolResultMessage:
				role = "🔧"
				content = m.ToolName
			}
			fmt.Fprintf(os.Stderr, "  %d. %s %s\n", i+1, role, content)
		}

	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", parts[0])
		fmt.Fprintln(os.Stderr, "可用: :history :context :compact :memory :remember :forget :save :load :stats :keys :quit")
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
			fmt.Fprintf(os.Stderr, "[thinking] %s", ae.Delta)
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
		if e.Message.StopReason == core.StopError {
			if e.Message.ErrorMessage != "" {
				fmt.Fprintf(os.Stderr, "\nError: %s\n", e.Message.ErrorMessage)
			} else {
				fmt.Fprintf(os.Stderr, "\nError: 请求失败（未提供错误详情）\n")
			}
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

// newLLMExtractor 创建 LLM 提取器，从 keyPool 拿 key 调用 LLM 同步获取响应。
func newLLMExtractor(model core.Model, keyPool *keypool.Pool, verbose bool) autolearn.Extractor {
	return &autolearn.LLMSimpleExtractor{
		SummarizeFunc: func(ctx context.Context, prompt string) (string, error) {
			key, err := keyPool.Next()
			if err != nil {
				return "", err
			}
			opts := core.SimpleStreamOptions{}
			opts.APIKey = key
			opts.MaxRetries = 1
			opts.MaxRetryDelayMs = 5000

			if verbose {
				fmt.Fprintln(os.Stderr, "[extract] 调用 LLM 提取记忆...")
			}

			msgs := []core.Message{
				core.UserMessage{
					Role:    "user",
					Content: prompt,
				},
			}
			resp, err := llm.CompleteSimple(ctx, model, msgs, opts)
			if err != nil {
				keyPool.MarkFailedByKey(key, keypool.CategorizeError(err))
				return "", err
			}
			keyPool.MarkSuccessByKey(key)

			// 提取文本
			var text string
			for _, b := range resp.Content {
				if c, ok := b.(core.TextContent); ok {
					text += c.Text
				}
			}
			return text, nil
		},
	}
}

// newWorkflowExtractor 创建工作流提取器，复用与 LLM 提取相同的 LLM 调用通道。
func newWorkflowExtractor(model core.Model, keyPool *keypool.Pool, verbose bool) *autolearn.WorkflowExtractor {
	return &autolearn.WorkflowExtractor{
		SummarizeFunc: func(ctx context.Context, prompt string) (string, error) {
			key, err := keyPool.Next()
			if err != nil {
				return "", err
			}
			opts := core.SimpleStreamOptions{}
			opts.APIKey = key
			opts.MaxRetries = 1
			opts.MaxRetryDelayMs = 5000

			if verbose {
				fmt.Fprintln(os.Stderr, "[workflow] 调用 LLM 提取工作流...")
			}

			msgs := []core.Message{
				core.UserMessage{
					Role:    "user",
					Content: prompt,
				},
			}
			resp, err := llm.CompleteSimple(ctx, model, msgs, opts)
			if err != nil {
				keyPool.MarkFailedByKey(key, keypool.CategorizeError(err))
				return "", err
			}
			keyPool.MarkSuccessByKey(key)

			var text string
			for _, b := range resp.Content {
				if c, ok := b.(core.TextContent); ok {
					text += c.Text
				}
			}
			return text, nil
		},
	}
}

// maybeCompact 在超出软限制时自动压缩。
// 返回 (new messages, true if compacted)
func maybeCompact(
	ctx context.Context,
	messages []core.Message,
	tokenStats *contextmgr.TokenStats,
	settings contextmgr.Settings,
	model core.Model,
	keyPool *keypool.Pool,
	verbose bool,
) ([]core.Message, bool) {
	if !tokenStats.ShouldCompact() {
		return messages, false
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "\n⚠️  上下文达到 %d tokens, 触发自动压缩...\n", tokenStats.Tokens())
	}

	newMsgs, err := doCompact(ctx, messages, settings, model, keyPool, verbose)
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
	keyPool *keypool.Pool,
	verbose bool,
) ([]core.Message, error) {
	key, kerr := keyPool.Next()
	if kerr != nil {
		return nil, kerr
	}
	opts := core.SimpleStreamOptions{}
	opts.APIKey = key
	result, err := contextmgr.Compact(ctx, model, messages, settings, opts)
	if err != nil {
		keyPool.MarkFailedByKey(key, keypool.CategorizeError(err))
		return nil, err
	}
	keyPool.MarkSuccessByKey(key)
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

// formatContentBlocks 把任意 []ContentBlock 格式化为可读字符串。
func formatContentBlocks(blocks []core.ContentBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		switch c := b.(type) {
		case core.TextContent:
			parts = append(parts, c.Text)
		case core.ThinkingContent:
			parts = append(parts, "[思考]"+c.Thinking)
		default:
			parts = append(parts, fmt.Sprintf("[%T]", b))
		}
	}
	return strings.Join(parts, " ")
}

// envBool 解析布尔环境变量。
func envBool(key string, def bool) bool {
	v := strings.ToLower(os.Getenv(key))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

// envInt 解析整数环境变量。
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func extractAssistantText(m core.AssistantMessage) string {
	var parts []string
	for _, b := range m.Content {
		switch c := b.(type) {
		case core.TextContent:
			parts = append(parts, c.Text)
		case core.ThinkingContent:
			parts = append(parts, "[思考]"+c.Thinking)
		}
	}
	return strings.Join(parts, "")
}

// toolCallInfo 工具调用信息（用于 :history 显示）。
type toolCallInfo struct {
	Name string
	Args string
}

// extractToolCalls 从 AssistantMessage 中提取工具调用信息。
func extractToolCalls(m core.AssistantMessage) []toolCallInfo {
	var calls []toolCallInfo
	for _, b := range m.Content {
		if c, ok := b.(core.ToolCall); ok {
			// c.Arguments 是 json.RawMessage（[]byte）
			args := string(c.Arguments)
			calls = append(calls, toolCallInfo{Name: c.Name, Args: args})
		}
	}
	return calls
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

// collectAPIKeys 收集 API key 列表。
// 优先级：-api-keys CLI > API_KEYS env > -api-key CLI > API_KEY env > {PROVIDER}_API_KEY env
// -api-keys 和 API_KEYS 都支持逗号分隔多个 key。
func collectAPIKeys(apiKeysFlag, apiKeyFlag, provider string, verbose bool) []string {
	keys := []string{}

	// 1. -api-keys CLI
	if apiKeysFlag != "" {
		for _, k := range strings.Split(apiKeysFlag, ",") {
			if k = strings.TrimSpace(k); k != "" {
				keys = append(keys, k)
			}
		}
		if len(keys) > 0 {
			if verbose {
				fmt.Fprintf(os.Stderr, "[keypool] loaded %d key(s) from -api-keys\n", len(keys))
			}
			return keys
		}
	}

	// 2. API_KEYS env
	if envKeys := os.Getenv("API_KEYS"); envKeys != "" {
		for _, k := range strings.Split(envKeys, ",") {
			if k = strings.TrimSpace(k); k != "" {
				keys = append(keys, k)
			}
		}
		if len(keys) > 0 {
			if verbose {
				fmt.Fprintf(os.Stderr, "[keypool] loaded %d key(s) from API_KEYS env\n", len(keys))
			}
			return keys
		}
	}

	// 3. 兼容单 key：-api-key / API_KEY / {PROVIDER}_API_KEY
	single := apiKeyFlag
	if single == "" {
		single = os.Getenv("API_KEY")
		if single == "" && provider != "" {
			single = os.Getenv(strings.ToUpper(provider) + "_API_KEY")
		}
	}
	if single != "" {
		keys = append(keys, strings.TrimSpace(single))
	}
	return keys
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
