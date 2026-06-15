# Agent 上下文管理与记忆存储 Demo

演示如何使用 pi-ai-go 的 `agent` 和 `session` 包管理 Agent 的上下文、记忆和对话持久化。

## 功能

| 功能 | 说明 |
|------|------|
| **多轮对话** | Agent 自动保持上下文连贯，跨轮次记住用户信息 |
| **会话持久化** | 支持内存存储和 JSONL 文件持久化，断电可恢复 |
| **上下文压缩** | 配置 soft/hard token 限制，自动触发滑动窗口压缩 |
| **分支摘要** | LLM 生成对话分支摘要，用于回溯和分支切换 |
| **事件流监控** | 实时获取 token 使用、费用、工具调用、压缩事件 |
| **会话历史** | 查看完整会话树（消息、压缩记录、分支摘要、元信息） |

## 快速开始

```bash
cd examples/agent-memory-demo

# 配置 API Key
cp .env.example .env
# 编辑 .env 填入你的 API Key

# 内存模式（重启丢失）
go run .

# 持久化模式（JSONL 文件，断电可恢复）
go run . -persist session.jsonl
```

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-provider` | `deepseek` | Provider 名称 |
| `-model` | `deepseek-chat` | 模型 ID |
| `-api-key` | 从环境变量读取 | API Key |
| `-persist` | 空（内存模式） | JSONL 持久化文件路径 |

## 交互命令

在对话中输入以下命令：

| 命令 | 功能 |
|------|------|
| `:history` | 显示完整会话历史 |
| `:context` | 显示当前上下文统计（消息数、类型、token 估算） |
| `:compact` | 手动触发 LLM 摘要压缩 |
| `:branch` | 生成当前对话分支摘要 |
| `:save` | 显示会话状态 |
| `:quit` | 退出 |

## 使用示例

```
🧑 你好，请记住我的名字叫小明
🤖 你好，小明！很高兴认识你！

🧑 我的名字是什么？
🤖 你的名字是小明。

🧑 :history
📜 会话历史 (5 条记录):
  1. ℹ️  会话: demo-20260615-153043
  2. 🧑 你好，请记住我的名字叫小明
  3. 🤖 你好，小明！很高兴认识你！
  4. 🧑 我的名字是什么？
  5. 🤖 你的名字是小明。

🧑 :context
📊 上下文信息:
  消息数量: 4
  用户消息: 2, 助手消息: 2, 工具结果: 0
  估算 token: ~88

🧑 :quit
再见!
```

## 架构说明

### 会话存储

```
SessionStorage (接口)
├── MemoryStorage    — 内存存储，进程退出即丢失
└── JSONLStorage     — JSONL 文件持久化，每条记录一行 JSON
```

### 会话条目类型

| 类型 | 说明 |
|------|------|
| `EntryMessage` | 用户/助手/工具消息 |
| `EntryCustomMessage` | 应用自定义消息 |
| `EntryCompaction` | 上下文压缩记录（摘要 + token 数） |
| `EntryBranchSummary` | 对话分支摘要 |
| `EntryModelChange` | 模型切换记录 |
| `EntryThinkingChange` | 思考级别变更 |
| `EntrySessionInfo` | 会话元信息（ID、描述） |

### 上下文压缩流程

```
用户消息 → Agent Loop → LLM 调用
                          ↓
                   检查 token 使用
                          ↓
              超过 soft limit (80%)?
                    ↓           ↓
                   否           是
                   ↓           ↓
              继续对话    执行压缩策略
                          ↓           ↓
                   滑动窗口      LLM 摘要
                   (丢弃旧消息)  (生成摘要 + 保留尾部)
                          ↓
                   发送 EventCompaction
                          ↓
                   继续对话 (压缩后上下文)
```

### JSONL 文件格式

每行一个 JSON 对象，按时间顺序追加：

```jsonl
{"id":"...","type":"session_info","sessionId":"demo-xxx","timestamp":"..."}
{"id":"...","type":"message","messageRole":"user","message":{"role":"user","content":"你好"},"timestamp":"..."}
{"id":"...","type":"message","messageRole":"assistant","message":{"role":"assistant","content":[...]},"timestamp":"..."}
{"id":"...","type":"compaction","compactionSummary":"压缩了 10 条消息","tokensBefore":5000,"timestamp":"..."}
```

## 关键 API

### 创建会话

```go
storage, _ := session.NewJSONLStorage("session.jsonl")
sess, _ := session.NewSession(storage)
defer sess.Close()
```

### 追加消息

```go
sess.Append(session.SessionTreeEntry{
    ID:        session.GenerateID(),
    Type:      session.EntryMessage,
    Timestamp: time.Now(),
    Message:   core.UserMessage{Role: "user", Content: "你好"},
})
```

### 重建上下文

```go
ctx := sess.BuildContext()
messages := ctx.Messages  // 可直接传给 AgentLoop
```

### 配置自动压缩

```go
config := agent.AgentLoopConfig{
    Model: model,
    ContextPolicy: &agent.ContextPolicy{
        SoftLimit:       0.80,   // 80% 时触发
        HardLimit:       0.95,   // 95% 时强制
        ReservedOutput:  2048,   // 为输出预留 token
        MinTailMessages: 6,      // 至少保留最近 6 条
        Strategy:        agent.CompactionStrategySlidingWindow,
    },
    OnCompaction: func(e agent.CompactionEvent) {
        // 记录压缩事件到会话
        sess.Append(session.SessionTreeEntry{
            Type:              session.EntryCompaction,
            CompactionSummary: fmt.Sprintf("压缩了 %d 条消息", e.Dropped),
            TokensBefore:      e.TokensBefore,
        })
    },
}
```

### 手动压缩

```go
settings := session.DefaultCompactionSettings()
settings.MaxTokensBeforeCompaction = 100000
result, _ := session.CompactSession(ctx, sess, model, settings)
fmt.Printf("节省 %d token, 保留 %d 条消息\n", result.TokensSaved, result.EntriesKept)
```

---
  Demo 功能说明

  examples/agent-memory-demo/
  ├── main.go      # 完整 demo
  ├── go.mod
  └── .env

  运行方式

  # 内存存储 (重启丢失)
  go run .

  # JSONL 持久化 (重启可恢复)
  go run . -persist session.jsonl

  内置命令

  ┌──────────┬─────────────────────────────────────────────┐
  │   命令   │                    功能                     │
  ├──────────┼─────────────────────────────────────────────┤
  │ :history │ 显示完整会话历史 (消息/压缩/分支/元信息)    │
  ├──────────┼─────────────────────────────────────────────┤
  │ :context │ 显示当前上下文统计 (消息数/类型/token 估算) │
  ├──────────┼─────────────────────────────────────────────┤
  │ :compact │ 手动触发 LLM 摘要压缩                       │
  ├──────────┼─────────────────────────────────────────────┤
  │ :branch  │ 生成当前对话分支摘要                        │
  ├──────────┼─────────────────────────────────────────────┤
  │ :save    │ 显示会话状态信息                            │
  ├──────────┼─────────────────────────────────────────────┤
  │ :quit    │ 退出                                        │
  └──────────┴─────────────────────────────────────────────┘

  展示的核心能力

  1. 会话管理 — Session + MemoryStorage / JSONLStorage，支持断电恢复

  # JSONL 持久化 (重启可恢复)
  go run . -persist session.jsonl

  内置命令

  ┌──────────┬─────────────────────────────────────────────┐
  │   命令   │                    功能                     │
  ├──────────┼─────────────────────────────────────────────┤
  │ :history │ 显示完整会话历史 (消息/压缩/分支/元信息)    │
  ├──────────┼─────────────────────────────────────────────┤
  │ :context │ 显示当前上下文统计 (消息数/类型/token 估算) │
  ├──────────┼─────────────────────────────────────────────┤
  │ :compact │ 手动触发 LLM 摘要压缩                       │
  ├──────────┼─────────────────────────────────────────────┤
  │ :branch  │ 生成当前对话分支摘要                        │
  ├──────────┼─────────────────────────────────────────────┤
  │ :save    │ 显示会话状态信息                            │
  ├──────────┼─────────────────────────────────────────────┤
  │ :quit    │ 退出                                        │
  └──────────┴─────────────────────────────────────────────┘

  展示的核心能力

  1. 会话管理 — Session + MemoryStorage / JSONLStorage，支持断电恢复
  └──────────┴─────────────────────────────────────────────┘

  展示的核心能力

  1. 会话管理 — Session + MemoryStorage / JSONLStorage，支持断电恢复
  2. 多轮记忆 — Agent 自动保持上下文连贯，跨轮次记住用户信息
  3. 上下文压缩 — ContextPolicy 配置 soft/hard limit，自动滑动窗口压缩
  4. 分支摘要 — SummarizeBranch 生成对话分支摘要，用于回溯场景
  5. JSONL 持久化 — 每条消息实时追加到文件，支持断电恢复和审计
  6. 事件流监控 — 通过 AgentLoopDetailed 获取 token 使用、费用、压缩事件