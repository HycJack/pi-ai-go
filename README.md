# pi-ai-go

统一的多模型 AI API Go 语言 SDK，支持流式文本生成、工具调用、思维链推理和 Agent 智能体。

## 核心特性

- **统一接口** — 一套 API 调用所有支持的模型
- **流式响应** — 基于 Go channel 的泛型异步事件流
- **工具调用** — 统一的 Function Calling 接口
- **思维链** — 支持推理模型的 thinking/reasoning
- **Agent 智能体** — 内置多轮对话 + 工具执行循环
- **OAuth 认证** — Anthropic、GitHub Copilot、OpenAI Codex
- **零外部依赖** — 核心功能仅使用 Go 标准库

## 支持的服务商

| 服务商 | API 类型 | 包路径 |
|--------|----------|--------|
| Anthropic | Messages API | `providers/anthropic` |
| OpenAI | Chat Completions / Responses | `providers/openai` |
| Azure OpenAI | Responses API | `providers/openai` |
| OpenAI Codex | Responses API | `providers/openai` |
| Google Gemini | Generative AI | `providers/google` |
| Google Vertex | Vertex AI | `providers/google` |
| Amazon Bedrock | Converse Stream | `providers/bedrock` |
| Mistral | Conversations API | `providers/mistral` |
| OpenRouter | 图像生成 | `providers/images` |

## 安装

```bash
go get pi-ai-go
```

## 快速开始

### 基本调用

```go
package main

import (
    "context"
    "fmt"
    "log"

    piai "pi-ai-go"
    _ "pi-ai-go/providers" // 注册内置服务商
)

func main() {
    model := piai.Model{
        ID:       "gpt-4o",
        API:      piai.APIOpenAICompletions,
        Provider: piai.ProviderOpenAI,
        Input:    []piai.Modality{piai.ModalityText},
        MaxTokens: 4096,
    }

    msg, err := piai.CompleteSimple(context.Background(), model, []piai.Message{
        piai.UserMessage{Content: "你好，请介绍一下自己"},
    }, piai.SimpleStreamOptions{
        Reasoning: piai.ThinkingMedium,
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

### 流式响应

```go
stream, err := piai.StreamSimple(ctx, model, []piai.Message{
    piai.UserMessage{Content: "写一首关于春天的诗"},
})
if err != nil {
    log.Fatal(err)
}

stream.ForEach(ctx, func(evt piai.AssistantMessageEvent) error {
    switch e := evt.(type) {
    case piai.EventTextDelta:
        fmt.Print(e.Delta)
    case piai.EventThinkingDelta:
        fmt.Printf("[思考] %s", e.Delta)
    case piai.EventDone:
        fmt.Printf("\n费用: $%.4f\n", e.Message.Usage.Cost.Total)
    }
    return nil
})
```

### Agent 智能体

```go
import (
    piai "pi-ai-go"
    "pi-ai-go/agent"
    _ "pi-ai-go/providers"
)

tools := []agent.AgentTool{
    {
        Name:        "get_time",
        Description: "获取当前时间",
        Parameters:  json.RawMessage(`{}`),
        Execute: func(ctx context.Context, id string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
            return agent.AgentToolResult{
                Content: []piai.ContentBlock{
                    piai.TextContent{Text: time.Now().Format("2006-01-02 15:04:05")},
                },
            }, nil
        },
    },
}

aiAgent := agent.New(agent.AgentOptions{
    InitialState: &agent.AgentState{
        Model:        model,
        SystemPrompt: "你是一个助手。",
        Tools:        tools,
        SimpleStreamOptions: piai.SimpleStreamOptions{
            StreamOptions: piai.StreamOptions{APIKey: "your-key"},
        },
    },
})

result, err := aiAgent.Run(ctx, piai.UserMessage{Content: "现在几点了？"})
```

### 工具调用

```go
tools := []piai.Tool{
    {
        Name:        "get_weather",
        Description: "获取天气信息",
        Parameters:  json.RawMessage(`{
            "type": "object",
            "properties": {
                "city": {"type": "string"}
            },
            "required": ["city"]
        }`),
    },
}

stream, _ := piai.Stream(ctx, model, []piai.Message{
    piai.UserMessage{Content: "北京天气怎么样？"},
}, piai.StreamOptions{})

stream.ForEach(ctx, func(evt piai.AssistantMessageEvent) error {
    if e, ok := evt.(piai.EventToolCallEnd); ok {
        // 解析参数，执行工具，返回结果
    }
    return nil
})
```

### 图像输入

```go
msg, _ := piai.CompleteSimple(ctx, model, []piai.Message{
    piai.UserMessage{
        Content: []piai.ContentBlock{
            piai.TextContent{Text: "描述这张图片"},
            piai.ImageContent{Data: base64Data, MimeType: "image/jpeg"},
        },
    },
})
```

## 环境变量

| 环境变量 | 服务商 |
|----------|--------|
| `ANTHROPIC_API_KEY` | Anthropic |
| `OPENAI_API_KEY` | OpenAI |
| `GOOGLE_API_KEY` / `GEMINI_API_KEY` | Google |
| `MISTRAL_API_KEY` | Mistral |
| `AZURE_OPENAI_API_KEY` | Azure OpenAI |
| `DEEPSEEK_API_KEY` | DeepSeek |
| `GROQ_API_KEY` | Groq |
| `XAI_API_KEY` | xAI |
| `OPENROUTER_API_KEY` | OpenRouter |
| `FIREWORKS_API_KEY` | Fireworks |
| `TOGETHER_API_KEY` | Together |
| `CEREBRAS_API_KEY` | Cerebras |
| `CLOUDFLARE_API_KEY` | Cloudflare |
| `HUGGINGFACE_API_KEY` / `HF_API_TOKEN` | HuggingFace |
| `MOONSHOT_API_KEY` | Moonshot |
| `MINIMAX_API_KEY` | Minimax |
| `XIAOMI_API_KEY` / `MI_API_KEY` | Xiaomi |

也可在代码中直接指定：

```go
piai.Complete(ctx, model, msgs, piai.StreamOptions{APIKey: "your-key"})
```

## 项目结构

```
pi-ai-go/
├── piai.go                 # Facade 入口，re-export core + llm
│
├── core/                   # 核心层：纯类型 + 工具契约，零依赖
│   ├── types.go            #   类型定义 (Message, Model, Tool, AgentTool...)
│   ├── events.go           #   EventStream 泛型 + 流式事件
│   ├── registry.go         #   Provider 注册表 + APIProvider 接口
│   ├── errors.go           #   错误类型体系
│   ├── retry.go            #   自动重试 + 指数退避
│   └── env.go              #   环境变量 API Key 解析
│
├── llm/                    # LLM 调用层：公开 API + 模型管理
│   ├── api.go              #   Stream / Complete / GenerateImages
│   └── models.go           #   Model 注册表 + ThinkingLevel 工具
│
├── providers/              # Provider 实现层
│   ├── register.go         #   内置 Provider 注册
│   ├── openai/             #   OpenAI Responses / Azure / Codex
│   │   └── format/         #     OpenAI 消息格式转换
│   ├── anthropic/          #   Anthropic Messages
│   ├── bedrock/            #   Amazon Bedrock
│   ├── google/             #   Google Gemini / Vertex
│   ├── mistral/            #   Mistral
│   ├── compat/             #   OpenAI 兼容路由（DeepSeek/Kimi/Xiaomi/GLM）
│   └── openrouter/         #   OpenRouter 图像生成
│
├── agent/                  # Agent 智能体层
│   ├── types.go            #   Agent 事件/配置（类型别名 → core）
│   ├── agent.go            #   Agent 状态管理
│   ├── agent-loop.go       #   核心循环 + 工具执行
│   ├── context.go          #   上下文窗口管理 + 压缩
│   ├── session/            #   会话持久化、技能、提示模板
│   └── tools/              #   内置工具 (read/write/edit/bash/glob/grep)
│
├── internal/               # 内部工具包（仅本模块可见）
│   ├── oauth/              #   OAuth 认证
│   ├── validation/         #   工具调用参数校验
│   ├── overflow/           #   Context 溢出检测
│   ├── jsonparse/          #   JSON 修复解析
│   ├── sanitize/           #   Unicode 清理
│   ├── hash/               #   短哈希
│   └── diagnostics/        #   诊断工具
│
├── cmd/                    # CLI 工具
├── examples/               # 示例代码
└── docs/                   # 文档
```

### 依赖关系

```
core/       ← 零依赖 (纯类型 + 注册表 + 工具契约)
  ↑
llm/        ← 仅依赖 core
  ↑
providers/  ← 仅依赖 core
  ↑
agent/      ← 依赖 core + llm
  ↑
piai.go     ← 依赖 core + llm (facade re-export)
```

## 运行测试

```bash
go test ./...                    # 所有测试
go test -race ./...              # 竞态检测
go test -v ./providers/openai/... # 特定包
go vet ./...                     # 静态分析
```

## CLI 工具

```bash
go build -o pi-ai ./cmd/pi-ai
./pi-ai list           # 列出 OAuth 服务商
./pi-ai login anthropic  # 登录
```

## 许可证

MIT License
