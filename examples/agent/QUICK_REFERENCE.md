# Agent 工具调用快速参考

## 🚀 快速开始

### 最小化示例

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    
    piai "pi-ai-go"
    "pi-ai-go/agent"
    _ "pi-ai-go/providers"
)

func main() {
    // 1. 配置
    model := piai.Model{
        ID:       "Qwen/Qwen2.5-7B-Instruct",
        API:      piai.APIOpenAICompletions,
        Provider: piai.ProviderDeepSeek,
        BaseURL:  "https://api.siliconflow.cn/v1",
        Input:    []piai.Modality{piai.ModalityText},
    }

    // 2. 定义工具
    tool := agent.AgentTool{
        Name:        "hello",
        Description: "打招呼",
        Parameters:  json.RawMessage(`{"type": "object", "properties": {}}`),
        Execute: func(ctx context.Context, id string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
            return agent.AgentToolResult{
                Content: []piai.ContentBlock{piai.TextContent{Type: "text", Text: "Hello!"}},
            }, nil
        },
    }

    // 3. 创建 Agent
    aiAgent := agent.New(agent.AgentOptions{
        InitialState: &agent.AgentState{
            Model:        model,
            SystemPrompt: "你是一个助手，使用 hello 工具打招呼。",
            Tools:        []agent.AgentTool{tool},
            SimpleStreamOptions: piai.SimpleStreamOptions{
                StreamOptions: piai.StreamOptions{APIKey: "your-key"},
            },
        },
    })

    // 4. 运行
    result, _ := aiAgent.Run(context.Background(), piai.UserMessage{
        Role:    "user",
        Content: "你好",
    })

    // 5. 处理结果
    for _, msg := range result {
        if am, ok := msg.(piai.AssistantMessage); ok {
            for _, block := range am.Content {
                if text, ok := block.(piai.TextContent); ok {
                    fmt.Println(text.Text)
                }
            }
        }
    }
}
```

---

## 📦 核心类型

### AgentTool

```go
agent.AgentTool{
    Name:        string,              // 工具名称（必需）
    Description: string,              // 工具描述（必需）
    Parameters:  json.RawMessage,     // JSON Schema（必需）
    Execute:     ToolExecuteFunc,     // 执行函数（必需）
    ExecutionMode: ToolExecutionMode, // 执行模式（可选）
}
```

### ToolExecuteFunc

```go
type ToolExecuteFunc func(
    ctx context.Context,           // 上下文
    toolCallID string,             // 工具调用ID
    params json.RawMessage,        // 参数JSON
    onUpdate func(json.RawMessage), // 进度更新回调
) (AgentToolResult, error)
```

### AgentToolResult

```go
agent.AgentToolResult{
    Content:   []piai.ContentBlock, // 返回内容
    Details:   json.RawMessage,     // 详细信息（可选）
    IsError:   bool,                // 是否为错误
    Terminate: bool,                // 是否终止Agent循环
}
```

---

## 🛠️ 工具定义模板

### 基础模板

```go
func myTool() agent.AgentTool {
    return agent.AgentTool{
        Name:        "my_tool",
        Description: "工具功能描述",
        Parameters: json.RawMessage(`{
            "type": "object",
            "properties": {
                "param1": {
                    "type": "string",
                    "description": "参数说明"
                },
                "param2": {
                    "type": "integer",
                    "description": "参数说明"
                }
            },
            "required": ["param1"]
        }`),
        Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
            // 1. 解析参数
            var args struct {
                Param1 string `json:"param1"`
                Param2 int    `json:"param2"`
            }
            if err := json.Unmarshal(params, &args); err != nil {
                return agent.AgentToolResult{
                    Content: []piai.ContentBlock{piai.TextContent{
                        Type: "text",
                        Text: fmt.Sprintf("参数错误: %v", err),
                    }},
                    IsError: true,
                }, nil
            }

            // 2. 执行逻辑
            result := doSomething(args.Param1, args.Param2)

            // 3. 返回结果
            return agent.AgentToolResult{
                Content: []piai.ContentBlock{piai.TextContent{
                    Type: "text",
                    Text: result,
                }},
            }, nil
        },
    }
}
```

### 带进度更新的模板

```go
func longRunningTool() agent.AgentTool {
    return agent.AgentTool{
        Name:        "long_task",
        Description: "长时间运行的任务",
        Parameters:  json.RawMessage(`{}`),
        Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
            total := 100
            for i := 0; i < total; i++ {
                // 检查取消
                select {
                case <-ctx.Done():
                    return agent.AgentToolResult{
                        Content: []piai.ContentBlock{piai.TextContent{
                            Type: "text",
                            Text: "任务被取消",
                        }},
                        IsError: true,
                    }, nil
                default:
                }

                // 发送进度
                progress := map[string]interface{}{
                    "current": i + 1,
                    "total":   total,
                    "percent": (i + 1) * 100 / total,
                }
                progressJSON, _ := json.Marshal(progress)
                onUpdate(progressJSON)

                // 执行任务
                time.Sleep(100 * time.Millisecond)
            }

            return agent.AgentToolResult{
                Content: []piai.ContentBlock{piai.TextContent{
                    Type: "text",
                    Text: "任务完成",
                }},
            }, nil
        },
    }
}
```

---

## 🎯 Agent 配置

### 基础配置

```go
aiAgent := agent.New(agent.AgentOptions{
    InitialState: &agent.AgentState{
        Model:        model,           // LLM 模型
        SystemPrompt: "...",           // 系统提示词
        Tools:        tools,           // 工具列表
        SimpleStreamOptions: piai.SimpleStreamOptions{
            StreamOptions: piai.StreamOptions{
                APIKey: apiKey,        // API 密钥
            },
        },
    },
})
```

### 高级配置

```go
aiAgent := agent.New(agent.AgentOptions{
    InitialState: &agent.AgentState{
        // 基础配置
        Model:        model,
        SystemPrompt: "...",
        Tools:        tools,

        // 工具执行模式
        ToolExecution: agent.ToolExecParallel, // 并行执行

        // 消息转换
        ConvertToLlm: func(msgs []piai.Message) []piai.Message {
            // 自定义消息转换逻辑
            return msgs
        },

        // 上下文管理
        TransformContext: func(msgs []piai.Message) []piai.Message {
            // 限制上下文长度
            if len(msgs) > 100 {
                return msgs[len(msgs)-100:]
            }
            return msgs
        },

        // 轮次控制
        ShouldStopAfterTurn: func(msg piai.AssistantMessage, results []piai.ToolResultMessage) bool {
            // 自定义停止条件
            return false
        },

        // 工具钩子
        BeforeToolCall: func(ctx agent.BeforeToolCallContext) *agent.ToolCallBlock {
            // 工具执行前的验证
            return nil // 返回 nil 表示允许执行
        },

        AfterToolCall: func(ctx agent.AfterToolCallContext) *agent.ToolCallOverride {
            // 工具执行后的处理
            return nil // 返回 nil 表示不修改结果
        },

        SimpleStreamOptions: piai.SimpleStreamOptions{
            StreamOptions: piai.StreamOptions{
                APIKey: apiKey,
            },
        },
    },
})
```

---

## 📡 事件订阅

### 事件类型

```go
aiAgent.Subscribe(func(evt agent.AgentEvent) {
    switch e := evt.(type) {
    case agent.EventAgentStart:
        // Agent 开始运行
    case agent.EventAgentEnd:
        // Agent 运行结束
        // e.Messages - 最终消息列表
    case agent.EventTurnStart:
        // 对话轮次开始
    case agent.EventTurnEnd:
        // 对话轮次结束
        // e.Message - 助手消息
        // e.ToolResults - 工具结果列表
    case agent.EventMessageStart:
        // 助手消息流开始
    case agent.EventMessageUpdate:
        // 助手消息流更新
        // e.Message - 当前消息
        // e.AssistantEvent - 原始事件
    case agent.EventMessageEnd:
        // 助手消息流结束
    case agent.EventToolExecStart:
        // 工具开始执行
        // e.ToolCallID - 调用ID
        // e.ToolName - 工具名称
        // e.Args - 参数
    case agent.EventToolExecUpdate:
        // 工具执行进度更新
        // e.PartialResult - 部分结果
    case agent.EventToolExecEnd:
        // 工具执行完成
        // e.Result - 结果
        // e.IsError - 是否错误
    }
})
```

### 常用模式

#### 日志记录

```go
aiAgent.Subscribe(func(evt agent.AgentEvent) {
    switch e := evt.(type) {
    case agent.EventToolExecStart:
        log.Printf("[TOOL START] %s (ID: %s)", e.ToolName, e.ToolCallID)
    case agent.EventToolExecEnd:
        if e.IsError {
            log.Printf("[TOOL ERROR] %s: %s", e.ToolName, string(e.Result))
        } else {
            log.Printf("[TOOL END] %s", e.ToolName)
        }
    }
})
```

#### 实时输出

```go
aiAgent.Subscribe(func(evt agent.AgentEvent) {
    switch e := evt.(type) {
    case agent.EventMessageUpdate:
        for _, block := range e.Message.Content {
            if text, ok := block.(piai.TextContent); ok {
                fmt.Print(text.Text)
            }
        }
    case agent.EventMessageEnd:
        fmt.Println()
    }
})
```

#### 收集指标

```go
metrics := struct {
    ToolCalls  int
    Errors     int
    TotalTime  time.Duration
}{}

startTime := time.Now()

aiAgent.Subscribe(func(evt agent.AgentEvent) {
    switch e := evt.(type) {
    case agent.EventToolExecStart:
        metrics.ToolCalls++
    case agent.EventToolExecEnd:
        if e.IsError {
            metrics.Errors++
        }
    case agent.EventAgentEnd:
        metrics.TotalTime = time.Since(startTime)
    }
})
```

---

## 🔄 Agent 方法

### Run - 执行新对话

```go
result, err := aiAgent.Run(ctx, piai.UserMessage{
    Role:    "user",
    Content: "你的问题",
})
```

### RunContinue - 继续对话

```go
// 无需传递消息，自动使用历史记录
result, err := aiAgent.RunContinue(ctx)
```

### 消息管理

```go
// 获取消息历史
messages := aiAgent.Messages()

// 获取状态
state := aiAgent.State()

// 更新工具
aiAgent.SetTools(newTools)

// 更新模型
aiAgent.SetModel(newModel)

// 更新系统提示
aiAgent.SetSystemPrompt("新的提示词")
```

### 流控制

```go
// 注入紧急消息（当前轮次处理）
aiAgent.Steering(piai.UserMessage{
    Role:    "user",
    Content: "紧急消息",
})

// 注入后续消息（下一轮处理）
aiAgent.FollowUp(piai.UserMessage{
    Role:    "user",
    Content: "后续消息",
})

// 取消当前运行
aiAgent.Abort()
```

---

## ⚙️ 工具执行模式

### 并行执行（默认）

```go
config := agent.AgentLoopConfig{
    ToolExecution: agent.ToolExecParallel,
    // ...
}
```

### 顺序执行

```go
config := agent.AgentLoopConfig{
    ToolExecution: agent.ToolExecSequential,
    // ...
}
```

### 单个工具强制顺序

```go
tool := agent.AgentTool{
    Name:          "sequential_tool",
    ExecutionMode: agent.ToolExecSequential,
    // ...
}
```

---

## 🎨 JSON Schema 参考

### 常用类型

```go
// 字符串
`{"type": "string", "description": "描述"}`

// 整数
`{"type": "integer", "description": "描述"}`

// 浮点数
`{"type": "number", "description": "描述"}`

// 布尔值
`{"type": "boolean", "description": "描述"}`

// 数组
`{
    "type": "array",
    "items": {"type": "string"},
    "description": "字符串数组"
}`

// 对象
`{
    "type": "object",
    "properties": {
        "key": {"type": "string", "description": "描述"}
    },
    "required": ["key"]
}`
```

### 枚举

```go
`{
    "type": "string",
    "enum": ["option1", "option2", "option3"],
    "description": "选择一项"
}`
```

### 默认值

```go
`{
    "type": "string",
    "default": "default_value",
    "description": "可选参数"
}`
```

---

## 🔧 调试技巧

### 启用详细日志

```go
aiAgent.Subscribe(func(evt agent.AgentEvent) {
    log.Printf("Event: %T %+v", evt, evt)
})
```

### 打印消息历史

```go
func printMessages(messages []piai.Message) {
    for i, msg := range messages {
        switch m := msg.(type) {
        case piai.UserMessage:
            fmt.Printf("[%d] User: %s\n", i, m.Content)
        case piai.AssistantMessage:
            fmt.Printf("[%d] Assistant: ", i)
            for _, block := range m.Content {
                if text, ok := block.(piai.TextContent); ok {
                    fmt.Print(text.Text)
                }
            }
            fmt.Println()
        case piai.ToolResultMessage:
            fmt.Printf("[%d] Tool Result (%s): ", i, m.ToolName)
            for _, block := range m.Content {
                if text, ok := block.(piai.TextContent); ok {
                    fmt.Print(text.Text)
                }
            }
            fmt.Println()
        }
    }
}
```

### 测试工具

```go
func TestMyTool(t *testing.T) {
    tool := myTool()
    
    params := json.RawMessage(`{"param1": "test"}`)
    result, err := tool.Execute(context.Background(), "test-id", params, func(json.RawMessage) {})
    
    if err != nil {
        t.Fatalf("Error: %v", err)
    }
    
    if result.IsError {
        t.Error("Expected success, got error")
    }
    
    // 验证结果
    for _, block := range result.Content {
        if text, ok := block.(piai.TextContent); ok {
            if text.Text != "expected" {
                t.Errorf("Expected 'expected', got '%s'", text.Text)
            }
        }
    }
}
```

---

## 📋 System Prompt 模板

### 通用模板

```
你是一个智能助手，可以使用以下工具：

{工具列表}

使用说明：
1. 根据用户问题选择合适的工具
2. 确保参数正确
3. 解释工具返回的结果
4. 如果工具失败，向用户解释原因

注意事项：
- 只在必要时使用工具
- 提供清晰、有用的回答
- 保持友好、专业的语气
```

### 工具专用模板

```
你是{领域}助手。你可以使用 {工具名} 工具来{功能}。

工具使用方法：
- {参数1}: {说明}
- {参数2}: {说明}

请根据用户需求，合理使用工具并解释结果。
```

---

## ⚠️ 常见陷阱

### ❌ 错误做法

```go
// 1. 忽略错误
result, _ := tool.Execute(...)

// 2. 返回 Go error（应该返回 AgentToolResult）
return agent.AgentToolResult{}, err

// 3. 不检查上下文
func Execute(ctx context.Context, ...) {
    // 长时间运行，不检查 ctx.Done()
}

// 4. 参数描述不清晰
Parameters: json.RawMessage(`{"type": "object", "properties": {}}`)
```

### ✅ 正确做法

```go
// 1. 处理错误
result, err := tool.Execute(...)
if err != nil {
    log.Printf("Error: %v", err)
}

// 2. 返回带错误标记的结果
return agent.AgentToolResult{
    Content: []piai.ContentBlock{piai.TextContent{
        Type: "text",
        Text: "错误信息",
    }},
    IsError: true,
}, nil

// 3. 检查上下文
func Execute(ctx context.Context, ...) {
    select {
    case <-ctx.Done():
        return agent.AgentToolResult{...}, nil
    default:
    }
}

// 4. 清晰的参数描述
Parameters: json.RawMessage(`{
    "type": "object",
    "properties": {
        "city": {
            "type": "string",
            "description": "城市名称，例如：北京、上海"
        }
    },
    "required": ["city"]
}`)
```

---

## 📚 相关资源

- [README.md](README.md) - 项目说明
- [INSTALL.md](INSTALL.md) - 安装指南
- [USAGE_EXAMPLES.md](USAGE_EXAMPLES.md) - 使用示例
- [Agent 源码](../../agent/) - Agent 包源码
- [测试用例](../../agent/agent_test.go) - 单元测试

---

**最后更新**: 2024-01-20
**版本**: 1.0.0
