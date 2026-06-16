# 超时错误日志区分方案

## 问题

### 问题 1: 超时日志无法区分来源
之前项目中所有超时错误都显示为 `context deadline exceeded`，无法区分：
- Agent 运行超时
- HTTP 请求超时
- 工具执行超时

这导致调试困难，无法快速定位问题来源。

### 问题 2: 重试时界面卡住
当 LLM API 调用失败触发自动重试时，界面会卡住，用户不知道系统在做什么，体验很差。

## 解决方案

### 1. 新增超时错误类型

在 `core/errors.go` 中添加：

```go
// 超时来源标识
type TimeoutSource string

const (
    TimeoutSourceAgent TimeoutSource = "agent"  // Agent 运行超时
    TimeoutSourceHTTP  TimeoutSource = "http"   // HTTP 请求超时
    TimeoutSourceTool  TimeoutSource = "tool"   // 工具执行超时
)

// 超时错误，携带来源信息
type TimeoutError struct {
    Source   TimeoutSource   // 超时来源
    Duration time.Duration   // 超时时长
    Provider KnownProvider   // 提供者（HTTP 超时时使用）
    ToolName string          // 工具名称（工具超时时使用）
    Cause    error           // 底层原因
}
```

### 2. 便捷包装函数

```go
// 通用超时包装
func WrapTimeout(source TimeoutSource, duration time.Duration, cause error) error

// HTTP 请求超时包装
func WrapHTTPTimeout(provider KnownProvider, duration time.Duration, cause error) error

// 工具执行超时包装
func WrapToolTimeout(toolName string, duration time.Duration, cause error) error
```

### 3. 更新重试逻辑

在 `core/retry.go` 中，超时错误被标记为不可重试：

```go
if errors.Is(err, ErrAborted) || errors.Is(err, context.Canceled) ||
   errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrTimeout) {
    return false
}
```

### 4. 重试进度回调（解决界面卡住问题）

`RetryConfig` 已经支持 `OnRetry` 回调，可以在重试时显示进度信息：

```go
type RetryConfig struct {
    // ... 其他字段

    // OnRetry, if set, is called before each backoff sleep with the
    // upcoming attempt number (1-based). Use it to surface "retrying in Ns"
    // messages to the user.
    OnRetry func(attempt int, nextDelay time.Duration, err error)
}
```

#### 在 Agent 中使用重试进度回调

```go
// 在 AgentLoopConfig 中设置重试配置
config := agent.AgentLoopConfig{
    // ... 其他配置
    RetryConfig: core.RetryConfig{
        Enabled:    true,
        MaxRetries: 3,
        BaseDelay:  2 * time.Second,
        MaxDelay:   5 * time.Minute,
        Multiplier: 2.0,
        OnRetry: func(attempt int, nextDelay time.Duration, err error) {
            // 通过事件流发送重试进度信息
            stream.Push(agent.EventProgress{
                Message: fmt.Sprintf("LLM 请求失败，%v 后重试 (尝试 %d/%d)",
                    nextDelay.Round(time.Millisecond), attempt, 3),
            })
        },
    },
}
```

### 5. 更新各层超时处理

#### Agent 层 (`agent/agent-loop.go`)
```go
if ctx.Err() != nil {
    if errors.Is(ctx.Err(), context.DeadlineExceeded) {
        // 包装超时错误
        stream.Error(core.WrapTimeout(core.TimeoutSourceAgent, duration, ctx.Err()))
    }
    finalize()
    return
}
```

#### Provider 层 (所有 HTTP provider)
```go
resp, err := client.Do(req)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        return core.AssistantMessage{}, core.WrapHTTPTimeout(provider, 5*time.Minute, err)
    }
    return core.AssistantMessage{}, err
}
```

#### 工具层 (`agent/tools/bash.go`)
```go
err := cmd.Run()
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        exitCode = 124 // 标准超时退出码
    }
}
// timedOut 标志在 details 中设置
```

## 效果对比

### 之前
```
Error: context deadline exceeded
```

### 现在

| 超时来源 | 日志输出 |
|---------|---------|
| Agent 运行超时 | `timeout: agent after 3m0s` |
| HTTP 请求超时 | `timeout: http after 5m0s provider=openai` |
| 工具执行超时 | `timeout: tool after 30s tool=bash` |
| 带自定义原因 | `timeout: agent after 1m0s cause=database connection failed` |

### 重试进度显示

```
⏳ LLM 请求失败，2s 后重试 (尝试 1/3)
⏳ LLM 请求失败，4s 后重试 (尝试 2/3)
⏳ LLM 请求失败，8s 后重试 (尝试 3/3)
```

## 使用示例

### 1. 判断是否为超时错误
```go
if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, core.ErrTimeout) {
    // 是超时错误
}
```

### 2. 提取超时详情
```go
var timeoutErr *core.TimeoutError
if errors.As(err, &timeoutErr) {
    fmt.Printf("超时来源: %s\n", timeoutErr.Source)
    fmt.Printf("超时时长: %v\n", timeoutErr.Duration)
    if timeoutErr.Provider != "" {
        fmt.Printf("提供者: %s\n", timeoutErr.Provider)
    }
    if timeoutErr.ToolName != "" {
        fmt.Printf("工具: %s\n", timeoutErr.ToolName)
    }
}
```

### 3. 根据来源采取不同措施
```go
var timeoutErr *core.TimeoutError
if errors.As(err, &timeoutErr) {
    switch timeoutErr.Source {
    case core.TimeoutSourceAgent:
        // Agent 运行超时，可能需要增加整体超时时间
    case core.TimeoutSourceHTTP:
        // HTTP 请求超时，可能需要检查网络或增加客户端超时
    case core.TimeoutSourceTool:
        // 工具执行超时，可能需要优化工具或增加超时时间
    }
}
```

### 4. 使用重试进度回调
```go
cfg := core.DefaultRetryConfig()
cfg.OnRetry = func(attempt int, nextDelay time.Duration, err error) {
    fmt.Printf("⏳ 请求失败，%v 后重试 (尝试 %d/%d)\n",
        nextDelay.Round(time.Millisecond), attempt, cfg.MaxRetries)
}

err := core.Retry(ctx, cfg, func() error {
    // 执行可能失败的操作
    return doSomething()
})
```

## 测试

运行测试：
```bash
go test ./core/... -v
```

运行超时错误示例：
```bash
go run examples/timeout-demo/main.go
```

运行重试进度示例：
```bash
go run examples/retry-progress-demo/main.go
```

## 兼容性

- `errors.Is(err, context.DeadlineExceeded)` 仍然返回 `true`
- `errors.Is(err, core.ErrTimeout)` 返回 `true`
- `core.IsRetryableError(err)` 返回 `false`（超时不可重试）
- 现有代码无需修改，向后兼容

## 影响范围

修改的文件：
- `core/errors.go` - 新增超时错误类型
- `core/retry.go` - 更新重试判断逻辑
- `agent/agent-loop.go` - Agent 超时包装
- `agent/tools/bash.go` - 工具超时处理
- `providers/compat/compat.go` - HTTP 超时包装
- `providers/openai/responses.go` - HTTP 超时包装
- `providers/mistral/mistral.go` - HTTP 超时包装
- `providers/google/google.go` - HTTP 超时包装
- `providers/google/vertex.go` - HTTP 超时包装
- `providers/bedrock/bedrock.go` - HTTP 超时包装
- `providers/anthropic/anthropic.go` - HTTP 超时包装
- `core/timeout_test.go` - 新增测试
- `examples/timeout-demo/main.go` - 超时错误示例
- `examples/retry-progress-demo/main.go` - 重试进度示例