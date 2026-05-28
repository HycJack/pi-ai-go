# API 参考文档

## 目录

- [核心函数](#核心函数)
- [模型注册表](#模型注册表)
- [服务商注册表](#服务商注册表)
- [图像生成](#图像生成)
- [OAuth 认证](#oauth-认证)
- [工具包](#工具包)
- [类型定义](#类型定义)
- [常量定义](#常量定义)

---

## 核心函数

### Stream

创建流式请求，返回事件流。

```go
func Stream(ctx context.Context, model Model, msgs []Message, opts ...StreamOptions) (*EventStream[AssistantMessageEvent, AssistantMessage], error)
```

**参数：**
- `ctx` - 上下文，支持取消和超时
- `model` - 模型配置
- `msgs` - 消息列表
- `opts` - 可选的流式选项

**返回：**
- `*EventStream` - 异步事件流
- `error` - 错误信息

**示例：**
```go
stream, err := piai.Stream(ctx, model, []piai.Message{
    piai.UserMessage{Content: "你好"},
}, piai.StreamOptions{
    APIKey: "your-api-key",
})

for event := range stream.Events() {
    switch e := event.(type) {
    case piai.EventTextDelta:
        fmt.Print(e.Delta)
    case piai.EventDone:
        fmt.Println("完成")
    }
}
```

---

### Complete

发送请求并等待完整响应。

```go
func Complete(ctx context.Context, model Model, msgs []Message, opts ...StreamOptions) (AssistantMessage, error)
```

**参数：**
- `ctx` - 上下文
- `model` - 模型配置
- `msgs` - 消息列表
- `opts` - 可选的流式选项

**返回：**
- `AssistantMessage` - 完整的助手消息
- `error` - 错误信息

**示例：**
```go
msg, err := piai.Complete(ctx, model, []piai.Message{
    piai.UserMessage{Content: "你好"},
})
if err != nil {
    log.Fatal(err)
}

for _, block := range msg.Content {
    if text, ok := block.(piai.TextContent); ok {
        fmt.Println(text.Text)
    }
}
```

---

### StreamSimple

使用简化选项创建流式请求。

```go
func StreamSimple(ctx context.Context, model Model, msgs []Message, opts ...SimpleStreamOptions) (*EventStream[AssistantMessageEvent, AssistantMessage], error)
```

**参数：**
- `ctx` - 上下文
- `model` - 模型配置
- `msgs` - 消息列表
- `opts` - 简化选项（包含推理深度等）

**示例：**
```go
stream, err := piai.StreamSimple(ctx, model, messages, piai.SimpleStreamOptions{
    Reasoning: piai.ThinkingMedium,
    StreamOptions: piai.StreamOptions{
        APIKey: "your-api-key",
    },
})
```

---

### CompleteSimple

使用简化选项发送请求并等待完整响应。

```go
func CompleteSimple(ctx context.Context, model Model, msgs []Message, opts ...SimpleStreamOptions) (AssistantMessage, error)
```

**示例：**
```go
msg, err := piai.CompleteSimple(ctx, model, []piai.Message{
    piai.UserMessage{Content: "解释量子计算"},
}, piai.SimpleStreamOptions{
    Reasoning: piai.ThinkingHigh,
})
```

---

## 模型注册表

### GetModel

根据服务商和模型 ID 获取模型配置。

```go
func GetModel(provider KnownProvider, modelID string) (Model, error)
```

**参数：**
- `provider` - 服务商名称
- `modelID` - 模型 ID

**返回：**
- `Model` - 模型配置
- `error` - 如果模型不存在则返回错误

**示例：**
```go
model, err := piai.GetModel(piai.ProviderOpenAI, "gpt-4o")
if err != nil {
    log.Fatal(err)
}
```

---

### GetProviders

获取所有已注册的服务商列表。

```go
func GetProviders() []KnownProvider
```

**返回：**
- `[]KnownProvider` - 服务商列表

**示例：**
```go
providers := piai.GetProviders()
for _, p := range providers {
    fmt.Println(p)
}
```

---

### GetModels

获取指定服务商的所有模型。

```go
func GetModels(provider KnownProvider) []Model
```

**参数：**
- `provider` - 服务商名称

**返回：**
- `[]Model` - 模型列表

**示例：**
```go
models := piai.GetModels(piai.ProviderAnthropic)
for _, m := range models {
    fmt.Printf("%s: %s\n", m.ID, m.Name)
}
```

---

### CalculateCost

计算请求费用。

```go
func CalculateCost(model Model, usage Usage) CostBreakdown
```

**参数：**
- `model` - 模型配置（包含定价信息）
- `usage` - Token 用量

**返回：**
- `CostBreakdown` - 费用明细

**示例：**
```go
cost := piai.CalculateCost(model, msg.Usage)
fmt.Printf("输入费用: $%.4f\n", cost.Input)
fmt.Printf("输出费用: $%.4f\n", cost.Output)
fmt.Printf("总费用: $%.4f\n", cost.Total)
```

---

### GetSupportedThinkingLevels

获取模型支持的思考级别。

```go
func GetSupportedThinkingLevels(model Model) []ThinkingLevel
```

**参数：**
- `model` - 模型配置

**返回：**
- `[]ThinkingLevel` - 支持的思考级别列表，如果不支持则返回 nil

---

### ClampThinkingLevel

将思考级别限制为模型支持的最接近级别。

```go
func ClampThinkingLevel(model Model, level ThinkingLevel) ThinkingLevel
```

**参数：**
- `model` - 模型配置
- `level` - 请求的思考级别

**返回：**
- `ThinkingLevel` - 调整后的思考级别

---

## 服务商注册表

### RegisterProvider

注册 API 服务商。

```go
func RegisterProvider(api KnownAPI, provider Provider, sourceID ...string)
```

**参数：**
- `api` - API 类型标识
- `provider` - Provider 实现
- `sourceID` - 可选的来源 ID，用于批量注销

**示例：**
```go
piai.RegisterProvider(piai.APIOpenAICompletions, myProvider, "my-plugin")
```

---

### GetProvider

获取 API 服务商。

```go
func GetProvider(api KnownAPI) (Provider, error)
```

**参数：**
- `api` - API 类型标识

**返回：**
- `Provider` - Provider 实现
- `error` - 如果未注册则返回错误

---

### GetRegisteredProviders

获取所有已注册的 API 类型。

```go
func GetRegisteredProviders() []KnownAPI
```

---

### UnregisterProviders

注销指定来源的所有服务商。

```go
func UnregisterProviders(sourceID string)
```

**参数：**
- `sourceID` - 来源 ID

---

### ClearProviders

清空所有已注册的服务商。

```go
func ClearProviders()
```

---

## 图像生成

### GenerateImages

生成图像。

```go
func GenerateImages(ctx context.Context, model ImagesModel, msgs []Message, opts ...ImageOptions) (AssistantImages, error)
```

**参数：**
- `ctx` - 上下文
- `model` - 图像模型配置
- `msgs` - 消息列表（通常包含提示词）
- `opts` - 图像选项

**返回：**
- `AssistantImages` - 生成的图像结果
- `error` - 错误信息

**示例：**
```go
imgModel, _ := piai.GetImageModel(piai.ProviderOpenRouter, "flux-pro")

result, err := piai.GenerateImages(ctx, imgModel, []piai.Message{
    piai.UserMessage{Content: "一只可爱的橘猫"},
})
if err != nil {
    log.Fatal(err)
}

for i, img := range result.Output {
    data, _ := base64.StdEncoding.DecodeString(img.Data)
    os.WriteFile(fmt.Sprintf("image_%d.png", i), data, 0644)
}
```

---

## OAuth 认证

### Login

启动 OAuth 登录流程。

```go
func Login(ctx context.Context, providerID string, callbacks LoginCallbacks) (Credentials, error)
```

**参数：**
- `ctx` - 上下文
- `providerID` - 服务商 ID（"anthropic", "github-copilot", "openai-codex"）
- `callbacks` - 登录回调函数

**返回：**
- `Credentials` - OAuth 凭证
- `error` - 错误信息

**示例：**
```go
creds, err := oauth.Login(ctx, "anthropic", oauth.LoginCallbacks{
    OnAuth: func(url string) {
        fmt.Printf("请在浏览器中打开: %s\n", url)
    },
    OnProgress: func(msg string) {
        fmt.Print(".")
    },
})
```

---

### GetAPIKey

获取有效的 API Key，自动刷新过期的 token。

```go
func GetAPIKey(ctx context.Context, providerID string, credentials Credentials) (string, error)
```

**参数：**
- `ctx` - 上下文
- `providerID` - 服务商 ID
- `credentials` - OAuth 凭证

**返回：**
- `string` - 有效的 API Key
- `error` - 错误信息

---

### List

列出所有已注册的 OAuth 服务商。

```go
func List() []string
```

**返回：**
- `[]string` - 服务商 ID 列表

---

## 工具包

### EventStream

异步事件流，基于 Go channel 实现。

```go
type Stream[T any, R any] struct {
    // ...
}

// 推送事件
func (s *Stream[T, R]) Push(event T)

// 正常结束
func (s *Stream[T, R]) End(result R)

// 错误结束
func (s *Stream[T, R]) Error(err error)

// 等待结果
func (s *Stream[T, R]) Result() (R, error)

// 迭代事件
func (s *Stream[T, R]) ForEach(ctx context.Context, fn func(T) error) (R, error)
```

**示例：**
```go
stream := piai.NewEventStream[string, int]()

go func() {
    stream.Push("hello")
    stream.Push("world")
    stream.End(42)
}()

result, err := stream.ForEach(ctx, func(s string) error {
    fmt.Println(s)
    return nil
})
// result = 42
```

---

### ParseJson

JSON 修复解析，处理不完整的 JSON。

```go
func Parse[T any](data string) (T, error)
func Streaming[T any](partial string) (T, bool)
```

**示例：**
```go
// 解析不完整的 JSON
result, ok := jsonparse.Streaming[map[string]string](`{"name":"test","value":`)
// ok = true, result = {"name": "test"}
```

---

### ValidateToolCall

验证工具调用参数。

```go
func ValidateToolCall(tools []ToolDef, call ToolCall) (*ToolDef, ValidationResult)
```

**参数：**
- `tools` - 工具定义列表
- `call` - 工具调用

**返回：**
- `*ToolDef` - 匹配的工具定义
- `ValidationResult` - 验证结果

---

### IsOverflow

检测上下文溢出。

```go
func IsOverflow(errMsg string, contextWindow int, usage int) bool
```

**参数：**
- `errMsg` - 错误消息
- `contextWindow` - 上下文窗口大小
- `usage` - 当前 usage

**返回：**
- `bool` - 是否溢出

---

## 类型定义

### KnownAPI

```go
type KnownAPI string

const (
    APIOpenAICompletions    KnownAPI = "openai-completions"
    APIAnthropicMessages    KnownAPI = "anthropic-messages"
    APIBedrockConverse      KnownAPI = "bedrock-converse-stream"
    APIOpenAIResponses      KnownAPI = "openai-responses"
    APIAzureOpenAIResponses KnownAPI = "azure-openai-responses"
    APIOpenAICodexResponses KnownAPI = "openai-codex-responses"
    APIGoogleGenerative     KnownAPI = "google-generative"
    APIGoogleVertex         KnownAPI = "google-vertex"
    APIMistralConversations KnownAPI = "mistral-conversations"
)
```

---

### KnownProvider

```go
type KnownProvider string

const (
    ProviderAnthropic     KnownProvider = "anthropic"
    ProviderOpenAI        KnownProvider = "openai"
    ProviderAmazonBedrock KnownProvider = "amazon-bedrock"
    ProviderGoogle        KnownProvider = "google"
    ProviderGoogleVertex  KnownProvider = "google-vertex"
    ProviderMistral       KnownProvider = "mistral"
    ProviderAzureOpenAI   KnownProvider = "azure-openai"
    ProviderOpenAICodex   KnownProvider = "openai-codex"
    ProviderGitHubCopilot KnownProvider = "github-copilot"
    ProviderOpenRouter    KnownProvider = "openrouter"
    ProviderDeepSeek      KnownProvider = "deepseek"
    // ... 更多服务商
)
```

---

### ThinkingLevel

```go
type ThinkingLevel string

const (
    ThinkingMinimal ThinkingLevel = "minimal"
    ThinkingLow     ThinkingLevel = "low"
    ThinkingMedium  ThinkingLevel = "medium"
    ThinkingHigh    ThinkingLevel = "high"
    ThinkingXHigh   ThinkingLevel = "xhigh"
)
```

---

### StopReason

```go
type StopReason string

const (
    StopStop    StopReason = "stop"     // 正常停止
    StopLength  StopReason = "length"   // 达到最大长度
    StopToolUse StopReason = "toolUse"  // 工具调用
    StopError   StopReason = "error"    // 错误
    StopAborted StopReason = "aborted"  // 中止
)
```

---

### Modality

```go
type Modality string

const (
    ModalityText  Modality = "text"   // 文本
    ModalityImage Modality = "image"  // 图像
    ModalityAudio Modality = "audio"  // 音频
)
```

---

### CacheRetention

```go
type CacheRetention string

const (
    CacheNone  CacheRetention = "none"   // 不缓存
    CacheShort CacheRetention = "short"  // 短期缓存
    CacheLong  CacheRetention = "long"   // 长期缓存
)
```

---

### Transport

```go
type Transport string

const (
    TransportSSE       Transport = "sse"        // Server-Sent Events
    TransportWebSocket Transport = "websocket"   // WebSocket
    TransportAuto      Transport = "auto"        // 自动选择
)
```

---

### StreamOptions

```go
type StreamOptions struct {
    Temperature     *float64          // 温度参数
    MaxTokens       *int              // 最大输出 token
    Signal          <-chan struct{}    // 取消信号
    APIKey          string            // API Key
    Transport       Transport         // 传输方式
    CacheRetention  CacheRetention    // 缓存策略
    SessionID       string            // 会话 ID
    OnPayload       func(any)         // 请求体回调
    OnResponse      func(any)         // 响应回调
    Headers         map[string]string // 自定义请求头
    TimeoutMs       int               // 超时时间（毫秒）
    MaxRetries      int               // 最大重试次数
    MaxRetryDelayMs int               // 最大重试延迟（毫秒）
    Metadata        map[string]any    // 元数据
}
```

---

### SimpleStreamOptions

```go
type SimpleStreamOptions struct {
    StreamOptions                   // 嵌入基础选项
    Reasoning       ThinkingLevel   // 推理深度
    ThinkingBudgets map[string]int  // 自定义思考预算
}
```

---

### Usage

```go
type Usage struct {
    Input       int           // 输入 token 数
    Output      int           // 输出 token 数
    CacheRead   int           // 缓存读取 token 数
    CacheWrite  int           // 缓存写入 token 数
    TotalTokens int           // 总 token 数
    Cost        CostBreakdown // 费用明细
}
```

---

### CostBreakdown

```go
type CostBreakdown struct {
    Input      float64 // 输入费用
    Output     float64 // 输出费用
    CacheRead  float64 // 缓存读取费用
    CacheWrite float64 // 缓存写入费用
    Total      float64 // 总费用
}
```

---

## 常量定义

### API 类型

| 常量 | 值 | 说明 |
|------|-----|------|
| `APIOpenAICompletions` | `"openai-completions"` | OpenAI Chat Completions API |
| `APIAnthropicMessages` | `"anthropic-messages"` | Anthropic Messages API |
| `APIBedrockConverse` | `"bedrock-converse-stream"` | Amazon Bedrock Converse Stream API |
| `APIOpenAIResponses` | `"openai-responses"` | OpenAI Responses API |
| `APIAzureOpenAIResponses` | `"azure-openai-responses"` | Azure OpenAI Responses API |
| `APIOpenAICodexResponses` | `"openai-codex-responses"` | OpenAI Codex Responses API |
| `APIGoogleGenerative` | `"google-generative"` | Google Generative AI API |
| `APIGoogleVertex` | `"google-vertex"` | Google Vertex AI API |
| `APIMistralConversations` | `"mistral-conversations"` | Mistral Conversations API |

### 服务商

| 常量 | 值 | 说明 |
|------|-----|------|
| `ProviderAnthropic` | `"anthropic"` | Anthropic |
| `ProviderOpenAI` | `"openai"` | OpenAI |
| `ProviderAmazonBedrock` | `"amazon-bedrock"` | Amazon Bedrock |
| `ProviderGoogle` | `"google"` | Google |
| `ProviderGoogleVertex` | `"google-vertex"` | Google Vertex AI |
| `ProviderMistral` | `"mistral"` | Mistral AI |
| `ProviderAzureOpenAI` | `"azure-openai"` | Azure OpenAI |
| `ProviderOpenAICodex` | `"openai-codex"` | OpenAI Codex |
| `ProviderGitHubCopilot` | `"github-copilot"` | GitHub Copilot |
| `ProviderOpenRouter` | `"openrouter"` | OpenRouter |
| `ProviderDeepSeek` | `"deepseek"` | DeepSeek |
| `ProviderFireworks` | `"fireworks"` | Fireworks |
| `ProviderTogether` | `"together"` | Together AI |
| `ProviderGroq` | `"groq"` | Groq |
| `ProviderXAI` | `"xai"` | xAI |
