# Go-Copilot 技术文档

> **版本**: v0.1  
> **状态**: 草案  
> **读者**: 架构师、前端/后端/全栈开发者  

---

## 1. 系统架构

### 1.1 架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│                          CLI (main.go)                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │  单次查询模式  │  │  REPL 交互   │  │  交互命令处理器       │  │
│  │  -query "xx" │  │  scanner     │  │  :history :compact   │  │
│  └──────┬───────┘  └──────┬───────┘  │  :memory :keys ...   │  │
│         │                 │           └──────────┬───────────┘  │
│         └────────┬────────┘                      │              │
│                  │                               │              │
│          ┌───────▼────────┐              ┌───────▼───────────┐  │
│          │   AgentRunner  │              │   Session 管理     │  │
│          │ (消息编排引擎)  │              │ SessionRef /       │  │
│          │                │              │ SessionFlusher     │  │
│          └───────┬────────┘              └───────────────────┘  │
│                  │                                               │
│   ┌──────────────┼──────────────┬──────────────┬──────────────┐ │
│   │              │              │              │              │ │
│   ▼              ▼              ▼              ▼              ▼ │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────┐│
│ │Skills    │ │Memory    │ │Context   │ │KeyPool   │ │LLM     ││
│ │Manager   │ │Manager   │ │Manager   │ │          │ │Client  ││
│ │          │ │          │ │          │ │多Key轮询  │ │        ││
│ │SKILL.md  │ │JSON KV   │ │Token估算  │ │故障转移  │ │Claude  ││
│ │加载/注入  │ │持久化    │ │自动压缩  │ │冷却管理  │ │GPT/... ││
│ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └────────┘│
│                                                                 │
│   ┌──────────────────────────────────────────────────────────┐ │
│   │                   pi-ai-go 框架层                         │ │
│   │  core (消息类型) │ llm (LLM调用) │ agent (AgentLoop)     │ │
│   │  agent/session  │ agent/tools   │ providers (多厂商)     │ │
│   └──────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### 1.2 请求生命周期

```
用户输入
  │
  ▼
┌──────────────────────────────────────────────────────────────┐
│ 1. 接收输入 (scan / -query)                                   │
│    ├─ 以 : 开头 → 交互命令处理 → 返回等待输入                    │
│    └─ 否则 → 进入 LLM 处理流程                                 │
└───────────────────────┬──────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────────────┐
│ 2. 自动记忆提取 (AutoLearner.ProcessUserInput)               │
│    匹配 [remember:key=val] / 请记住：key=val 等标记            │
│    └─ 如果匹配 → 写入 Memory → 失效 PromptCache               │
└───────────────────────┬──────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────────────┐
│ 3. 上下文窗口检查 (TokenStats.ShouldCompact)                  │
│    ├─ 超过软限制(70%) → LLM 摘要早期消息 → 替换为摘要注入       │
│    └─ 未超过 → 直接使用当前消息列表                             │
└───────────────────────┬──────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────────────┐
│ 4. 获取 API Key (KeyPool.Next)                                │
│    ├─ cursor 位置 key 可用 → 返回                              │
│    ├─ 轮询找下一个 available key                               │
│    └─ 全部 cooldown → 返回 ErrNoAvailableKey                  │
└───────────────────────┬──────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────────────┐
│ 5. 调用 LLM (AgentLoopDetailed)                               │
│    ├─ SystemPrompt: 基础指令 + Skills + Memory                │
│    ├─ Messages: 历史消息(含可能的摘要) + 新用户消息              │
│    ├─ Tools: pi-ai-go 工具集                                  │
│    └─ 流式输出 → 边接收边打印                                   │
└───────────────────────┬──────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────────────┐
│ 6. 后处理                                                     │
│    ├─ 成功 → KeyPool.MarkSuccess / 更新 session               │
│    ├─ 失败 → KeyPool.MarkFailed + 分类错误类型                  │
│    ├─ 追加消息到 SessionFlusher (异步后台写入)                  │
│    └─ TokenStats 同步更新                                     │
└──────────────────────────────────────────────────────────────┘
```

---

## 2. 技术选型

| 技术 | 选型 | 理由 |
|------|------|------|
| **语言** | Go 1.21+ | 与 `pi-ai-go` 一致，天然适合 CLI 工具 |
| **LLM 框架** | `pi-ai-go` | 本项目的上游框架，提供 AgentLoop、Session、Provider 等能力 |
| **持久化** | JSON + JSONL | 人类可读，方便调试和手动编辑 |
| **Token 估算** | 自研（CJK 感知） | 无需引入 tiktoken 等重量依赖 |
| **并发模型** | goroutine + channel | Go 标准并发模式 |
| **配置** | `.env` + CLI flags | 开发体验优先 |
| **测试** | Go testing + testify | 标准方案 |

---

## 3. 核心模块设计

### 3.1 Agent 引擎

**文件**: `main.go`

Agent 引擎是整个工具的编排层，负责连接所有子模块。入口函数 `main()` 按以下顺序初始化：

```
加载 .env → 解析 CLI 参数 → 收集 API Keys → 并行加载(Skills + Memory + Session)
→ 构建 SystemPromptCache → 配置 AgentLoop → 加载历史消息
→ 初始化 AutoLearner → 进入查询/交互循环
```

**关键设计决策**:

1. **并行加载**: Skills、Memory、Session 在单个 goroutine 中加载（相对于 LLM 调用），但 Skills 文本格式化是同步的。未来可以完全并行化这三个 I/O 操作。

2. **配置分离**: `AgentLoopConfig` 是静态配置（Model、SystemPrompt、Tools），而 API Key 每次调用前动态注入。这允许 KeyPool 无缝替换。

3. **双模式**: `runSingleQuery` 和 `runInteractive` 共享核心逻辑但入口不同。单次模式有 `context.WithTimeout`，交互模式无超时。

### 3.2 Skills 系统

**依赖**: `pi-ai-go/agent/session` 包 (LoadSkills, FormatSkillsForSystemPrompt)

Skills 是 Go-Copilot 的核心差异化能力。系统从目录递归加载 Trae 风格的 `SKILL.md` 文件，将其内容注入到 system prompt 中。

```
┌───────────────┐     ┌────────────────────┐     ┌───────────────────┐
│ ./skills/     │────►│ LoadSkills(dir)     │────►│ FormatSkillsFor   │
│ ├─ go-best/   │     │ 扫描所有 SKILL.md   │     │ SystemPrompt()    │
│ │  SKILL.md   │     │ 解析 frontmatter    │     │ 格式化为 prompt    │
│ ├─ project/   │     │ 返回 []Skill + 诊断  │     │ 文本               │
│ │  SKILL.md   │     └────────────────────┘     └─────────┬─────────┘
│ ...           │                                         │
└───────────────┘                          ┌──────────────▼──────────┐
                                           │ buildSystemPrompt()     │
                                           │ "You are a helpful..."  │
                                           │ + skillsText            │
                                           │ + memory.FormatFor...   │
                                           └─────────────────────────┘
```

**SKILL.md 格式** (Trae 兼容):

```markdown
---
name: go-best-practices
description: Go 语言最佳实践
---

# Go 编码规范

## 错误处理
- 不要使用 panic 替代 error 返回
- 使用 fmt.Errorf("...: %w", err) 包装错误

## 并发
- 使用 context 控制 goroutine 生命周期
- channel 关闭由发送方负责
```

**设计要点**:
- 技能文本是纯 Markdown，LLM 可直接理解
- 多个 Skill 按加载顺序拼接
- Skill 变更需要重启才能生效（P1 规划热加载）
- 技能通过 `promptCache` 缓存，Hash 不变时不重新格式化

### 3.3 Memory 系统 (长期记忆)

**包**: `examples/agent-with-skills/memory`

#### 数据模型

```go
type Memory struct {
    mu   sync.RWMutex
    path string           // 持久化文件路径
    data map[string]Entry // 内存中的键值对
}

type Entry struct {
    Value     string    `json:"value"`
    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
    Category  string    `json:"category,omitempty"`
}
```

#### 持久化格式 (memory.json)

```json
{
  "user.name": {
    "value": "小明",
    "createdAt": "2025-01-01T10:00:00Z",
    "updatedAt": "2025-01-01T10:00:00Z",
    "category": "user"
  },
  "project.code-style": {
    "value": "slog-only",
    "createdAt": "2025-01-01T10:05:00Z",
    "updatedAt": "2025-01-01T10:05:00Z",
    "category": "preference"
  }
}
```

#### 原子写入

Save 使用 **先写临时文件再 rename** 的策略保证原子性：

```go
tmp, _ := os.CreateTemp(dir, ".memory-*.tmp")
tmp.Write(data)
tmp.Close()
os.Rename(tmpPath, m.path)  // 原子替换
```

#### Hash 缓存集成

```go
// SystemPromptCache.Get 检查 memory hash
func (c *SystemPromptCache) Get(skillsText string, mem *memory.Memory) string {
    memKey := mem.Hash()  // 基于所有 Entry.UpdatedAt 拼接
    if c.cached != "" && c.memKey == memKey {
        return c.cached  // 命中缓存
    }
    // 重建...
}
```

当 Memory 发生变更（Set/Delete/自动学习）时，调用 `promptCache.Invalidate()` 强制下次重建。

### 3.4 KeyPool 系统

**包**: `examples/agent-with-skills/keypool`

#### 状态机

```
     ┌──────────┐
     │Available │◄────────────────────────────┐
     └────┬─────┘                             │
          │                                   │
    API调用失败(非429)                  冷却时间到期
    API调用失败(429)                         │
          │                                   │
    ┌─────▼──────┐   冷却时间到期    ┌─────────┴─────┐
    │  Cooldown   │────────────────►│  Available     │
    │  (60s)      │                 │               │
    └─────────────┘                 └───────────────┘
          │
    API调用失败(429)
          │
    ┌─────▼──────┐   冷却时间到期    ┌───────────────┐
    │RateLimited │────────────────►│  Available     │
    │  (120s)    │                 │               │
    └────────────┘                 └───────────────┘
```

#### 轮询算法

```go
func (p *Pool) Next() (string, error) {
    // 1. 周期重置：距上次 Next 超过 CycleReset → cursor=0
    // 2. 清理过期 cooldown
    // 3. 从 cursor 开始找第一个 Available
    // 4. cursor 推进到 (idx+1) % len，为下次准备
    // 5. 全部不可用 → ErrNoAvailableKey
}
```

#### 错误分类

```go
func CategorizeError(err error) FailureKind {
    // 基于错误消息中的关键字启发式匹配
    // 401/403 → FailureAuth (60s cooldown)
    // 429     → FailureRate (120s cooldown)
    // 5xx     → FailureServer (60s cooldown)
    // timeout → FailureNetwork (60s cooldown)
    // 其他    → FailureUnknown (60s cooldown)
}
```

#### Key 来源优先级

```
-api-keys CLI > API_KEYS env > -api-key CLI > API_KEY env > {PROVIDER}_API_KEY env
```

多 key 以逗号分隔：`-api-keys "sk-ant-xxx,sk-ant-yyy,sk-ant-zzz"`

### 3.5 Context 管理器

**包**: `examples/agent-with-skills/contextmgr`

#### Token 估算算法

```go
func estimateStringTokens(s string) int {
    // CJK字符 (0x4E00-0x9FFF): ~1.5 字符/token → count * 2/3
    // ASCII字符:             ~4 字符/token   → count / 4
    return cjkCount*2/3 + otherCount/4
}
```

#### 压缩策略

```
上下文窗口 = 200,000 (Claude) / 128,000 (GPT-4o) / 64,000 (DeepSeek)
可用 = MaxContext - ReservedForResponse(4096)

软限制 = 可用 * 0.7  → 达到后自动触发 LLM 摘要压缩
硬限制 = 可用 * 0.95 → 达到后强制截断最早消息
```

#### 压缩算法 (`Compact`)

```
1. 拆分: messages[0:splitIdx] → 待摘要, messages[splitIdx:] → 保留
2. 串行化待摘要消息 (SerializeMessagesForSummary)
3. 调用 LLM: "请简洁地总结以下对话历史..."
4. 构造新消息列表:
   [摘要UserMessage] + [AckAssistantMessage] + [保留消息]
```

**性能优化**:
- 摘要过程被放入 goroutine，与保留消息构造并发
- 增量 TokenStats 避免每轮 O(N) 重算

### 3.6 LLM 集成层

| Provider | API | 模型示例 | 上下文窗口 |
|----------|-----|---------|-----------|
| Anthropic | Anthropic Messages | claude-sonnet-4-20250514 | 200,000 |
| OpenAI | OpenAI Completions | gpt-4o | 128,000 |
| DeepSeek | OpenAI Completions | deepseek-chat | 64,000 |
| Groq | OpenAI Completions | - | - |
| Fireworks | OpenAI Completions | - | - |
| Together | OpenAI Completions | - | - |
| Cerebras | OpenAI Completions | - | - |
| Google | Google Generative | - | - |
| Google Vertex | Google Vertex | - | - |
| Mistral | Mistral Conversations | - | - |
| Azure OpenAI | Azure OpenAI Responses | - | - |
| OpenRouter | OpenRouter | - | - |
| Amazon Bedrock | Bedrock Converse | - | - |

---

## 4. API 接口文档

Go-Copilot 本身是 CLI 工具，不暴露 HTTP API。其内部接口如下：

### 4.1 Memory API

```go
// 创建/加载
mem, err := memory.New("./memory.json")

// 读写
mem.Set("key", "value")
mem.SetWithCategory("key", "value", "category")
val, ok := mem.Get("key")
mem.Delete("key")

// 查询
keys := mem.Keys()
items := mem.ListByCategory("user")
size := mem.Size()
exists := mem.Has("key")

// 持久化
mem.Save()   // 原子写入 (tmp → rename)
mem.Load()   // 从磁盘重新加载

// System Prompt
text := mem.FormatForPrompt()
hash := mem.Hash()
```

### 4.2 KeyPool API

```go
pool := keypool.New(keys, keypool.DefaultSettings())

key, err := pool.Next()          // 获取下一个可用 key
pool.MarkSuccess()               // 标记成功
pool.MarkFailed(keypool.FailureRate) // 标记失败
pool.MarkFailedByKey(key, kind)  // 标记指定 key 失败

kind := keypool.CategorizeError(err) // 错误分类

statuses := pool.Status()        // 查看所有 key 状态
```

### 4.3 ContextMgr API

```go
settings := contextmgr.DefaultSettings("claude-sonnet-4-20250514")
stats := contextmgr.NewTokenStats(settings)

stats.Add(msg)                   // 增量添加消息（O(1)）
stats.AddMany(msgs)              // 批量添加
stats.Recompute(messages)        // 完全重算
stats.ShouldCompact()            // 是否应触发压缩
tokens := stats.Tokens()         // 当前 token 数

// 压缩
result, err := contextmgr.Compact(ctx, model, messages, settings, opts)
// result.NewMessages 是压缩后的消息列表
```

---

## 5. 数据模型

### 5.1 核心类型

```go
// 消息类型（来自 pi-ai-go/core）
type UserMessage struct {
    Role      string
    Content   interface{}   // string 或 []ContentBlock
    Timestamp time.Time
}

type AssistantMessage struct {
    Role    string
    Content []ContentBlock
    Usage   Usage
}

type ToolResultMessage struct {
    Role     string
    ToolName string
    Content  []ContentBlock
}

// ContentBlock 子类型
type TextContent struct {
    Type string
    Text string
}

type ToolCall struct {
    Name      string
    Arguments json.RawMessage
}

type ThinkingContent struct {
    Thinking string
}
```

### 5.2 Session 持久化格式 (JSONL)

每行一条记录：

```jsonl
{"id":"abc123","type":"message","timestamp":"2025-01-01T10:00:00Z","message":{"role":"user","content":"你好","timestamp":"2025-01-01T10:00:00Z"}}
{"id":"def456","type":"message","timestamp":"2025-01-01T10:00:05Z","message":{"role":"assistant","content":[{"type":"text","text":"你好！..."}],"timestamp":"2025-01-01T10:00:05Z"}}
```

### 5.3 AgentLoop 事件类型

```go
// 流式事件（来自 pi-ai-go/agent）
type EventMessageUpdate struct {
    AssistantEvent interface{} // TextDelta / ThinkingDelta / ToolCallStart / ...
}

type EventToolExecStart struct {
    ToolName string
    Args     json.RawMessage
}

type EventToolExecEnd struct {
    ToolName string
    IsError  bool
}

type EventTurnEnd struct {
    ToolResults []ToolResultMessage
}

type EventAgentEnd struct{}
```

---

## 6. CLI 设计

### 6.1 命令参考

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `-skills` | string | `$SKILLS` | Skills 目录路径 |
| `-model` | string | `$MODEL` | 模型 ID |
| `-provider` | string | `$PROVIDER` | Provider 名称 |
| `-base-url` | string | `$BASE_URL` | API Base URL (代理/中转) |
| `-api-key` | string | - | 单个 API Key (兼容旧版) |
| `-api-keys` | string | `$API_KEYS` | 逗号分隔的多 Key (优先) |
| `-query` | string | - | 单次查询 (有值则非交互模式) |
| `-session` | string | `$SESSION` | JSONL session 文件路径 |
| `-memory` | string | `$MEMORY` | 长期记忆文件路径 |
| `-auto-compact` | bool | `true` | 启用自动压缩 |
| `-v` | bool | `false` | 详细日志 |

### 6.2 交互命令

| 命令 | 参数 | 说明 |
|------|------|------|
| `:help` | - | 显示帮助 |
| `:history` | - | 显示对话历史（含摘要标记） |
| `:context` | - | Token 使用情况 + 可视化进度条 |
| `:compact` | - | 手动触发 LLM 摘要压缩 |
| `:memory` | - | 列出所有长期记忆 |
| `:remember` | `key = value` | 添加/更新记忆 |
| `:forget` | `key` | 删除记忆 |
| `:save` | - | 手动保存 (memory + session 状态) |
| `:load` | - | 从磁盘重新加载记忆 |
| `:stats` | - | 完整统计 (tokens + memory + session) |
| `:keys` | - | API Key 池状态 |
| `:sessions` / `:list` | - | 列出所有 session 文件 |
| `:current` | - | 显示当前 session 名称和消息数 |
| `:open` | `name` | 切换到指定 session |
| `:new` | `[name]` | 新建 session 并切换 |
| `:view` | `name` | 只读查看 session 内容 |
| `:quit` / `:exit` | - | 退出 |

---

## 7. 配置与部署

### 7.1 环境配置 (.env)

```env
# LLM 配置
PROVIDER=anthropic
MODEL=claude-sonnet-4-20250514
BASE_URL=https://api.anthropic.com

# API Key (支持多 Key，逗号分隔)
API_KEYS=sk-ant-xxx,sk-ant-yyy

# 持久化路径
SESSION=./session.jsonl
MEMORY=./memory.json

# Skills 目录
SKILLS=./skills
```

### 7.2 快速部署

```bash
# 1. 克隆项目
git clone https://github.com/pi-ai-go/pi-ai-go.git
cd pi-ai-go/examples/agent-with-skills

# 2. 配置环境
cp .env.example .env
# 编辑 .env 填入 API Key

# 3. 安装依赖
go mod download

# 4. 运行
go run . -skills ./skills -memory ./memory.json

# 可选：编译为二进制
go build -o go-copilot .
./go-copilot -skills ./skills -memory ./memory.json
```

### 7.3 Skills 目录结构

```
skills/
├── go-best-practices/
│   └── SKILL.md          # Go 最佳实践
├── project-conventions/
│   └── SKILL.md          # 项目约定
└── custom/
    └── SKILL.md          # 自定义技能
```

每个 Skill 目录下必须包含一个 `SKILL.md` 文件。

---

## 8. 安全设计

### 8.1 API Key 保护

| 措施 | 实现 |
|------|------|
| 内存脱敏 | `maskKey()` 显示时只保留前 4 和后 4 位 |
| 日志脱敏 | 调试输出中使用脱敏后的 key |
| 文件隔离 | `.env` 不提交到 Git (加入 `.gitignore`) |
| 环境变量 | 支持从环境变量读取，不会出现在命令行历史中 |

### 8.2 数据隐私

| 方面 | 策略 |
|------|------|
| Memory 存储 | 本地 JSON 文件，不主动上传 |
| Session 存储 | 本地 JSONL 文件，不主动上传 |
| LLM 通信 | 通过用户自己的 API Key，数据发送至对应 LLM 服务商 |
| 对话内容 | 仅在用户的 API 调用中传输，无中间服务器 |

### 8.3 文件操作安全

| 措施 | 说明 |
|------|------|
| 工具调用透传 | 文件操作通过 pi-ai-go 的标准工具集，由用户确认 |
| 只读查看 | `:view` 命令只读加载 session，不修改 |
| 原子写入 | Memory 保存使用 tmp→rename，不会出现半写状态 |
| 优雅关闭 | `SessionRef.Close()` 确保退出前 flush 所有缓冲数据 |

---

## 9. 性能优化清单

| 优化点 | 位置 | 技术 | 效果 |
|--------|------|------|------|
| 增量 Token 统计 | `TokenStats.Add/AddMany` | 只计算新增消息 | O(1) vs O(N) |
| System Prompt 缓存 | `SystemPromptCache` | Hash 比对复用 | 避免每轮重复格式化 |
| 异步 Session 写入 | `SessionFlusher` | 后台批量 fsync | 不阻塞主流程 |
| 并行启动加载 | `loadResult` goroutine | Skills/Memory/Session 并发 | 启动更快 |
| LLM 摘要并发 | `go func()` | 串行化与 LLM 调用并发 | 压缩延迟减半 |
| 流式输出 | `stream.ForEach` | SSE 逐块接收打印 | 首 token 即见 |
| KeyPool 故障转移 | 自动跳过冷却 key | 多 Key 分摊限制 | 提高可用性 |
| 原子文件写入 | `os.CreateTemp + Rename` | 避免半写状态 | 数据安全 |

---

## 10. 包依赖关系

```
examples/agent-with-skills/
├── main.go           → 依赖: pi-ai-go/agent, pi-ai-go/llm, pi-ai-go/core,
│                               pi-ai-go/agent/session, pi-ai-go/agent/tools,
│                               pi-ai-go/providers,
│                               ./memory, ./contextmgr, ./autolearn, ./keypool
├── memory/           → 依赖: 标准库 (encoding/json, os, sync)
├── contextmgr/       → 依赖: pi-ai-go/agent/session, pi-ai-go/core, pi-ai-go/llm
├── autolearn/        → 依赖: pi-ai-go/core, ./memory
└── keypool/          → 依赖: 标准库 (sync, time)
```

**外部依赖**:
- `pi-ai-go/core` - 消息类型、Stream、Model
- `pi-ai-go/llm` - CompleteSimple
- `pi-ai-go/agent` - AgentLoopDetailed、AgentEvent
- `pi-ai-go/agent/session` - Session、JSONLStorage、LoadSkills
- `pi-ai-go/agent/tools` - 工具集 All()
- `pi-ai-go/providers` - Provider 注册
