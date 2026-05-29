# API 参考文档

所有函数通过 `import piai "pi-ai-go"` 访问。也可按层导入：`core/`、`ai/`、`agent/`。

---

## 公开 API

### Stream

创建流式请求，返回事件流。

```go
func Stream(ctx context.Context, model Model, msgs []Message, opts ...StreamOptions) (*core.EventStream[AssistantMessageEvent, AssistantMessage], error)
```

### Complete

发送请求并等待完整响应（内部调用 Stream + Result）。

```go
func Complete(ctx context.Context, model Model, msgs []Message, opts ...StreamOptions) (AssistantMessage, error)
```

### StreamSimple

使用简化选项（含推理深度）创建流式请求。

```go
func StreamSimple(ctx context.Context, model Model, msgs []Message, opts ...SimpleStreamOptions) (*core.EventStream[AssistantMessageEvent, AssistantMessage], error)
```

### StreamSimpleWithContext

使用完整 Context（含 system prompt 和 tools）创建流式请求。

```go
func StreamSimpleWithContext(ctx context.Context, model Model, llmCtx Context, opts ...SimpleStreamOptions) (*core.EventStream[AssistantMessageEvent, AssistantMessage], error)
```

### CompleteSimple

简化选项 + 等待完整响应。

```go
func CompleteSimple(ctx context.Context, model Model, msgs []Message, opts ...SimpleStreamOptions) (AssistantMessage, error)
```

### GenerateImages

生成图像。

```go
func GenerateImages(ctx context.Context, model ImagesModel, msgs []Message, opts ...ImageOptions) (AssistantImages, error)
```

---

## EventStream

泛型异步事件流。

```go
type EventStream[T any, R any] struct { ... }

func NewEventStream[T any, R any]() *EventStream[T, R]
func (s *EventStream[T, R]) Push(event T) bool       // 推送事件，buffer 满返回 false
func (s *EventStream[T, R]) End(result R)             // 正常结束
func (s *EventStream[T, R]) Error(err error)          // 错误结束
func (s *EventStream[T, R]) Stop()                    // 消费者停止
func (s *EventStream[T, R]) Result() (R, error)       // 等待最终结果
func (s *EventStream[T, R]) ForEach(ctx context.Context, fn func(T) error) (R, error) // 迭代事件
```

**用法**：
```go
stream, _ := piai.StreamSimple(ctx, model, msgs)

// 方式一：ForEach（推荐）
result, err := stream.ForEach(ctx, func(evt piai.AssistantMessageEvent) error {
    if e, ok := evt.(piai.EventTextDelta); ok {
        fmt.Print(e.Delta)
    }
    return nil
})

// 方式二：手动迭代
for evt := range stream.Events() {
    // 处理事件
}
result, err := stream.Result()
```

---

## 流式事件类型

| 事件 | 说明 | 关键字段 |
|------|------|---------|
| `EventStart` | 流开始 | API, Provider, Model |
| `EventTextStart` | 文本块开始 | — |
| `EventTextDelta` | 文本增量 | Delta |
| `EventTextEnd` | 文本块结束 | TextSignature |
| `EventThinkingStart` | 思考块开始 | — |
| `EventThinkingDelta` | 思考增量 | Delta |
| `EventThinkingEnd` | 思考块结束 | ThinkingSignature |
| `EventToolCallStart` | 工具调用开始 | ID, Name |
| `EventToolCallDelta` | 工具参数增量 | ID, ArgumentsDelta |
| `EventToolCallEnd` | 工具调用结束 | ID, Arguments |
| `EventDone` | 流完成 | Message |
| `EventError` | 流错误 | Error |

---

## Model 注册表

```go
func LoadModels(models map[KnownProvider]map[string]Model)
func GetModel(provider KnownProvider, modelID string) (Model, error)
func GetProviders() []KnownProvider
func GetModels(provider KnownProvider) []Model
func ModelsAreEqual(a, b Model) bool
```

---

## Provider 注册表

```go
func RegisterProvider(api KnownAPI, provider APIProvider, sourceID ...string)
func GetProvider(api KnownAPI) (APIProvider, error)
func GetRegisteredProviders() []KnownAPI
func UnregisterProviders(sourceID string)
func ClearProviders()
```

---

## 工具函数

```go
func CalculateCost(model Model, usage Usage) CostBreakdown
func ResolveAPIKey(provider KnownProvider, optsKey string) string
func ResolveBaseURL(model Model, defaultURL string) string
func GetEnvAPIKey(provider KnownProvider) string
func ClampThinkingLevel(model Model, level ThinkingLevel) ThinkingLevel
func GetSupportedThinkingLevels(model Model) []ThinkingLevel
```

---

## 核心类型

### Model

```go
type Model struct {
    ID               string            // 模型 ID
    API              KnownAPI          // API 协议
    Provider         KnownProvider     // 服务商
    BaseURL          string            // 自定义 Base URL
    Reasoning        bool              // 是否支持推理
    ThinkingLevelMap map[string]string  // 思考级别映射
    Input            []Modality        // 输入模态
    Cost             Cost              // 定价（每百万 token）
    ContextWindow    int               // 上下文窗口
    MaxTokens        int               // 最大输出 token
    Headers          map[string]string // 自定义请求头
    Compat           *Compat           // 兼容性配置
}
```

### Message 类型

```go
// 用户消息
type UserMessage struct {
    Role      string    // "user"
    Content   any       // string 或 []ContentBlock
    Timestamp time.Time
}

// 助手消息
type AssistantMessage struct {
    Role       string
    Content    []ContentBlock
    API        KnownAPI
    Provider   KnownProvider
    Model      string
    Usage      Usage
    StopReason StopReason
    Timestamp  time.Time
}

// 工具结果消息
type ToolResultMessage struct {
    Role       string
    ToolCallID string
    ToolName   string
    Content    []ContentBlock
    IsError    bool
    Timestamp  time.Time
}
```

### ContentBlock 类型

```go
type TextContent struct { Type, Text, TextSignature string }
type ThinkingContent struct { Type, Thinking, ThinkingSignature string }
type ImageContent struct { Type, Data, MimeType string }
type ToolCall struct { Type, ID, Name string; Arguments json.RawMessage }
```

### StreamOptions

```go
type StreamOptions struct {
    Temperature *float64
    MaxTokens   *int
    APIKey      string
    Headers     map[string]string
    OnPayload   func(any)    // 请求体回调
    OnResponse  func(any)    // 响应回调
    // ...
}

type SimpleStreamOptions struct {
    StreamOptions
    Reasoning       ThinkingLevel   // 推理深度
    ThinkingBudgets map[string]int  // 自定义预算
}
```

### 常量

```go
// 推理深度
ThinkingMinimal / ThinkingLow / ThinkingMedium / ThinkingHigh / ThinkingXHigh

// 停止原因
StopStop / StopLength / StopToolUse / StopError / StopAborted

// 输入模态
ModalityText / ModalityImage / ModalityAudio
```

---

## Agent API

```go
// 创建 Agent
agent.New(opts AgentOptions) *Agent

// 运行
agent.Run(ctx context.Context, prompts ...Message) ([]Message, error)
agent.RunContinue(ctx context.Context) ([]Message, error)

// 控制
agent.Abort()
agent.SetTools(tools []AgentTool)
agent.SetModel(model Model)
agent.SetSystemPrompt(prompt string)
agent.Subscribe(fn func(AgentEvent))
agent.Steering(msgs ...Message)   // 注入当前轮
agent.FollowUp(msgs ...Message)   // 注入下一轮
```

### AgentTool

```go
type AgentTool struct {
    Name         string
    Description  string
    Parameters   json.RawMessage
    Execute      ToolExecuteFunc
    ExecutionMode ToolExecutionMode  // "parallel" 或 "sequential"
}
```

### Agent 事件

| 事件 | 说明 |
|------|------|
| `EventAgentStart` | Agent 开始 |
| `EventTurnStart` / `EventTurnEnd` | 轮次开始/结束 |
| `EventMessageStart` / `EventMessageUpdate` / `EventMessageEnd` | 消息流式更新 |
| `EventToolExecStart` / `EventToolExecUpdate` / `EventToolExecEnd` | 工具执行生命周期 |

---

## OAuth

```go
import "pi-ai-go/utils/oauth"

// 登录
provider, _ := oauth.Get("anthropic")
creds, err := provider.Login(ctx, oauth.LoginCallbacks{
    OnAuth: func(url string) { ... },
    OnDeviceCode: func(code, uri string) { ... },
})

// 刷新
apiKey, err := oauth.GetAPIKey(ctx, "anthropic", creds)

// 列出
providers := oauth.List()
```

---

## 工具包

### jsonparse — JSON 修复解析

```go
import "pi-ai-go/utils/jsonparse"

result, err := jsonparse.Parse[map[string]any](data)       // 修复 + 解析
result, ok := jsonparse.Streaming[map[string]any](partial)  // 流式部分解析
```

### validation — 工具调用校验

```go
import "pi-ai-go/utils/validation"

tool, result := validation.ValidateToolCall(tools, call)
```

### overflow — Context 溢出检测

```go
import "pi-ai-go/utils/overflow"

if overflow.IsOverflow(errMsg, contextWindow, usage) {
    // 上下文溢出，需要裁剪消息
}
```

### sanitize — Unicode 清理

```go
import "pi-ai-go/utils/sanitize"

clean := sanitize.Surrogates(text) // 移除无效 UTF-8
```
