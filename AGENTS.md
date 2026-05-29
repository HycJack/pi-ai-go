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
core/ ← 零依赖 (类型 + EventStream + 注册表)
  ↑
ai/   ← 仅依赖 core (公开 API + Model 注册表)
  ↑
providers/ ← 仅依赖 core (LLM 实现)
  ↑
agent/ ← 依赖 core + ai (多轮循环 + 工具执行)
  ↑
piai.go ← facade re-export
```

## 如何添加新 Provider

1. 创建 `providers/<name>/<name>.go`
2. 实现 `core.APIProvider` 接口（`Stream` + `StreamSimple`，均接收 `context.Context`）
3. 在 `providers/register.go` 的 `RegisterBuiltInProviders()` 中注册：`core.RegisterProvider(...)`
4. 如需新 provider 常量，添加到 `core/types.go` 的 `KnownProvider`
5. 添加环境变量映射到 `core/env.go` 的 `providerEnvVars`
6. 添加 `Model` 数据到模型注册表

### Provider 实现清单

- 从 `core.Context` + `core.StreamOptions` 构建 JSON body
- 启动 goroutine 执行 HTTP POST + SSE 解析
- 事件顺序：`EventStart` → `EventTextDelta*` → `EventTextEnd` → `EventDone`
- 成功调用 `stream.End(msg)`，失败调用 `stream.Error(err)`
- 处理 `opts.OnPayload` 和 `opts.OnResponse` 回调
- 设置 `msg.Usage` 和 `msg.StopReason`
- 调用 `core.CalculateCost(model, msg.Usage)` 计算费用
- 使用 `http.NewRequestWithContext(ctx, ...)` 支持 context 取消
- **SSE 解析后必须执行 finalization**（刷新 text/thinking buffer + EventDone），放在 scanner 循环外

### SSE 处理模式

```go
scanner := bufio.NewScanner(body)
scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

for scanner.Scan() {
    line := scanner.Text()
    if !strings.HasPrefix(line, "data: ") { continue }
    data := strings.TrimPrefix(line, "data: ")
    // Parse JSON, dispatch on event type, push to stream
}

// Finalization — 必须在循环外，确保连接中断时也能执行
if textBuf.Len() > 0 { /* flush text */ }
if thinkingBuf.Len() > 0 { /* flush thinking */ }
msg.Usage.Cost = core.CalculateCost(model, msg.Usage)
stream.Push(core.EventDone{Message: msg})
return msg, nil
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
- Anthropic 多轮对话需要 thinking/text block 的 `signature` 字段
- Mistral 工具调用 ID 必须是 9 字符字母数字
- Vertex AI URL 格式不同于 Google AI
- Provider finalization 必须在 SSE 循环外（防止连接中断丢失内容）
