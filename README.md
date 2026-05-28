# pi-ai-go

统一的多模型 AI API Go 语言 SDK，是对 TypeScript 包 `@earendil-works/pi-ai` 的完整重写。

## 项目简介

`pi-ai-go` 提供了一个统一的接口，用于流式（streaming）和完成式（completion）文本生成，支持多种 AI 服务商 API。通过抽象不同服务商的差异，开发者可以用一套代码无缝切换不同的大语言模型。

### 核心特性

- **统一接口** - 一套 API 调用所有支持的模型
- **流式响应** - 基于 Go channel 的异步事件流
- **多模态支持** - 支持文本、图像输入
- **工具调用** - 统一的 Function Calling 接口
- **思维链** - 支持推理模型的 thinking/reasoning 功能
- **OAuth 认证** - 内置 Anthropic、GitHub Copilot、OpenAI Codex 的 OAuth 流程
- **零外部依赖** - 核心功能仅使用 Go 标准库

## 支持的服务商

| 服务商 | API 类型 | 包路径 |
|--------|----------|--------|
| Anthropic | Messages API | `providers/anthropic` |
| OpenAI | Chat Completions API | `providers/openai` |
| OpenAI | Responses API | `providers/openai` |
| Azure OpenAI | Responses API | `providers/openai` |
| OpenAI Codex | Responses API | `providers/openai` |
| Google Gemini | Generative AI API | `providers/google` |
| Google Vertex | Vertex AI API | `providers/google` |
| Amazon Bedrock | Converse Stream API | `providers/bedrock` |
| Mistral | Conversations API | `providers/mistral` |
| OpenRouter | 图像生成 API | `providers/images` |

## 安装

```bash
go get pi-ai-go
```

## 快速开始

### 基本使用

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
	// 获取模型
	model, err := piai.GetModel(piai.ProviderAnthropic, "claude-3-opus")
	if err != nil {
		log.Fatal(err)
	}

	// 发送请求并等待完整响应
	msg, err := piai.CompleteSimple(context.Background(), model, []piai.Message{
		piai.UserMessage{
			Role:    "user",
			Content: "你好，请介绍一下自己",
		},
	}, piai.SimpleStreamOptions{
		Reasoning: piai.ThinkingMedium, // 设置推理深度
	})
	if err != nil {
		log.Fatal(err)
	}

	// 输出响应
	for _, block := range msg.Content {
		if text, ok := block.(piai.TextContent); ok {
			fmt.Println(text.Text)
		}
	}
}
```

### 流式响应

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
	model, _ := piai.GetModel(piai.ProviderOpenAI, "gpt-4o")

	// 创建流式请求
	stream, err := piai.StreamSimple(context.Background(), model, []piai.Message{
		piai.UserMessage{
			Role:    "user",
			Content: "写一首关于春天的诗",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// 处理流式事件
	for event := range stream.Events() {
		switch e := event.(type) {
		case piai.EventTextDelta:
			fmt.Print(e.Delta) // 实时输出文本
		case piai.EventThinkingDelta:
			fmt.Printf("[思考] %s", e.Delta)
		case piai.EventToolCallStart:
			fmt.Printf("\n[调用工具] %s\n", e.Name)
		case piai.EventDone:
			fmt.Println("\n\n--- 完成 ---")
			fmt.Printf("Token 用量: 输入=%d, 输出=%d\n", 
				e.Message.Usage.Input, 
				e.Message.Usage.Output)
			fmt.Printf("费用: $%.4f\n", e.Message.Usage.Cost.Total)
		case piai.EventError:
			log.Printf("错误: %v", e.Error)
		}
	}
}
```

### 工具调用（Function Calling）

```go
// 定义工具
tools := []piai.Tool{
	{
		Name:        "get_weather",
		Description: "获取指定城市的天气信息",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"city": {
					"type": "string",
					"description": "城市名称"
				}
			},
			"required": ["city"]
		}`),
	},
}

// 发送带工具的请求
stream, _ := piai.Stream(ctx, model, []piai.Message{
	{Role: "user", Content: "北京今天天气怎么样？"},
}, piai.StreamOptions{})

// 处理工具调用
for event := range stream.Events() {
	switch e := event.(type) {
	case piai.EventToolCallEnd:
		// 解析工具参数
		var args struct {
			City string `json:"city"`
		}
		json.Unmarshal(e.Arguments, &args)
		
		// 执行工具逻辑
		result := getWeather(args.City)
		
		// 添加工具结果并继续对话
		messages = append(messages, 
			piai.ToolResultMessage{
				ToolCallID: e.ID,
				ToolName:   "get_weather",
				Content: []piai.ContentBlock{
					piai.TextContent{Text: result},
				},
			},
		)
	}
}
```

### 多模态（图像输入）

```go
// 读取图片并编码为 base64
imageData, _ := os.ReadFile("photo.jpg")
base64Data := base64.StdEncoding.EncodeToString(imageData)

msg, _ := piai.CompleteSimple(ctx, model, []piai.Message{
	piai.UserMessage{
		Role: "user",
		Content: []piai.ContentBlock{
			piai.TextContent{Text: "描述这张图片"},
			piai.ImageContent{
				Data:     base64Data,
				MimeType: "image/jpeg",
			},
		},
	},
})
```

### 图像生成

```go
import "pi-ai-go/providers/images"

// 获取图像模型
imgModel, _ := piai.GetImageModel(piai.ProviderOpenRouter, "flux-pro")

// 生成图像
result, err := piai.GenerateImages(ctx, imgModel, []piai.Message{
	piai.UserMessage{Content: "一只可爱的橘猫"},
})
if err != nil {
	log.Fatal(err)
}

// 保存图像
for i, img := range result.Output {
	data, _ := base64.StdEncoding.DecodeString(img.Data)
	os.WriteFile(fmt.Sprintf("output_%d.png", i), data, 0644)
}
```

## 环境变量配置

服务商 API Key 会自动从环境变量读取：

| 环境变量 | 服务商 |
|----------|--------|
| `ANTHROPIC_API_KEY` | Anthropic |
| `OPENAI_API_KEY` | OpenAI |
| `GOOGLE_API_KEY` / `GEMINI_API_KEY` | Google Gemini |
| `MISTRAL_API_KEY` | Mistral |
| `AZURE_OPENAI_API_KEY` | Azure OpenAI |
| `OPENROUTER_API_KEY` | OpenRouter |
| `FIREWORKS_API_KEY` | Fireworks |
| `TOGETHER_API_KEY` | Together |
| `GROQ_API_KEY` | Groq |
| `XAI_API_KEY` | xAI |
| `DEEPSEEK_API_KEY` | DeepSeek |

也可以在代码中直接指定：

```go
msg, _ := piai.Complete(ctx, model, messages, piai.StreamOptions{
	APIKey: "your-api-key-here",
})
```

## OAuth 认证

支持 OAuth 登录的服务商：

```go
import "pi-ai-go/utils/oauth"

// Anthropic OAuth 登录
creds, err := oauth.Login(ctx, "anthropic", oauth.LoginCallbacks{
	OnAuth: func(url string) {
		fmt.Printf("请在浏览器中打开: %s\n", url)
	},
})

// GitHub Copilot 登录
creds, err := oauth.Login(ctx, "github-copilot", oauth.LoginCallbacks{
	OnDeviceCode: func(code, uri string) {
		fmt.Printf("访问: %s\n输入代码: %s\n", uri, code)
	},
})

// 使用凭证获取 API Key
apiKey, _ := oauth.GetAPIKey(ctx, "anthropic", creds)
```

## 高级配置

### 推理深度控制

```go
// 使用 SimpleStreamOptions 控制推理深度
opts := piai.SimpleStreamOptions{
	Reasoning: piai.ThinkingHigh, // minimal, low, medium, high, xhigh
}

// 或使用自定义 thinking budget
opts := piai.SimpleStreamOptions{
	Reasoning: piai.ThinkingMedium,
	ThinkingBudgets: map[string]int{
		"medium": 10000, // 自定义 token 预算
	},
}
```

### 服务商特定选项

```go
// Anthropic 特定选项
import "pi-ai-go/providers/anthropic"

// OpenAI 特定选项
import "pi-ai-go/providers/openai"

// Google 特定选项
import "pi-ai-go/providers/google"
```

### 自定义 Base URL

```go
model := piai.Model{
	ID:      "custom-model",
	BaseURL: "https://your-proxy.com/v1",
	// ...
}
```

## 项目结构

```
pi-ai-go/
├── pi.go                      # 主入口：Stream, Complete 等函数
├── types.go                   # 核心类型定义
├── models.go                  # 模型注册表
├── api-registry.go            # API 服务商注册表
├── images.go                  # 图像生成 API
├── env-api-keys.go            # 环境变量 API Key 解析
├── session-resources.go       # 会话资源清理
├── eventstream.go             # 异步事件流
│
├── providers/                 # 服务商实现
│   ├── provider.go            # Provider 接口定义
│   ├── register.go            # 服务商注册
│   ├── transform.go           # 跨服务商消息转换
│   ├── simple-options.go      # 共享选项构建器
│   ├── anthropic/             # Anthropic Messages API
│   ├── openai/                # OpenAI 系列 API
│   │   ├── completions.go     # Chat Completions
│   │   ├── responses.go       # Responses API
│   │   ├── azure.go           # Azure OpenAI
│   │   └── codex.go           # OpenAI Codex
│   ├── google/                # Google 系列 API
│   │   ├── google.go          # Gemini
│   │   └── vertex.go          # Vertex AI
│   ├── bedrock/               # Amazon Bedrock
│   ├── mistral/               # Mistral
│   └── images/                # 图像生成
│       └── openrouter.go      # OpenRouter
│
├── utils/                     # 工具包
│   ├── eventstream/           # 通用异步事件流
│   ├── diagnostics/           # 诊断工具
│   ├── hash/                  # 快速哈希
│   ├── jsonparse/             # JSON 修复解析
│   ├── sanitize/              # Unicode 清理
│   ├── overflow/              # 上下文溢出检测
│   ├── validation/            # 工具调用验证
│   └── oauth/                 # OAuth 认证
│       ├── anthropic.go       # Anthropic OAuth
│       ├── github-copilot.go  # GitHub Copilot OAuth
│       └── openai-codex.go    # OpenAI Codex OAuth
│
├── cmd/pi-ai/                 # CLI 工具
│   └── main.go                # OAuth 登录命令行
│
└── docs/                      # 文档
    ├── DESIGN.md              # 架构设计文档
    └── API.md                 # API 参考文档
```

## 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./providers/anthropic/...

# 运行测试并显示详细输出
go test -v ./...

# 运行测试并显示覆盖率
go test -cover ./...
```

## CLI 工具

项目包含一个命令行工具用于 OAuth 登录：

```bash
# 构建 CLI
go build -o pi-ai ./cmd/pi-ai

# 列出可用的 OAuth 服务商
./pi-ai list

# 登录到 Anthropic
./pi-ai login anthropic

# 登录到 GitHub Copilot
./pi-ai login github-copilot
```

## 核心类型

### Model（模型）

```go
type Model struct {
    ID           string            // 模型 ID
    Name         string            // 显示名称
    API          KnownAPI          // API 类型
    Provider     KnownProvider     // 服务商
    BaseURL      string            // 自定义 Base URL
    Reasoning    bool              // 是否支持推理
    Input        []Modality        // 支持的输入模态
    Cost         Cost              // 定价
    ContextWindow int              // 上下文窗口大小
    MaxTokens    int               // 最大输出 token
}
```

### Message（消息）

```go
// 用户消息
type UserMessage struct {
    Role      string
    Content   any  // string 或 []ContentBlock
    Timestamp time.Time
}

// 助手消息
type AssistantMessage struct {
    Role       string
    Content    []ContentBlock
    Usage      Usage
    StopReason StopReason
}

// 工具结果消息
type ToolResultMessage struct {
    ToolCallID string
    ToolName   string
    Content    []ContentBlock
    IsError    bool
}
```

### Event（事件）

```go
// 流式事件类型
EventStart          // 开始
EventTextDelta      // 文本增量
EventThinkingDelta  // 思考增量
EventToolCallStart  // 工具调用开始
EventToolCallDelta  // 工具参数增量
EventToolCallEnd    // 工具调用结束
EventDone           // 完成
EventError          // 错误
```

## 设计决策

1. **无外部 SDK 依赖** - 直接使用 `net/http` 调用 API，保持最小依赖
2. **Channel 实现流式** - 使用 Go channel 实现异步事件流
3. **接口驱动** - 通过 Provider 接口实现可扩展性
4. **懒加载** - 按需加载服务商实现
5. **类型安全** - 充分利用 Go 泛型

## 许可证

MIT License
