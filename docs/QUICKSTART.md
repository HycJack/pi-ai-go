# 快速开始

## 安装

```bash
go get pi-ai-go
```

## 设置 API Key

```bash
export OPENAI_API_KEY="sk-..."
# 或
export ANTHROPIC_API_KEY="sk-ant-..."
```

## 基本调用

```go
package main

import (
    "context"
    "fmt"
    "log"

    piai "pi-ai-go"
    _ "pi-ai-go/providers"
)

func main() {
    // 定义模型（也可以用 piai.GetModel() 从注册表获取）
    model := piai.Model{
        ID:       "gpt-4o",
        API:      piai.APIOpenAICompletions,
        Provider: piai.ProviderOpenAI,
        Input:    []piai.Modality{piai.ModalityText},
        MaxTokens: 4096,
    }

    // 发送请求
    msg, err := piai.CompleteSimple(context.Background(), model, []piai.Message{
        piai.UserMessage{Content: "用一句话介绍 Go 语言"},
    })
    if err != nil {
        log.Fatal(err)
    }

    for _, block := range msg.Content {
        if text, ok := block.(piai.TextContent); ok {
            fmt.Println(text.Text)
        }
    }
}
```

## 流式输出

```go
stream, err := piai.StreamSimple(ctx, model, []piai.Message{
    piai.UserMessage{Content: "写一首五言绝句"},
})
if err != nil {
    log.Fatal(err)
}

stream.ForEach(ctx, func(evt piai.AssistantMessageEvent) error {
    switch e := evt.(type) {
    case piai.EventTextDelta:
        fmt.Print(e.Delta)
    case piai.EventDone:
        fmt.Printf("\n--- Token: %d ---\n", e.Message.Usage.TotalTokens)
    }
    return nil
})
```

## 带推理深度

```go
msg, err := piai.CompleteSimple(ctx, model, []piai.Message{
    piai.UserMessage{Content: "解释费马大定理"},
}, piai.SimpleStreamOptions{
    Reasoning: piai.ThinkingHigh, // minimal / low / medium / high / xhigh
})
```

## 使用不同服务商

只需改 `Model` 的 `API`、`Provider`、`BaseURL`：

```go
// DeepSeek
model := piai.Model{
    ID:       "deepseek-chat",
    API:      piai.APIOpenAICompletions,
    Provider: piai.ProviderDeepSeek,
    BaseURL:  "https://api.deepseek.com/v1",
}

// Anthropic
model := piai.Model{
    ID:       "claude-sonnet-4-20250514",
    API:      piai.APIAnthropicMessages,
    Provider: piai.ProviderAnthropic,
}

// Google Gemini
model := piai.Model{
    ID:       "gemini-2.0-flash",
    API:      piai.APIGoogleGenerative,
    Provider: piai.ProviderGoogle,
}

// 自定义 OpenAI 兼容 API
model := piai.Model{
    ID:       "your-model",
    API:      piai.APIOpenAICompletions,
    Provider: piai.ProviderOpenAI,
    BaseURL:  "https://your-proxy.com/v1",
}
```

## Agent 智能体

```go
import (
    "pi-ai-go/agent"
)

tools := []agent.AgentTool{
    {
        Name:        "search",
        Description: "搜索信息",
        Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
        Execute: func(ctx context.Context, id string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
            var args struct{ Query string }
            json.Unmarshal(params, &args)
            // 执行搜索...
            return agent.AgentToolResult{
                Content: []piai.ContentBlock{piai.TextContent{Text: "搜索结果..."}},
            }, nil
        },
    },
}

aiAgent := agent.New(agent.AgentOptions{
    InitialState: &agent.AgentState{
        Model:        model,
        SystemPrompt: "你是一个助手，可以搜索信息。",
        Tools:        tools,
        SimpleStreamOptions: piai.SimpleStreamOptions{
            StreamOptions: piai.StreamOptions{APIKey: apiKey},
        },
    },
})

// 订阅事件（可选）
aiAgent.Subscribe(func(evt agent.AgentEvent) {
    if e, ok := evt.(agent.EventToolExecStart); ok {
        fmt.Printf("🔧 调用工具: %s\n", e.ToolName)
    }
})

// 运行
result, err := aiAgent.Run(ctx, piai.UserMessage{Content: "帮我搜索 Go 1.23 的新特性"})
```

## 下一步

- [API 参考](API.md) — 完整 API 文档
- [架构设计](DESIGN.md) — 分层架构详解
- [examples/](../examples/) — 完整示例代码
