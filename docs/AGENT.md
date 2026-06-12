# Agent 智能体模块指南

## 一、整体架构

```
用户代码
   │
   ▼
┌─────────────────────────────────────────────────────┐
│  Agent (agent.go)              高层状态管理           │
│  ├─ AgentState (model, tools, messages, hooks)      │
│  ├─ Subscribe() / Steering() / FollowUp()           │
│  └─ Run() / RunContinue() / Abort()                 │
└──────────────┬──────────────────────────────────────┘
               │ 调用
               ▼
┌─────────────────────────────────────────────────────┐
│  AgentLoop / AgentLoopContinue (agent-loop.go)      │
│  ── 核心双层循环 ──                                   │
│                                                      │
│  外层循环：FollowUp 消息驱动                           │
│  ┌─────────────────────────────────────────────┐    │
│  │ 内层循环：LLM 调用 + 工具执行                  │    │
│  │  ① DrainSteering() 注入中控消息              │    │
│  │  ② streamAssistantResponse() → LLM 流式调用  │    │
│  │  ③ extractToolCalls() 提取工具调用            │    │
│  │  ④ executeToolCalls() 并行/顺序执行工具       │    │
│  │  ⑤ 有工具调用或 steering → 继续内层循环       │    │
│  │  ⑥ 无工具且无 steering → 退出内层循环         │    │
│  └─────────────────────────────────────────────┘    │
│  DrainFollowUp() 有消息 → 继续外层循环               │
│  无消息 → finalize() 结束                            │
└─────────────────────────────────────────────────────┘
               │ 依赖
               ▼
┌──────────────┬──────────────┬───────────────────────┐
│  core/       │  llm/        │  agent/tools/          │
│  类型+事件流  │  Stream API  │  bash/read/write/...   │
└──────────────┴──────────────┴───────────────────────┘
```

## 二、构建 Agent 的三种方式

### 方式 1：高层 Agent API（推荐，最简单）

```go
aiAgent := agent.New(agent.AgentOptions{
    InitialState: &agent.AgentState{
        Model:        model,           // core.Model
        SystemPrompt: "你是一个助手",   // 系统提示词
        Tools:        tools,           // []agent.AgentTool
        SimpleStreamOptions: piai.SimpleStreamOptions{
            StreamOptions: piai.StreamOptions{
                APIKey: "your-key",
            },
            Reasoning: piai.ThinkingMedium,  // 推理级别
        },
    },
})

// 订阅事件（流式输出、工具执行进度等）
aiAgent.Subscribe(func(evt agent.AgentEvent) {
    switch e := evt.(type) {
    case agent.EventMessageUpdate:
        if dt, ok := e.AssistantEvent.(piai.EventTextDelta); ok {
            fmt.Print(dt.Delta)
        }
    case agent.EventToolExecStart:
        fmt.Printf("🔧 调用工具: %s\n", e.ToolName)
    case agent.EventToolExecEnd:
        fmt.Printf("✅ 工具完成: %s\n", e.ToolName)
    }
})

// 运行
result, err := aiAgent.Run(ctx, piai.UserMessage{Content: "你好"})
```

**优点**：自动管理消息历史、支持多轮对话、支持 Abort()、支持 Steering/FollowUp 注入。

### 方式 2：底层 AgentLoop API（更灵活）

```go
stream := agent.AgentLoop(ctx, []core.Message{
    core.UserMessage{Content: "你好"},
}, agent.AgentLoopConfig{
    Model:        model,
    SystemPrompt: "你是一个助手",
    Tools:        tools,
    StreamFn:     nil, // 默认用 llm.StreamSimpleWithContext
})

// 消费事件流
messages, err := stream.ForEach(ctx, func(evt agent.AgentEvent) error {
    // 处理事件
    return nil
})
```

**优点**：无状态，适合一次性任务，可完全控制循环行为。

### 方式 3：AgentLoopDetailed（带统计信息）

```go
stream, detailed := agent.AgentLoopDetailed(ctx, prompts, config)

// 消费事件流
stream.ForEach(ctx, func(evt agent.AgentEvent) error { ... })

// 获取详细统计
result, _ := detailed()
fmt.Printf("步骤数: %d, 工具调用: %d, 费用: ¥%.4f\n",
    result.Summary.StepCount,
    result.Summary.ToolCallCount,
    result.Summary.TotalCost,
)
```

## 三、AgentLoopConfig 完整配置项

```go
type AgentLoopConfig struct {
    // ── 基础配置 ──
    Model         core.Model           // 使用的模型
    SystemPrompt  string               // 系统提示词
    Tools         []AgentTool          // 可用工具列表
    ToolExecution ToolExecutionMode    // "parallel"(默认) 或 "sequential"
    SimpleStreamOptions                // API Key、推理级别等

    // ── 消息转换钩子 ──
    ConvertToLlm    func([]Message) []Message  // 转换消息给 LLM（默认过滤非 LLM 类型）
    TransformContext func([]Message) []Message  // 上下文窗口管理

    // ── 生命周期钩子 ──
    GetApiKey           func() string                              // 动态获取 API Key（OAuth 刷新）
    ShouldStopAfterTurn func(AssistantMessage, []ToolResultMessage) bool // 是否停止
    PrepareNextTurn     func(*AgentLoopConfig, ...)                // 修改下一轮配置

    // ── 工具钩子 ──
    BeforeToolCall func(BeforeToolCallContext) *ToolCallBlock  // 拦截工具调用
    AfterToolCall  func(AfterToolCallContext) *ToolCallOverride // 覆盖工具结果

    // ── 消息注入 ──
    Queue *MessageQueue  // 线程安全的消息队列（Steering/FollowUp）
    // 或用回调：
    GetSteeringMessages  func() []Message  // 中控消息
    GetFollowUpMessages  func() []Message  // 后续消息

    // ── 上下文管理 ──
    ContextPolicy   *ContextPolicy   // 软/硬限制、压缩策略
    SummarizeModel  *SummarizeModel  // 用于 LLM 摘要压缩的模型
    SummarizePrompt string           // 自定义摘要提示词
    OnCompaction    func(CompactionEvent)  // 压缩回调
    OnOverflow      func(*OverflowSignal) error  // 溢出回调

    // ── 运行时 ──
    Yielder   *Yielder       // 协作调度（默认 50ms yield）
    Collector *RunCollector   // 统计收集器
    StreamFn  StreamFn        // 自定义流式函数（替换默认 LLM 调用）
}
```

## 四、工具定义模式

```go
func myTool() agent.AgentTool {
    return agent.AgentTool{
        Name:        "tool_name",
        Description: "工具描述（LLM 根据这个决定是否调用）",
        Parameters: json.RawMessage(`{
            "type": "object",
            "properties": {
                "param1": {"type": "string", "description": "参数说明"}
            },
            "required": ["param1"]
        }`),
        Execute: func(ctx context.Context, toolCallID string, params json.RawMessage,
            onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {

            // 1. 解析参数
            var args struct{ Param1 string }
            json.Unmarshal(params, &args)

            // 2. 执行业务逻辑
            result := doSomething(args.Param1)

            // 3. 返回结果
            return agent.AgentToolResult{
                Content: []core.ContentBlock{
                    core.TextContent{Type: "text", Text: result},
                },
                // IsError: true,    // 标记为错误（LLM 会看到错误信息）
                // Terminate: true,  // 终止 Agent 循环
            }, nil
        },
    }
}
```

## 五、事件流（订阅/消费）

Agent 运行过程中产生以下事件：

```
EventAgentStart                    Agent 开始
  ├─ EventTurnStart                一轮开始
  │   ├─ EventMessageStart         LLM 回复开始
  │   │   ├─ EventMessageUpdate    流式更新（嵌套 AssistantMessageEvent）
  │   │   │   ├─ EventTextDelta    文本增量
  │   │   │   ├─ EventThinkingDelta 思考增量
  │   │   │   └─ EventToolCallDelta 工具参数增量
  │   │   └─ EventMessageEnd       LLM 回复结束
  │   ├─ EventToolExecStart        工具开始执行
  │   │   ├─ EventToolExecUpdate   工具执行中（部分结果）
  │   │   └─ EventToolExecEnd      工具执行结束
  │   └─ EventTurnEnd              一轮结束
  ├─ EventCompaction               上下文压缩（可选）
  └─ EventAgentEnd                 Agent 结束（含 Summary/Coverage）
```

### 事件消费示例

```go
aiAgent.Subscribe(func(evt agent.AgentEvent) {
    switch e := evt.(type) {
    case agent.EventAgentStart:
        fmt.Println("🤖 Agent 开始运行")

    case agent.EventMessageUpdate:
        // 流式文本输出
        switch inner := e.AssistantEvent.(type) {
        case piai.EventTextDelta:
            fmt.Print(inner.Delta)
        case piai.EventThinkingDelta:
            fmt.Printf("[思考] %s", inner.Delta)
        }

    case agent.EventToolExecStart:
        fmt.Printf("🔧 [%s] 参数: %s\n", e.ToolName, string(e.Args))

    case agent.EventToolExecEnd:
        if e.IsError {
            fmt.Printf("❌ [%s] 失败: %s\n", e.ToolName, string(e.Result))
        } else {
            fmt.Printf("✅ [%s] 完成\n", e.ToolName)
        }

    case agent.EventCompaction:
        fmt.Printf("📦 上下文压缩: %d → %d tokens\n", e.TokensBefore, e.TokensAfter)

    case agent.EventAgentEnd:
        fmt.Printf("📊 运行结束，共 %d 条消息\n", len(e.Messages))
        if e.Summary != nil {
            fmt.Printf("   步骤: %d, 工具调用: %d, 费用: ¥%.4f\n",
                e.Summary.StepCount, e.Summary.ToolCallCount, e.Summary.TotalCost)
        }
    }
})
```

## 六、MessageQueue 中控注入

运行期间可以从外部 goroutine 注入消息：

```go
queue := agent.NewMessageQueue()
config.Queue = queue

// 启动 Agent（在另一个 goroutine 中）
go agent.AgentLoop(ctx, prompts, config)

// 从主 goroutine 注入中控消息（在下一个 yield 点插入）
queue.Steer(core.UserMessage{Content: "等等，换个方向"})

// 注入后续消息（当前轮次结束后追加）
queue.FollowUp(core.UserMessage{Content: "再帮我查一下..."})
```

### QueueMode 模式

| 模式 | 行为 |
|------|------|
| `QueueModeSteer`（默认） | 在下一个 yield 检查点注入当前轮次 |
| `QueueModeInterrupt` | 中止当前轮次，用新消息重新开始 |
| `QueueModeOneShot` | 中断一次，注入单条消息 |
| `QueueModeQueue` | 等当前轮次结束后，作为 follow-up 追加 |

## 七、上下文管理

### 自动压缩策略

```go
config.ContextPolicy = &agent.ContextPolicy{
    SoftLimit:       0.75,  // 75% 使用率触发压缩
    HardLimit:       0.95,  // 95% 强制压缩
    ReservedOutput:  4096,  // 预留输出 token
    MinTailMessages: 4,     // 保留最近 4 条消息
    Strategy:        agent.CompactionStrategySlidingWindow,
}
```

### 压缩策略

| 策略 | 说明 | 是否需要 LLM |
|------|------|-------------|
| `sliding_window`（默认） | 丢弃最老的消息，保留尾部 N 条 | ❌ 无开销 |
| `summarize` | 用 LLM 摘要被丢弃的消息，插入摘要 | ✅ 需要配置 SummarizeModel |

### Summarize 模式配置

```go
config.SummarizeModel = &agent.SummarizeModel{
    Model: cheapModel,  // 用便宜的模型做摘要
}
config.SummarizePrompt = "自定义摘要提示词..."  // 可选
```

### 溢出检测

```go
config.OnOverflow = func(sig *agent.OverflowSignal) error {
    fmt.Printf("⚠️ 上下文溢出: %s/%s, 使用 %d/%d tokens\n",
        sig.Provider, sig.ModelID, sig.Usage, sig.ContextWindow)
    return nil  // 返回 error 可中止循环
}
```

## 八、内置工具集

```go
import "pi-ai-go/agent/tools"

// 一键获取所有内置工具
allTools := tools.All()

// 或单独使用
customTools := []agent.AgentTool{
    tools.Read(),   // 读文件（支持 offset/limit）
    tools.Write(),  // 写文件
    tools.Edit(),   // 编辑文件（str_replace 模式）
    tools.Bash(),   // 执行 shell 命令
    tools.Glob(),   // 文件名匹配（glob 模式）
    tools.Grep(),   // 内容搜索（支持正则）
}
```

### 工具详情

| 工具 | 名称 | 功能 |
|------|------|------|
| Read | `read_file` | 读取文件内容，支持 offset/limit 行范围 |
| Write | `write_file` | 创建或覆盖文件 |
| Edit | `edit_file` | 字符串替换编辑（old_string → new_string） |
| Bash | `bash` | 执行 shell 命令，支持超时控制 |
| Glob | `glob` | 按模式匹配文件路径（如 `**/*.go`） |
| Grep | `grep` | 搜索文件内容，支持正则表达式 |

## 九、内置工具与自定义工具混合

```go
// 以内置工具为基础，添加自定义工具
tools := append(tools.All(),
    agent.AgentTool{
        Name:        "web_search",
        Description: "搜索互联网获取最新信息",
        Parameters: json.RawMessage(`{
            "type": "object",
            "properties": {
                "query": {"type": "string", "description": "搜索关键词"}
            },
            "required": ["query"]
        }`),
        Execute: func(ctx context.Context, id string, params json.RawMessage,
            onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
            // 调用搜索 API
            return agent.AgentToolResult{
                Content: []core.ContentBlock{
                    core.TextContent{Type: "text", Text: "搜索结果..."},
                },
            }, nil
        },
    },
)
```

## 十、运行统计（AgentRunSummary）

使用 `AgentLoopDetailed` 获取运行统计：

```go
stream, detailed := agent.AgentLoopDetailed(ctx, prompts, config)
stream.ForEach(ctx, func(evt agent.AgentEvent) error { ... })

result, _ := detailed()
s := result.Summary

fmt.Printf("步骤数:     %d\n", s.StepCount)
fmt.Printf("工具调用:   %d\n", s.ToolCallCount)
fmt.Printf("错误数:     %d\n", s.ErrorCount)
fmt.Printf("总耗时:     %v\n", s.Duration)
fmt.Printf("总费用:     ¥%.4f\n", s.TotalCost)
fmt.Printf("输入 Token: %d\n", s.TotalUsage.Input)
fmt.Printf("输出 Token: %d\n", s.TotalUsage.Output)
fmt.Printf("停止原因:   %s\n", s.StopReasonFinal)

// 按 provider 统计
for provider, hits := range s.Providers {
    fmt.Printf("  %s: %d 次调用\n", provider, hits)
}

// 按错误类型统计
for kind, count := range s.ErrorsByKind {
    fmt.Printf("  %s: %d 次\n", kind, count)
}
```

## 十一、完整构建示例

```go
package main

import (
    "context"
    "fmt"
    "time"

    piai "pi-ai-go"
    "pi-ai-go/agent"
    "pi-ai-go/agent/tools"
    _ "pi-ai-go/providers"
)

func main() {
    model := piai.Model{
        ID:            "deepseek-v4-flash",
        API:           piai.APIOpenAICompletions,
        Provider:      piai.ProviderDeepSeek,
        ContextWindow: 64000,
        MaxTokens:     4096,
    }

    // 1. 创建 Agent
    aiAgent := agent.New(agent.AgentOptions{
        InitialState: &agent.AgentState{
            Model:        model,
            SystemPrompt: "你是一个编程助手，可以读写文件和执行命令。",
            Tools:        tools.All(),
            SimpleStreamOptions: piai.SimpleStreamOptions{
                StreamOptions: piai.StreamOptions{
                    APIKey: "your-deepseek-key",
                },
            },
        },
    })

    // 2. 订阅事件（流式输出）
    aiAgent.Subscribe(func(evt agent.AgentEvent) {
        switch e := evt.(type) {
        case agent.EventMessageUpdate:
            if dt, ok := e.AssistantEvent.(piai.EventTextDelta); ok {
                fmt.Print(dt.Delta)
            }
        case agent.EventToolExecStart:
            fmt.Printf("\n🔧 [%s] %s\n", e.ToolName, string(e.Args))
        case agent.EventToolExecEnd:
            status := "✅"
            if e.IsError {
                status = "❌"
            }
            fmt.Printf("%s [%s]\n", status, e.ToolName)
        case agent.EventAgentEnd:
            fmt.Printf("\n📊 共 %d 条消息\n", len(e.Messages))
        }
    })

    // 3. 运行
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
    defer cancel()

    result, err := aiAgent.Run(ctx,
        piai.UserMessage{Content: "列出当前目录的文件，然后读取 README.md 的前 10 行"},
    )
    if err != nil {
        fmt.Printf("错误: %v\n", err)
        return
    }
    _ = result

    // 4. 多轮对话（自动保持历史）
    aiAgent.Run(ctx, piai.UserMessage{Content: "总结一下 README 的内容"})
}
```

## 十二、错误处理

Agent 定义了以下哨兵错误类型，支持 `errors.Is` / `errors.As`：

| 错误 | 含义 |
|------|------|
| `ErrAgentAborted` | 用户取消了 Agent 运行 |
| `ErrOverflow` | 上下文窗口溢出 |
| `ErrToolCallBlocked` | BeforeToolCall 钩子拦截了工具调用 |
| `ErrToolNotFound` | LLM 幻觉了一个不存在的工具名 |
| `ErrToolExecFailure` | 工具执行失败 |

```go
result, err := aiAgent.Run(ctx, msg)
if err != nil {
    if errors.Is(err, agent.ErrOverflow) {
        fmt.Println("上下文溢出，需要压缩或切换模型")
    } else if errors.Is(err, agent.ErrAgentAborted) {
        fmt.Println("Agent 被用户取消")
    }
}
```

## 十三、Abort 取消

```go
// 在另一个 goroutine 中取消
go func() {
    time.Sleep(5 * time.Second)
    aiAgent.Abort()
}()

result, err := aiAgent.Run(ctx, msg)
// err 会是 ErrAgentAborted
```
