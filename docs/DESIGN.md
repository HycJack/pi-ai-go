# 架构设计文档

## 设计目标

1. **统一接口** — 一套代码调用所有 AI 模型
2. **流式优先** — 基于 Go channel 的泛型异步事件流
3. **零依赖** — 核心功能仅使用 Go 标准库
4. **分层架构** — 类型、调用、Provider、Agent 四层解耦
5. **可扩展** — 通过 Provider 接口添加新服务商

## 分层架构

```
┌─────────────────────────────────────────────────┐
│  外部使用者 (import piai "pi-ai-go")             │
│         │                                       │
│         ▼                                       │
│  ┌───────────┐     re-export     ┌───────────┐  │
│  │  piai.go  │ ◄─────────────── │  core/    │  │
│  │ (facade)  │                   │  + ai/    │  │
│  └─────┬─────┘                   └─────┬─────┘  │
│        │                               │        │
│        ▼                               ▼        │
│  ┌──────────┐   depends on   ┌──────────────┐   │
│  │ agent/   │ ──────────────→│     ai/      │   │
│  └──────────┘                └──────┬───────┘   │
│        │                            │           │
│        ▼                            ▼           │
│  ┌──────────┐   depends on   ┌──────────────┐   │
│  │  core/   │ ◄─────────────│  providers/  │   │
│  └──────────┘                └──────────────┘   │
└─────────────────────────────────────────────────┘
```

### core/ — 核心层

**职责**：纯类型定义 + EventStream + Provider 注册表

**依赖**：零外部依赖

**内容**：
- 类型定义：`Message`, `Model`, `Tool`, `ContentBlock`, `Usage` 等
- 事件系统：`EventStream[T,R]` 泛型 + 所有事件类型
- Provider 注册：`APIProvider` 接口 + 注册/查找函数
- 环境变量：`ResolveAPIKey`, `ResolveBaseURL`

**设计原则**：
- 只有类型和接口，没有业务逻辑
- `CalculateCost` 等纯函数也放在这里（输入输出都是 core 类型）

### ai/ — AI 调用层

**职责**：公开 API 函数 + Model 注册表

**依赖**：仅依赖 `core/`

**内容**：
- 公开函数：`Stream()`, `Complete()`, `StreamSimple()`, `GenerateImages()`
- Model 注册表：`LoadModels()`, `GetModel()`, `GetModels()`
- ThinkingLevel 工具：`ClampThinkingLevel()`, `GetSupportedThinkingLevels()`

**设计原则**：
- 所有公开 API 函数都是对 Provider 接口调用的薄封装
- Model 注册表与 Provider 注册表分离（model 是配置，provider 是实现）

### providers/ — Provider 实现层

**职责**：各 AI 服务商的具体实现

**依赖**：仅依赖 `core/`

**内容**：
- 每个服务商一个子包（`openai/`, `anthropic/`, `google/` 等）
- `register.go`：通过 `init()` 自动注册所有内置 Provider

**设计原则**：
- Provider 实现 `core.APIProvider` 接口
- 通过 `core.RegisterProvider()` 注册，不依赖 `ai/` 包
- 每个 Provider 独立处理 HTTP 请求、SSE 解析、事件推送

### agent/ — Agent 智能体层

**职责**：多轮对话循环 + 工具执行 + 状态管理

**依赖**：`core/` + `ai/`

**内容**：
- `AgentLoop` / `AgentLoopContinue`：核心循环
- `Agent`：状态管理包装器
- 工具执行：并行/顺序、before/after hooks

### piai.go — Facade

**职责**：统一入口，re-export 所有公开符号

**依赖**：`core/` + `ai/`

外部使用者 `import piai "pi-ai-go"` 即可访问所有功能。

## 核心类型关系

```
Context
├── SystemPrompt: string
├── Messages: []Message ←──── UserMessage
│                            AssistantMessage
│                            ToolResultMessage
└── Tools: []Tool

AssistantMessage
├── Content: []ContentBlock ← TextContent
│                              ThinkingContent
│                              ImageContent
│                              ToolCall
├── Usage: Usage
└── StopReason: StopReason
```

## EventStream 设计

```go
type EventStream[T any, R any] struct {
    ch     chan streamEvt[T]  // 带缓冲的事件通道
    done   chan struct{}       // 完成信号
    stop   chan struct{}       // 消费者停止信号
    result R                  // 最终结果
    mu     sync.Mutex         // 保护 closed 标志
}
```

**关键特性**：
- `Push()` 非阻塞：buffer 满时返回 false，不阻塞生产者
- `End()`/`Error()` 在锁内完成所有 channel 操作：避免与 Push 竞态
- `Stop()` 关闭 stop channel：通知生产者停止
- `ForEach()` 支持 context 取消：消费者可随时停止

**事件序列**：
```
EventStart → EventTextDelta* → EventTextEnd
             EventThinkingDelta* → EventThinkingEnd
             EventToolCallStart → EventToolCallDelta* → EventToolCallEnd
→ EventDone
```

## Provider 接口

```go
type APIProvider interface {
    Stream(ctx context.Context, model Model, llmCtx Context, opts StreamOptions) (*EventStream[...], error)
    StreamSimple(ctx context.Context, model Model, llmCtx Context, opts SimpleStreamOptions) (*EventStream[...], error)
}
```

**实现新 Provider**：

1. 创建 `providers/your-provider/your-provider.go`
2. 实现 `core.APIProvider` 接口
3. 在 `providers/register.go` 添加注册
4. 添加 `_test.go`

## Agent 循环

```
┌──────────────────────────────────────┐
│           Outer Loop                 │
│  ┌──────────────────────────────┐    │
│  │        Inner Loop            │    │
│  │  1. 注入 steering 消息       │    │
│  │  2. 调用 LLM (stream)       │    │
│  │  3. 执行工具调用             │    │
│  │  4. 检查 terminate          │    │
│  │  5. 若有工具结果，继续内循环  │    │
│  └──────────────────────────────┘    │
│  6. 检查 follow-up 消息             │
│  7. 若有 follow-up，继续外循环       │
└──────────────────────────────────────┘
```

**工具执行模式**：
- `parallel`（默认）：所有工具并发执行
- `sequential`：按顺序执行，任一工具可请求终止

**生命周期钩子**：
- `BeforeToolCall`：执行前拦截（可阻止执行）
- `AfterToolCall`：执行后覆盖结果
- `PrepareNextTurn`：修改下一轮配置
- `ShouldStopAfterTurn`：决定是否停止

## 设计决策

1. **core 包含注册表** — 避免 `providers/` 和 `ai/` 之间的循环依赖
2. **Facade re-export** — 外部使用方式不变，内部结构清晰
3. **context 贯穿** — 从 API 层到 HTTP 请求全链路支持取消
4. **EventStream 泛型** — 类型安全，编译时检查
5. **init() 自动注册** — `_ "pi-ai-go/providers"` 触发注册，零配置
