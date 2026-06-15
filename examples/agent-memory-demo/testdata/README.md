# 测试数据 (Test Data)

为 agent-memory-demo 提供测试数据，覆盖各种上下文和记忆管理场景。

## 文件列表

| 文件 | 格式 | 用途 |
|------|------|------|
| `conversation-sample.jsonl` | JSONL | 完整对话样本，含用户/助手/工具消息 |
| `session-with-branches.jsonl` | JSONL | 包含分支摘要和压缩记录 |
| `user-profile.json` | JSON | 用户偏好和上下文（用于个性化） |
| `long-context.jsonl` | JSONL | 接近 context window 限制的长对话 |
| `tasks.md` | Markdown | 任务列表（用于演示 Agent 任务管理） |
| `notes.md` | Markdown | 笔记（用于演示记忆召回） |

## 使用方法

```go
// 加载测试会话
storage, _ := session.NewJSONLStorage("testdata/conversation-sample.jsonl")
sess, _ := session.NewSession(storage)

// 加载用户偏好
data, _ := os.ReadFile("testdata/user-profile.json")
var profile UserProfile
json.Unmarshal(data, &profile)
```

## 数据生成

所有测试数据使用真实场景，内容经过脱敏处理。
