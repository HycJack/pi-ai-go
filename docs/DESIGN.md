# pi-ai-go 架构设计文档

## 项目概述

`pi-ai-go` 是 TypeScript 包 `@earendil-works/pi-ai` 的 Go 语言重写版本。它提供了一个统一的接口，用于在多个 AI 服务商 API 之间进行流式和完成式文本生成。

## 设计目标

1. **统一接口** - 一套代码调用所有支持的 AI 模型
2. **流式优先** - 基于 Go channel 的异步事件流
3. **零依赖** - 核心功能仅使用 Go 标准库
4. **可扩展** - 通过接口轻松添加新的服务商
5. **类型安全** - 利用 Go 泛型提供类型安全

## 项目结构

```
pi-ai-go/
├── pi.go                      # 主入口：Stream, Complete, StreamSimple, CompleteSimple
├── types.go                   # 核心类型定义
├── models.go                  # 模型注册表和工具函数
├── image-models.go            # 图像模型注册表
├── images.go                  # 图像生成 API
├── api-registry.go            # API 服务商注册表
├── images-api-registry.go     # 图像 API 服务商注册表
├── env-api-keys.go            # 环境变量 API Key 解析
├── session-resources.go       # 会话资源清理
├── eventstream.go             # 泛型异步事件流
│
├── providers/                 # 服务商实现
│   ├── provider.go            # Provider 接口定义
│   ├── register.go            # 内置服务商注册
│   ├── transform.go           # 跨服务商消息转换
│   ├── simple-options.go      # 共享选项构建器
│   │
│   ├── anthropic/             # Anthropic Messages API
│   │   └── anthropic.go       # 完整实现
│   │
│   ├── openai/                # OpenAI 系列 API
│   │   ├── shared.go          # 共享工具函数
│   │   ├── completions.go     # Chat Completions API
│   │   ├── responses.go       # Responses API
│   │   ├── azure.go           # Azure OpenAI
│   │   └── codex.go           # OpenAI Codex
│   │
│   ├── google/                # Google 系列 API
│   │   ├── shared.go          # 共享工具函数
│   │   ├── google.go          # Google Gemini
│   │   └── vertex.go          # Google Vertex AI
│   │
│   ├── bedrock/               # Amazon Bedrock
│   │   └── bedrock.go         # Converse Stream API
│   │
│   ├── mistral/               # Mistral AI
│   │   └── mistral.go         # Conversations API
│   │
│   └── images/                # 图像生成
│       └── openrouter.go      # OpenRouter 图像生成
│
├── utils/                     # 工具包
│   ├── eventstream/           # 通用异步事件流
│   │   └── eventstream.go
│   │
│   ├── diagnostics/           # 诊断工具
│   │   └── diagnostics.go
│   │
│   ├── hash/                  # 快速哈希
│   │   └── hash.go
│   │
│   ├── jsonparse/             # JSON 修复解析
│   │   └── jsonparse.go
│   │
│   ├── sanitize/              # Unicode 清理
│   │   └── sanitize.go
│   │
│   ├── overflow/              # 上下文溢出检测
│   │   └── overflow.go
│   │
│   ├── validation/            # 工具调用验证
│   │   └── validation.go
│   │
│   └── oauth/                 # OAuth 认证
│       ├── types.go           # OAuth 类型定义
│       ├── oauth.go           # OAuth 注册表
│       ├── anthropic.go       # Anthropic OAuth
│       ├── github-copilot.go  # GitHub Copilot OAuth
│       ├── openai-codex.go    # OpenAI Codex OAuth
│       ├── device-code.go     # Device Code 流程
│       └── pkce.go            # PKCE 工具
│
├── cmd/pi-ai/                 # CLI 工具
│   └── main.go                # OAuth 登录命令行
│
├── test/                      # 测试示例
│   └── main.go                # 从 .env 加载配置的测试
│
└── docs/                      # 文档
    ├── DESIGN.md              # 本文档
    ├── API.md                 # API 参考文档
    └── QUICKSTART.md          # 快速开始指南
```

## 核心类型

### Model（模型）

```go
type Model struct {
    ID              string            `json:"id"`              // 模型 ID，如 "gpt-4o"
    Name            string            `json:"name"`            // 显示名称
    API             KnownAPI          `json:"api"`             // API 类型
    Provider        KnownProvider     `json:"provider"`        // 服务商
    BaseURL         string            `json:"baseUrl"`         // 自定义 Base URL
    Reasoning       bool              `json:"reasoning"`       // 是否支持推理
    ThinkingLevelMap map[string]string `json:"thinkingLevelMap"` // 思考级别映射
    Input           []Modality        `json:"input"`           // 支持的输入模态
    Cost            Cost              `json:"cost"`            // 定价
    ContextWindow   int               `json:"contextWindow"`   // 上下文窗口大小
    MaxTokens       int               `json:"maxTokens"`       // 最大输出 token
    Headers         map[string]string `json:"headers"`         // 自定义请求头
    Compat          *Compat           `json:"compat"`          // 兼容性配置
}
```

### Message（消息）

消息是一个接口，有三种实现：

```go
type Message interface {
    messageTag()
    GetTimestamp() time.Time
}

// 用户消息
type UserMessage struct {
    Role      string    // 固定为 "user"
    Content   any       // string 或 []ContentBlock
    Timestamp time.Time
}

// 助手消息
type AssistantMessage struct {
    Role          string         // 固定为 "assistant"
    Content       []ContentBlock // 内容块列表
    API           KnownAPI       // 使用的 API
    Provider      KnownProvider  // 使用的服务商
    Model         string         // 使用的模型
    Usage         Usage          // Token 用量
    StopReason    StopReason     // 停止原因
    ErrorMessage  string         // 错误信息
    Timestamp     time.Time
}

// 工具结果消息
type ToolResultMessage struct {
    Role       string         // 固定为 "toolResult"
    ToolCallID string         // 工具调用 ID
    ToolName   string         // 工具名称
    Content    []ContentBlock // 结果内容
    IsError    bool           // 是否错误
    Timestamp  time.Time
}
```

### ContentBlock（内容块）

```go
type ContentBlock interface {
    contentTag()
}

// 文本内容
type TextContent struct {
    Type string // "text"
    Text string
}

// 思考内容（用于推理模型）
type ThinkingContent struct {
    Type     string // "thinking"
    Thinking string
}

// 图像内容
type ImageContent struct {
    Type     string // "image"
    Data     string // base64 编码
    MimeType string // 如 "image/png"
}

// 工具调用
type ToolCall struct {
    Type      string          // "toolCall"
    ID        string          // 调用 ID
    Name      string          // 工具名称
    Arguments json.RawMessage // 参数 JSON
}
```

### Event（流式事件）

```go
type AssistantMessageEvent interface {
    eventTag()
}

// 开始事件
type EventStart struct {
    Type      string
    API       KnownAPI
    Provider  KnownProvider
    Model     string
    Timestamp time.Time
}

// 文本增量
type EventTextDelta struct {
    Type  string // "text_delta"
    Delta string // 增量文本
}

// 思考增量
type EventThinkingDelta struct {
    Type  string // "thinking_delta"
    Delta string // 增量思考
}

// 工具调用开始
type EventToolCallStart struct {
    Type string // "toolcall_start"
    ID   string
    Name string
}

// 工具参数增量
type EventToolCallDelta struct {
    Type           string // "toolcall_delta"
    ID             string
    ArgumentsDelta string
}

// 工具调用结束
type EventToolCallEnd struct {
    Type      string // "toolcall_end"
    ID        string
    Arguments json.RawMessage
}

// 完成事件
type EventDone struct {
    Type    string           // "done"
    Message AssistantMessage // 最终消息
}

// 错误事件
type EventError struct {
    Type  string // "error"
    Error error
}
```

## 数据流

```
┌─────────────────────────────────────────────────────────────────┐
│                         用户代码                                  │
│    stream(model, messages, options) 或 complete(...)             │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                      pi.go (主入口)                               │
│    1. 从 api-registry 查找 Provider                              │
│    2. 调用 provider.Stream(model, context, options)              │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Provider 实现                                   │
│    1. 转换消息格式 (transform.go)                                 │
│    2. 构建请求体                                                  │
│    3. 发送 HTTP 请求                                              │
│    4. 解析 SSE 响应                                               │
│    5. 推送事件到 EventStream                                      │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    EventStream                                   │
│    - 使用 Go channel 实现                                         │
│    - 支持 Push/End/Error 操作                                     │
│    - 支持 ForEach 迭代                                            │
│    - 支持 Result 等待最终结果                                      │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                       用户代码                                    │
│    for event := range stream.Events() { ... }                   │
│    result, err := stream.Result()                               │
└─────────────────────────────────────────────────────────────────┘
```

## Provider 接口

```go
type Provider interface {
    // Stream 开始流式请求
    Stream(model Model, context Context, opts StreamOptions) (*EventStream, error)
    
    // StreamSimple 使用简化选项开始流式请求
    StreamSimple(model Model, context Context, opts SimpleStreamOptions) (*EventStream, error)
}
```

### 实现新 Provider 的步骤

1. **创建包目录**
   ```
   providers/your-provider/
   └── your-provider.go
   ```

2. **实现 Provider 接口**
   ```go
   type YourProvider struct{}
   
   func (p *YourProvider) Stream(model piai.Model, ctx piai.Context, opts piai.StreamOptions) (*piai.EventStream, error) {
       // 实现流式请求
   }
   
   func (p *YourProvider) StreamSimple(model piai.Model, ctx piai.Context, opts piai.SimpleStreamOptions) (*piai.EventStream, error) {
       // 映射简单选项到服务商特定选项
   }
   ```

3. **注册 Provider**
   ```go
   // 在 providers/register.go 中添加
   piai.RegisterProvider(piai.APIYourProvider, yourprovider.New(), "builtin")
   ```

4. **添加测试**
   ```go
   // providers/your-provider/your-provider_test.go
   func TestStream(t *testing.T) {
       // 测试实现
   }
   ```

## 注册表模式

项目使用两个全局注册表：

### API Provider 注册表

```go
// 注册
RegisterProvider(api KnownAPI, provider Provider, sourceID ...string)

// 查找
GetProvider(api KnownAPI) (Provider, error)

// 列出
GetRegisteredProviders() []KnownAPI

// 注销
UnregisterProviders(sourceID string)

// 清空
ClearProviders()
```

### Images API Provider 注册表

```go
// 注册
RegisterImagesProvider(api KnownAPI, provider ImagesAPIProvider, sourceID ...string)

// 查找
GetImagesProvider(api KnownAPI) (ImagesAPIProvider, error)
```

## 消息转换

`transform.go` 提供跨服务商的消息转换：

1. **图像降级** - 对不支持图像的模型，移除图像内容
2. **思考块处理** - 相同模型保留思考块，不同模型转换为文本
3. **工具调用 ID 规范化** - 为跨服务商兼容性规范化 ID
4. **孤立工具结果处理** - 为孤立的工具结果插入空结果
5. **错误消息跳过** - 跳过错误/中止的助手消息

## 环境变量

`env-api-keys.go` 负责从环境变量解析 API Key：

```go
// 解析优先级
func ResolveAPIKey(provider KnownProvider, optsKey string) string {
    if optsKey != "" {
        return optsKey  // 1. 代码中直接指定
    }
    return GetEnvAPIKey(provider)  // 2. 环境变量
}
```

服务商与环境变量的映射关系：

| 服务商 | 环境变量 |
|--------|----------|
| Anthropic | `ANTHROPIC_OAUTH_TOKEN`, `ANTHROPIC_API_KEY` |
| OpenAI | `OPENAI_API_KEY` |
| Google | `GOOGLE_API_KEY`, `GEMINI_API_KEY` |
| Mistral | `MISTRAL_API_KEY` |
| Azure | `AZURE_OPENAI_API_KEY` |
| OpenRouter | `OPENROUTER_API_KEY` |

## OAuth 认证

`utils/oauth/` 包提供 OAuth 认证支持：

### 支持的 OAuth 流程

1. **Authorization Code + PKCE** - Anthropic, OpenAI Codex
2. **Device Code Flow** - GitHub Copilot

### OAuth Provider 接口

```go
type ProviderInterface struct {
    ID           string
    Name         string
    Login        func(ctx context.Context, callbacks LoginCallbacks) (Credentials, error)
    RefreshToken func(ctx context.Context, credentials Credentials) (Credentials, error)
    GetAPIKey    func(ctx context.Context, credentials Credentials) (string, error)
}
```

## 工具包说明

### eventstream

通用异步事件流，基于 Go channel 实现：

```go
type Stream[T any, R any] struct {
    ch   chan T
    done chan struct{}
}

func (s *Stream[T, R]) Push(event T)  // 推送事件
func (s *Stream[T, R]) End(result R)  // 正常结束
func (s *Stream[T, R]) Error(err error) // 错误结束
func (s *Stream[T, R]) Result() (R, error) // 等待结果
func (s *Stream[T, R]) ForEach(fn func(T) error) (R, error) // 迭代事件
```

### jsonparse

JSON 修复解析器，处理：

- 未转义的控制字符
- 错误的转义序列
- 不完整的 JSON（流式场景）

### overflow

上下文溢出检测，支持：

- 基于错误消息的检测（20+ 种模式）
- 基于 usage 的静默溢出检测
- 基于 stop reason 的溢出检测

### validation

工具调用验证，支持：

- JSON Schema 验证
- 类型强制转换
- 联合类型解析（anyOf/oneOf/allOf）

## 设计决策

1. **无外部 SDK 依赖**
   - 直接使用 `net/http` 调用 API
   - 避免 SDK 版本锁定问题
   - 保持最小依赖 footprint

2. **Channel 实现流式**
   - 符合 Go 并发模式
   - 支持背压处理
   - 支持 context 取消

3. **泛型 EventStream**
   - 类型安全的事件流
   - 编译时类型检查
   - 避免类型断言

4. **接口驱动设计**
   - Provider 接口定义标准行为
   - 易于测试和扩展
   - 支持自定义实现

5. **环境变量配置**
   - 符合 12-Factor App 原则
   - 支持 `.env` 文件
   - 优先级明确

## 测试策略

1. **单元测试** - 每个文件都有对应的 `_test.go`
2. **集成测试** - 使用真实 API Key 测试（可选）
3. **Mock 测试** - 使用 `httptest.Server` 模拟 API 响应
4. **边界测试** - 测试错误处理、取消、超时等场景

## 性能考虑

1. **懒加载** - 服务商按需初始化
2. **连接复用** - HTTP client 复用
3. **流式处理** - 避免大响应的内存占用
4. **零拷贝** - 尽量避免不必要的数据复制
