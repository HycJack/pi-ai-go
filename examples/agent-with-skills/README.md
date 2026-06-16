# agent-with-skills

具备 **Skills + Context Management + Memory Persistence** 的 Agent 示例。

## 功能特性

| 功能 | 说明 |
|------|------|
| **Skills 加载** | 从目录递归加载 Trae-style `SKILL.md`，渲染到 system prompt |
| **对话持久化** | 支持内存模式与 JSONL 文件持久化（断电可恢复） |
| **上下文窗口管理** | Token 估算（区分中英文）+ 软/硬阈值 + 自动压缩 |
| **长期记忆** | JSON KV 存储，跨会话保留用户偏好、关键事实 |
| **自动记忆触发** | 用户输入含 `[remember:key=val]` 时自动保存 |
| **多轮对话** | 历史消息自动加载与保存 |
| **增量 token 统计** | O(1) 添加 vs O(N) 重算 |
| **System prompt 缓存** | skills/memory 不变时复用 |
| **异步 session 写入** | 后台批量 fsync，不阻塞主流程 |
| **交互命令** | `:history :context :compact :memory :save :load :remember :forget :stats :quit` |

## 包结构

```
examples/agent-with-skills/
├── main.go              # 入口（集成所有组件）
├── memory/              # 长期记忆 KV 存储
│   ├── memory.go
│   └── memory_test.go
├── contextmgr/          # 上下文窗口管理
│   ├── contextmgr.go
│   └── contextmgr_test.go
├── autolearn/           # 自动记忆触发
│   ├── autolearn.go
│   └── autolearn_test.go
├── skills/              # Trae-style skills（外部）
├── session.jsonl        # 对话历史（运行时生成）
└── memory.json          # 长期记忆（运行时生成）
```

## 快速开始

```powershell
cd examples\agent-with-skills

# 复制配置
copy .env.example .env
# 编辑 .env 填入 MODEL, PROVIDER, API_KEY

# 单次查询（启用所有持久化）
go run . -skills ./skills -session ./session.jsonl -memory ./memory.json -query "你好，请记住我的名字叫小明"

# 交互模式
go run . -skills ./skills -session ./session.jsonl -memory ./memory.json

# 内存模式（不持久化）
go run . -skills ./skills -query "你好"
```

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-skills` | `SKILLS` 环境变量 | skills 目录 |
| `-model` | `MODEL` 环境变量 | 模型 ID |
| `-provider` | `PROVIDER` 环境变量 | Provider 名称 |
| `-base-url` | `BASE_URL` 环境变量 | API base URL |
| `-api-key` | `API_KEY` 环境变量 | API key |
| `-query` | （无） | 单次查询，交互模式则留空 |
| `-session` | `SESSION` 环境变量 | JSONL session 文件（空=内存模式） |
| `-memory` | `MEMORY` 环境变量 | 长期记忆文件（空=不启用） |
| `-auto-compact` | `true` | 超出软限制时自动压缩 |
| `-v` | `false` | 详细日志 |

## 交互命令

| 命令 | 功能 |
|------|------|
| `:history` | 显示完整对话历史 |
| `:context` | 显示当前上下文 token 使用情况 |
| `:compact` | 手动触发 LLM 摘要压缩 |
| `:memory` | 列出所有长期记忆 |
| `:remember <key> = <value>` | 添加/更新记忆 |
| `:forget <key>` | 删除记忆 |
| `:save` | 手动保存（JSONL 自动 sync，memory 需要手动） |
| `:load` | 重新加载记忆 |
| `:stats` | 显示完整统计信息 |
| `:quit` / `:exit` | 退出 |

## 记忆刷新机制

记忆有 **三种刷新路径**：

### 1. 磁盘 → 内存
- **程序启动时**自动加载
- **`:load` 命令**重新从磁盘读取

### 2. 内存 → 磁盘
- **每次 `Set/Delete`** 后立即 `Save()`
- **自动学习触发**后立即 `Save()`

### 3. 内存 → LLM（System Prompt 缓存失效）
基于 `Hash()` 检测变化（Hash 基于 `UpdatedAt` 时间戳拼接）：

| 触发动作 | 缓存失效方式 |
|---------|-------------|
| `:remember key=value` | `promptCache.Invalidate()` |
| `:forget key` | `promptCache.Invalidate()` |
| `:load` 重新加载 | `promptCache.Invalidate()` |
| 自动学习触发 | `promptCache.Invalidate()` |
| 每轮对话开始 | `promptCache.Get()` 检查 hash |

**关键代码**：
- [memory.go Hash()](file:///c:/Users/huangyicao/Downloads/hyperframes-test/pi-ai-go/examples/agent-with-skills/memory/memory.go) - 基于 `UpdatedAt` 的快速 hash
- [main.go SystemPromptCache](file:///c:/Users/huangyicao/Downloads/hyperframes-test/pi-ai-go/examples/agent-with-skills/main.go) - prompt 缓存与失效逻辑

## 自动记忆触发

支持两种自动触发方式：

### 1. 用户输入标记（同步）

用户输入包含以下任一标记时自动提取保存：

```
请记住：user.name=小明
记住：preference.language=zh-CN
[remember:project.name=pi-ai-go]
[memorize:user.email=test@example.com]
```

### 2. 工具结果标记

工具输出中包含：

```
REMEMBER:user.name=小明
SAVE_MEMORY:preference.theme=dark
```

### 3. LLM 异步提取（默认关闭）

每 N 轮对话后调用 LLM 提取可记忆的事实（可通过 `autolearn.Settings.AutoLearn = true` 启用）。

## 上下文窗口管理

### 工作流程

```
1. 加载历史消息 → tokenStats.Recompute()
2. 添加用户消息 → tokenStats.Add() (O(1))
3. 检查 tokenStats.ShouldCompact()
   - 超过软限制 → 调用 LLM 摘要早期消息
   - 超过硬限制 → 强制截断最早消息
4. 调用 LLM
5. 接收响应 → tokenStats.AddMany() 或 Recompute()
```

### 模型上下文窗口

| 模型 | MaxContext |
|------|-----------|
| Claude (Sonnet 4.5) | 200,000 |
| GPT-4o | 128,000 |
| DeepSeek | 64,000 |
| Kimi / GLM-4-Long | 128,000 |
| 其它 | 128,000 |

### 软/硬限制

- **软限制（70%）**：触发自动 LLM 摘要压缩
- **硬限制（95%）**：强制截断最早消息

## 性能优化

| 优化点 | 实现 | 效果 |
|--------|------|------|
| 增量 token 统计 | `TokenStats.Add/AddMany` | O(1) vs O(N) |
| System prompt 缓存 | `SystemPromptCache` 基于 hash 复用 | 避免每轮格式化 |
| 异步 session 写入 | `SessionFlusher` 后台批量 | 不阻塞主流程 |
| 并行加载 | `loadResult` goroutine | skills/memory/session 并行 |
| LLM 摘要并发 | `go func() { ... }` | 串行化与 LLM 调用并发 |
| 流式输出 | `stream.ForEach` | 边收边打印 |

## 测试

```powershell
# 全部测试
go test -count=1 ./...

# 单个包
go test -count=1 -v ./memory/...
go test -count=1 -v ./contextmgr/...
go test -count=1 -v ./autolearn/...
```

测试结果：
- `memory` 包：10 个测试
- `contextmgr` 包：多个测试
- `autolearn` 包：8 个测试

## 与 pi-ai-go 的关系

| 包 | 来源 | 作用 |
|---|------|------|
| `pi-ai-go/core` | 外部 | 消息类型、Stream、Model |
| `pi-ai-go/llm` | 外部 | CompleteSimple |
| `pi-ai-go/agent` | 外部 | AgentLoopDetailed、AgentEvent |
| `pi-ai-go/agent/session` | 外部 | Session, JSONLStorage, LoadSkills |
| `pi-ai-go/agent/tools` | 外部 | All() 工具集 |
| `pi-ai-go/providers` | 外部 | provider 注册（`_ "pi-ai-go/providers"`） |
| `examples/agent-with-skills/memory` | 本项目 | 长期记忆 |
| `examples/agent-with-skills/contextmgr` | 本项目 | 上下文管理 |
| `examples/agent-with-skills/autolearn` | 本项目 | 自动记忆触发 |
