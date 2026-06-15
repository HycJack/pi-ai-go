# Crux vs pi-ai-go 对比分析

本文档对比分析 `crux/crux` 仓库与 `pi-ai-go` 仓库的设计理念、架构差异、各自优势及可借鉴点。

## 一、总体规模

| 指标 | Crux | pi-ai-go |
|------|------|----------|
| 模块数 | 3 (crux-agent-runtime + crux-harness + crux-chat) | 单体 (agent + providers + llm) |
| Go 代码行数 | ~24,500 | ~14,000 |
| 测试代码 | 大量（每个包都有 `_test.go`） | 较少 |
| 架构 | 三层分离 | 单体包 |
| 依赖方向 | chat → harness → runtime → ai | agent → llm → providers → core |

## 二、架构对比

```
┌─────────────────────────────────────────────────────────────┐
│  Crux 三层架构                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │ crux-    │→ │ crux-    │→ │ crux-    │                  │
│  │ chat     │  │ harness  │  │ runtime  │                  │
│  │ (产品层) │  │ (治理层) │  │ (执行层) │                  │
│  └──────────┘  └──────────┘  └──────────┘                  │
│       ↑             ↑             ↑                          │
│    用户/UI       策略/审计     LLM/Tool                      │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│  pi-ai-go 单体架构                                          │
│  ┌────────────────────────────────────────────┐             │
│  │  agent (含 loop/session/compaction/queue)  │             │
│  │         ↑                                  │             │
│  │  llm + providers (OpenAI/Anthropic/...)    │             │
│  └────────────────────────────────────────────┘             │
└─────────────────────────────────────────────────────────────┘
```

## 三、核心设计差异

| 维度 | Crux | pi-ai-go |
|------|------|----------|
| **分层** | Runtime / Harness / Chat 三层 | agent 包 + llm 包 |
| **治理** | 独立的 `crux-harness` 模块（policy/dispatch/approval/hooks/turn） | 内嵌在 agent 包中（BeforeToolCall/AfterToolCall） |
| **状态机** | Turn FSM（8 个状态：received→streaming→dispatching→awaiting_approval→...→completed） | 两层循环（inner: tool calls, outer: follow-up） |
| **事件系统** | AgentEvent + 多 listener 订阅 | EventStream[T, R] 泛型 + subscriber |
| **持久化** | SQLite + JSONL 两种 store | JSONL + Memory 两种 storage |
| **压缩** | Compactor 接口（SlideWindow/LLMSummarize） | ContextPolicy + CompactByStrategy |
| **前缀缓存** | StablePrefix + AppendOnlyLog（sha256 fingerprint） | 无显式优化 |
| **并发控制** | TurnHarness + approval.Service | Yielder + context cancel |
| **消息类型** | 单结构 Message（带 Reasoning 字段） | 类型化联合 UserMessage/AssistantMessage/ToolResultMessage |
| **多模态** | ContentPart + ImageURL（text/image_url） | ContentBlock 接口（text/thinking/tool_call/image） |
| **Provider 抽象** | Provider 接口（Name/Stream/Complete），每个协议独立子包 | core.APIProvider 接口，注册到全局 Registry |
| **配置管理** | YAML 配置文件（cruxd-config.yaml） | 纯 Go struct（AgentLoopConfig） |
| **API 暴露** | HTTP API + SSE 流式（/v1/turns、/v1/approvals、/v1/metrics） | 仅 Go 库，无 HTTP 层 |

## 四、关键文件对比

| 关注点 | Crux 文件 (行数) | pi-ai-go 文件 (行数) |
|--------|------------------|----------------------|
| Agent Loop | `agentloop/loop.go` (119) | `agent/agent-loop.go` (762) |
| 高层 Agent | `agent/agent.go` (856) | `agent/agent.go` (282) |
| Session | `agent/session.go` (326) | `agent/agent.go` (Run/RunContinue) |
| Compaction | `agent/compaction.go` (245) | `agent/compaction.go` (212) + `session/compaction.go` |
| 前缀缓存 | `agent/append_only_context.go` (335) | 无 |
| Turn FSM | `turn/turn.go` + `turn/states.go` | 无（用循环替代） |
| 治理 | `policy/policy.go` + `dispatch/dispatch.go` + `approval/approval.go` | `BeforeToolCall/AfterToolCall` 钩子 |
| 类型定义 | `crux-ai/types.go`（单结构） | `core/types.go`（类型化联合） |
| Provider | `crux-ai/providers/*`（每个独立包） | `providers/{openai,anthropic,...}` |
| 工具 | `agent/agent.go`（AgentTool） | `agent/tools/*.go` + `tools/tools.go` |
| 存储 | `harness/store/sqlite.go` + `harness/session/session.go` | `agent/session/jsonl.go` + `memory.go` |
| 技能 | `harness/skills/*`（terminal/memory/cron/websearch/delegate/compaction） | `agent/session/skills.go`（基础加载） |
| 事件 | `crux-ai` 中 StreamChunk/StreamResponse | `core.EventStream[T,R]` 泛型 |

## 五、各自亮点

### Crux 做得好的地方

#### 1. 极简的 agentloop.Loop（119行）

裸 LLM→tool→repeat 循环，无事件、无治理。`Loop` 只接收 6 个回调，**接口设计干净**：

```go
type Loop struct {
    provider     ai.LLMProvider
    tools        []ai.ToolSchema
    executor     ToolExecutor
    persister    MessagePersister
    compactor    Compactor
    inputGuard   InputGuard
    outputGuard  OutputGuard
    MaxRounds    int
}
```

#### 2. 三层分离架构

Runtime 不知道 policy、approval、audit 的存在，治理完全由 Harness 通过回调注入。**关注点分离彻底**，可独立替换。

#### 3. Turn FSM（8 个状态）

状态机编排器，支持：
- 暂停/恢复
- 审批阻塞（`awaiting_approval`）
- 错误重试

比 pi-ai-go 的两层循环更**适合生产级管控**。

#### 4. StablePrefix + AppendOnlyLog

通过 sha256 fingerprint 锁定 system prompt + tool specs 的字节序列，**最大化 prefix cache 命中率**（OpenAI/Anthropic/DeepSeek 都提供 prefix cache 折扣，可降低 90% token 成本）。

```go
// 检测 prefix 变化
func (s *StablePrefix) Build(systemPrompt string, tools []ai.ToolSchema) {
    // 计算 sha256(systemPrompt + tool JSON)
    // 如果变化则 version++
}
```

#### 5. 完整的治理链

`policy (YAML) → dispatch (chokepoint) → approval (HITL) → hooks (fanout) → turn (FSM)`，**生产级审计、暂停、审批**。

#### 6. Skill 抽象

`terminal/memory/cron/websearch/delegate/compaction` 都是 skill，**可插拔**。

#### 7. SQLite store

支持事务、索引、查询，比纯 JSONL 强大。

#### 8. 多模态 ContentPart

`ContentPart` + `ImageURL` 完整支持 text/image_url 两种 OpenAI 多模态内容。

### pi-ai-go 做得好的地方

#### 1. 类型化消息联合

```go
// core/types.go
type Message interface { ... }
type UserMessage struct { ... }
type AssistantMessage struct { ... }
type ToolResultMessage struct { ... }
```

**编译时类型安全**。Crux 的单结构 Message 用 `Role` 字符串判别，运行时才能发现错误。

#### 2. EventStream 泛型

```go
// core/eventstream.go
type EventStream[T any, R any] struct { ... }
func (s *EventStream[T,R]) ForEach(...) (R, error)
func (s *EventStream[T,R]) Result() (R, error)
```

**可订阅、可 ForEach、可 Result**。Crux 用 channel 自行实现。

#### 3. 消息转换层清晰

```go
// providers/openai/convert/convert.go
func Messages(messages []core.Message, model core.Model) ([]map[string]any, error)
```

`convert.Messages` 负责内部→LLM 格式转换，**provider 可以热替换**。Crux 在每个 provider 包内自行转换。

#### 4. oh-my-pi 兼容性

明确标注 `Inspired by @oh-my-pi/agent`，设计选择有完整注释说明（`agent/compaction.go` 顶部就有 22 行注释说明来源和简化）。

#### 5. OverflowSignal 同时支持两种检测

- **Stream overflow**：provider 返回 4xx + 错误信息
- **Silent overflow**：usage > context window（provider 不报错但实际超限）

#### 6. 更细粒度的事件

`EventMessageStart/Update/End`、`EventToolExecStart/Update/End`、`EventCompaction`、`EventTurnStart/End`、`EventAgentStart/End`，覆盖 agent 生命周期的每个阶段。

#### 7. 更小的表面积

~14k 行 vs 24.5k 行，**学习曲线低、上手快**。

## 六、可借鉴点

| 借鉴项 | 来源 | 价值 | 优先级 |
|--------|------|------|--------|
| **StablePrefix** | Crux | 降低生产环境 token 成本 | 高 |
| **Turn FSM** | Crux | 适合需要审批/暂停的企业场景 | 中 |
| **Policy YAML** | Crux | 治理规则可配置化 | 中 |
| **Approval Service** | Crux | 人类审批工作流 | 中 |
| **SQLite store** | Crux | 生产级持久化（pi-ai-go 只有 JSONL） | 高 |
| **多模态 ContentPart** | Crux | 图像/音频输入 | 中 |
| **YAML 技能系统** | Crux | 可插拔技能 | 低 |

## 七、关键代码示例

### Crux 的极简 Loop

```go
// agentloop/loop.go:70 - 119行极简实现
func (l *Loop) Run(messages []ai.Message) ([]ai.Message, error) {
    for round := 0; round < l.MaxRounds; round++ {
        if l.compactor != nil {
            messages = l.compactor(messages)
        }
        resp, err := l.provider.Complete(messages, l.tools)
        if err != nil {
            return messages, fmt.Errorf("llm error: %w", err)
        }
        messages = append(messages, resp.Message)
        l.persist(resp.Message)
        if len(resp.Message.ToolCalls) == 0 {
            return messages, nil
        }
        for _, call := range resp.Message.ToolCalls {
            l.persist(l.executor(call))
        }
    }
    return messages, nil
}
```

### pi-ai-go 的两层循环

```go
// agent/agent-loop.go:67-234 - 167行
for {  // outer: follow-up
    for {  // inner: tool calls + steering
        // ... yield, inject steering, call LLM, execute tools
        if len(toolCalls) == 0 && !hasSteering { break }
    }
    // ... check follow-up
}
```

### Crux 的 StablePrefix

```go
// agent/append_only_context.go
type StablePrefix struct {
    mu       sync.RWMutex
    snapshot *StablePrefixSnapshot
    version  int
}

type StablePrefixSnapshot struct {
    SystemPrompt string
    Tools        []ai.ToolSchema
    Fingerprint  string  // sha256
}

func (s *StablePrefix) Build(systemPrompt string, tools []ai.ToolSchema) {
    h := sha256.New()
    h.Write([]byte(systemPrompt))
    for _, t := range tools {
        data, _ := json.Marshal(t)
        h.Write(data)
    }
    fp := hex.EncodeToString(h.Sum(nil))
    // 如果与上次相同 → 复用 prefix
    // 如果变化 → version++，触发 cache miss
}
```

### pi-ai-go 的类型化消息

```go
// core/types.go
type Message interface {
    isMessage()
}

type UserMessage struct {
    Role      string
    Content   any  // string 或 []ContentBlock
    Timestamp time.Time
}
func (UserMessage) isMessage() {}

type AssistantMessage struct {
    Role         string
    Content      []ContentBlock
    ToolCalls    []ToolCall
    StopReason   StopReason
    Usage        Usage
    ErrorMessage string
    Timestamp    time.Time
}
func (AssistantMessage) isMessage() {}
```

## 八、总结

| 维度 | 评价 |
|------|------|
| **设计哲学** | Crux 偏 **企业级治理**，pi-ai-go 偏 **开发体验** |
| **学习曲线** | pi-ai-go 简单直接，Crux 需要理解三层架构 |
| **生产就绪** | Crux 更成熟（FSM/Approval/Prefix Cache），pi-ai-go 更轻量 |
| **可扩展性** | Crux 通过 Skill/Policy/Hook 扩展，pi-ai-go 通过 callback 扩展 |
| **代码组织** | Crux 模块化（3 repo），pi-ai-go 单体（但有 session/ 子包） |
| **类型安全** | pi-ai-go 编译时检查，Crux 运行时检查 |
| **持久化** | Crux SQLite 强，pi-ai-go JSONL 够用 |
| **流式 API** | Crux HTTP+SSE，pi-ai-go 仅 Go channel |
| **文档注释** | pi-ai-go 每个文件有来源说明，Crux 也有但稍少 |

**两个框架都受 oh-my-pi 启发**（pi-ai-go 注释明确说明，Crux 注释中也多次提及 `omp`），但走了不同的路：

- **pi-ai-go**：保留 oh-my-pi 的事件流设计，简化部署，单库即用
- **Crux**：把 oh-my-pi 拆成 Runtime + Harness + Chat 三层，引入企业级治理

## 九、选型建议

| 场景 | 推荐 |
|------|------|
| 快速做 Demo / PoC | **pi-ai-go** |
| 单用户 / 内部工具 | **pi-ai-go** |
| 多用户 SaaS | **Crux**（有 HTTP API + 审批 + 审计） |
| 企业级（需合规、暂停、审批） | **Crux**（有 Turn FSM + Policy + Approval） |
| 大量调用（降本） | **Crux**（有 StablePrefix prefix cache） |
| 嵌入到现有 Go 项目 | **pi-ai-go**（库形态） |
| 独立部署服务 | **Crux**（daemon 形态） |

## 十、参考资料

- Crux 仓库：`/mnt/workspace/crux/crux/`
  - `crux-agent-runtime/agentloop/loop.go` - 极简 Loop
  - `crux-agent-runtime/agent/agent.go` - 高层 Agent
  - `crux-agent-runtime/agent/compaction.go` - 压缩
  - `crux-agent-runtime/agent/append_only_context.go` - 前缀缓存
  - `crux-agent-runtime/agent/session.go` - Session
  - `crux-harness/turn_harness.go` - 治理编排
  - `crux-harness/AGENTS.md` - 架构说明
  - `crux-ai/types.go` - 类型定义
  - `crux-ai/provider.go` - Provider 接口

- pi-ai-go 仓库：`/mnt/workspace/pi-ai-go/`
  - `agent/agent-loop.go` - 两层循环
  - `agent/agent.go` - Agent wrapper
  - `agent/compaction.go` - 压缩
  - `agent/session/session.go` - Session
  - `agent/context.go` - 上下文管理
  - `core/types.go` - 类型化消息
