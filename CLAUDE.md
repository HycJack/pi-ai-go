# pi-ai-go

多模型 AI API Go SDK，支持流式调用、工具调用、Agent 智能体。

## 架构

四层分层：core（纯类型）→ ai（公开 API）→ providers（实现）→ agent（编排）

- `core/` — 零依赖，类型 + EventStream + 注册表 + 环境变量
- `ai/` — 仅依赖 core，Stream/Complete API + Model 注册表
- `providers/` — 仅依赖 core，各 LLM Provider 实现
- `agent/` — 依赖 core+ai，多轮循环 + 工具执行
- `piai.go` — facade，re-export core+ai 所有符号

## 构建和测试

```bash
go build ./...
go test ./...
go test -race ./...
go vet ./...
```

## 关键文件

- `core/types.go` — 所有核心类型 (Message, Model, Tool 等)
- `core/events.go` — EventStream[T,R] 泛型实现
- `core/registry.go` — APIProvider 接口 + 注册表
- `ai/api.go` — Stream/Complete/GenerateImages
- `agent/agent-loop.go` — 核心 Agent 循环
- `providers/register.go` — 内置 Provider 注册
