# AGENTS.md

AI coding agent 指南。

## 构建和测试

```bash
go build ./...
go test ./...
go test -race ./...
go vet ./...
```

Go 1.23+。唯一外部依赖 `jsonschema/v6`。

## 架构

四层分层，依赖方向单一：

```
core/ ← 零依赖 (类型 + EventStream + 注册表 + 工具契约)
  ↑
llm/        ← 仅依赖 core (公开 API + Model 注册表)
  ↑
providers/  ← 仅依赖 core (LLM 实现)
  ↑
agent/      ← 依赖 core + llm (多轮循环 + 工具执行)
  ↑
piai.go     ← facade re-export
```

## Internal 包清单

`internal/` 下的工具包供各层使用。以下是每个包的用途和使用场景：

### `internal/sse` — SSE 流式解析

用于解析 Server-Sent Events 流。provider 实现必须使用此包处理 LLM 的 SSE 响应。

**使用方法**：`sse.Scan(ctx, resp.Body, sse.ScanConfig{}, onData)`

**已在以下位置使用**：
- `providers/compat/compat.go` — 所有 OpenAI-compatible provider 的 SSE 解析
- `providers/anthropic/anthropic.go`
- `providers/openai/responses.go`
- `providers/google/google.go`
- `providers/google/vertex.go`
- `providers/bedrock/bedrock.go`
- `providers/mistral/mistral.go`

**注意事项**：
- 使用 `sse.ScanConfig{InitialBufSize, MaxBufSize}` 控制缓冲区
- 函数在 context 取消时会自动关闭 reader
- `data: [DONE]` 标记正常结束

### `internal/jsonparse` — JSON 修复与流式解析

用于解析 LLM 返回的不规范 JSON。LLM 返回的 JSON 数据可能格式不完整或有转义问题，此包可以自动修复。

**关键函数**：
```go
// 智能解析 + 自动修复
result, err := jsonparse.Parse[T](rawJSON)

// 流式场景下的不完整 JSON 解析
result, ok := jsonparse.Streaming[T](partialJSON)
```

**使用场景**：
- 解析 LLM 工具调用参数（可能是不完整的 JSON）
- 解析 SSE 流式数据中的部分 JSON
- 修复 LLM 输出中的转义错误

**已在以下位置使用**：（暂无 — 待接入）

### `internal/diagnostics` — 诊断事件记录

用于在消息处理过程中记录诊断事件，追踪处理状态。

```go
diag := diagnostics.New("skill_loading", map[string]any{"file": path})
diagErr := diagnostics.NewWithError("tool_error", err)
errMsg := diagnostics.ExtractError(diagErr)
formatted := diagnostics.FormatThrownValue(recoveredValue)
```

**使用场景**：
- 技能加载过程中的诊断记录
- 工具执行过程中的错误追踪
- recover 时安全格式化 panic 值

**已在以下位置使用**：（暂无 — 待接入）

### `internal/hash` — 短字符串哈希

为任意字符串生成确定性 base36 短哈希（1-13 位字符）。

```go
id := hash.Short("user:你好 world") // 例如 "1a2b3c4d5e6f"
```

**使用场景**：
- 生成消息/会话的唯一 ID
- 工具调用的 ID 生成
- 缓存键的生成
- Agent 记忆/技能的内容寻址

**已在以下位置使用**：（暂无 — 待接入）

### `internal/sanitize` — Unicode 清理

移除文本中无效的 Unicode 代理字符，防止 JSON 序列化错误。

```go
clean := sanitize.Surrogates(rawText)
```

**使用场景**：
- 清理 LLM 返回的文本数据（特别是流式输出）
- 将用户消息写入日志或持久化前清理
- 处理来自外部数据源的不可信文本

**已在以下位置使用**：（暂无 — 待接入）

### `internal/overflow` — 上下文溢出检测

检测 LLM 返回的错误是否属于上下文窗口溢出，支持 20+ 种提供商的错误模式。

```go
if overflow.IsOverflow(errMsg, model.ContextWindow, currentUsage) {
    // 触发上下文压缩
}
```

**使用场景**：
- Agent 中检测上下文溢出后自动压缩
- 区分溢出错误与限流错误（限流应重试，溢出应压缩）

**已在以下位置使用**：（暂无 — 待接入）

### `internal/validation` — 工具参数验证

使用 JSON Schema 验证 LLM 返回的工具调用参数，支持自动类型转换。

```go
tool, result := validation.ValidateToolCall(tools, call)
if !result.Valid {
    // 向 LLM 报告验证错误
}
```

**使用场景**：
- Agent 执行工具前验证参数
- LLM 返回错误类型时自动转换（如字符串转数字）

**已在以下位置使用**：（暂无 — 待接入）

### `internal/oauth` — OAuth 认证

管理 OAuth 提供者的注册、登录和令牌刷新。内置支持 Anthropic、GitHub Copilot、OpenAI Codex。

```go
creds, err := oauth.GetAPIKey(ctx, "anthropic", credentials)
```

**使用场景**：
- 需要 OAuth 认证的 provider（GitHub Copilot 等）
- 令牌过期自动刷新

## 如何添加新 Provider

1. 创建 `providers/<name>/<name>.go`
2. 实现 `core.APIProvider` 接口（`Stream` + `StreamSimple`，均接收 `context.Context`）
3. 在 `providers/register.go` 的 `RegisterBuiltInProviders()` 中注册：`core.RegisterProvider(...)`
4. 如需新 provider 常量，添加到 `core/types.go` 的 `KnownProvider`
5. 添加环境变量映射到 `core/env.go` 的 `providerEnvVars`
6. 添加 `Model` 数据到模型注册表
7. **SSE 解析使用 `internal/sse`，JSON 修复使用 `internal/jsonparse`**

### Provider 实现清单

- 从 `core.Context` + `core.StreamOptions` 构建 JSON body
- 启动 goroutine 执行 HTTP POST + SSE 解析（使用 `internal/sse.Scan`）
- 事件顺序：`EventStart` → `EventTextDelta*` → `EventTextEnd` → `EventDone`
- 成功调用 `stream.End(msg)`，失败调用 `stream.Error(err)`
- 处理 `opts.OnPayload` 和 `opts.OnResponse` 回调
- 设置 `msg.Usage` 和 `msg.StopReason`
- 调用 `core.CalculateCost(model, msg.Usage)` 计算费用
- 使用 `http.NewRequestWithContext(ctx, ...)` 支持 context 取消
- **SSE 解析后必须执行 finalization**（刷新 text/thinking buffer + EventDone），放在 scanner 循环外
- 流式 Tool Call 的参数 JSON 使用 `internal/jsonparse.Streaming` 解析

### SSE 处理模式

```go
import "pi-ai-go/internal/sse"

err := sse.Scan(ctx, resp.Body, sse.ScanConfig{}, func(data string) error {
    var chunk map[string]any
    if err := json.Unmarshal([]byte(data), &chunk); err != nil {
        return nil // skip malformed events
    }
    // Parse delta, push to stream...
    return nil
})
if err != nil {
    return core.AssistantMessage{}, err
}

// Finalization — 必须在 Scan 结束后执行
if textBuf.Len() > 0 { /* flush text */ }
if thinkingBuf.Len() > 0 { /* flush thinking */ }
msg.Usage.Cost = core.CalculateCost(model, msg.Usage)
stream.Push(core.EventDone{Message: msg})
```

## EventStream 契约

- `Push()` 返回 `bool` — buffer 满时返回 false，生产者应停止
- `End()`/`Error()` 在锁内完成所有 channel 操作，避免与 Push 竞态
- `Stop()` 关闭 stop channel 通知生产者
- Channel buffer 为 64
- `ForEach()` 在 context 取消或回调错误时自动调用 `Stop()`

## 测试

- 每个 `.go` 文件对应 `_test.go`
- 注册表测试使用 `ClearProviders()` + `defer ClearProviders()`
- EventStream 测试用 goroutine 模拟并发
- 集成测试在 `examples/` 中（非 `_test.go`）

## 注意事项

- `json.Unmarshal` 解析工具参数必须检查错误
- LLM 返回的不规范 JSON 先用 `internal/jsonparse.Parse` 或 `internal/jsonparse.Repair` 处理
- LLM 返回的文本用 `internal/sanitize.Surrogates` 清理后再序列化
- Agent 中检测上下文溢出使用 `internal/overflow.IsOverflow`
- 工具调用参数验证使用 `internal/validation.ValidateToolCall`
- 生成唯一 ID 使用 `internal/hash.Short`
- 诊断事件使用 `internal/diagnostics.New`
- Anthropic 多轮对话需要 thinking/text block 的 `signature` 字段
- Mistral 工具调用 ID 必须是 9 字符字母数字
- Vertex AI URL 格式不同于 Google AI
- Provider finalization 必须在 SSE 循环外（防止连接中断丢失内容）
