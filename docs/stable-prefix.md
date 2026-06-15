# StablePrefix 研究文档

本文档深入研究 LLM 前缀缓存（Prefix Cache）机制、StablePrefix 设计模式、以及相关开源实现方案。

## 一、问题背景

### 1.1 LLM 调用成本结构

每次 LLM API 调用，输入 token（prompt）按输入价格计费，输出 token 按输出价格计费。以 Claude Sonnet 4 为例（2026 年价格）：

| 类型 | 价格（每 1M tokens） |
|------|---------------------|
| Input | $3.00 |
| Output | $15.00 |
| **Cache Read** | **$0.30**（10%） |
| **Cache Write** | **$3.75**（125%，一次性） |

多轮对话中，system prompt + 早期消息往往在每一轮都重复发送，占总输入 60-80%。如果能命中 prefix cache，可降低 **90% 的输入成本**。

### 1.2 Prefix Cache 的工作原理

LLM 在底层做 attention 计算时，前面已经处理过的 token 的 KV cache 可以被复用。如果后续请求的 prefix 与之前某个请求的 prefix 完全相同（或足够相似），就可以复用 KV cache。

**关键约束**：prefix cache 是**字节级稳定**的（byte-stable），任何细微的字节变化（空格、换行、字段顺序）都会导致 cache miss。

## 二、各大 Provider 的 Prefix Cache 机制

### 2.1 Anthropic（显式 cache_control）

Anthropic 提供显式的 `cache_control` 字段，需要在请求中标记哪些内容应该被缓存：

```json
{
  "system": [
    {
      "type": "text",
      "text": "You are a helpful assistant...",
      "cache_control": {"type": "ephemeral", "ttl": "5m"}
    }
  ],
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "Hello",
          "cache_control": {"type": "ephemeral"}
        }
      ]
    }
  ]
}
```

- **TTL 选项**：`5m`（5 分钟）或 `1h`（1 小时）
- **最多 4 个 breakpoint**（Anthropic 限制）
- **成本**：cache write 多收 25%，cache read 节省 90%

### 2.2 OpenAI（自动 prefix caching）

OpenAI 从 GPT-4o 开始自动启用 prefix caching，**无需显式标记**：

- 缓存命中时返回 `prompt_tokens_details.cached_tokens` 字段
- 自动匹配最长公共前缀
- 缓存有效期约 5-10 分钟

```json
{
  "usage": {
    "prompt_tokens": 1500,
    "completion_tokens": 200,
    "prompt_tokens_details": {
      "cached_tokens": 1200
    }
  }
}
```

### 2.3 DeepSeek（自动 prefix caching）

DeepSeek API 自动启用 prefix caching：

- 缓存命中价格约为输入价的 10%
- 缓存有效期较长（数小时）
- 推理模型（deepseek-reasoner、V4）有特殊行为

### 2.4 Google Gemini（隐式 caching）

Gemini 也支持隐式 prefix caching，但命中规则略有不同。

## 三、StablePrefix 设计模式

### 3.1 核心思想

**StablePrefix** 是一种客户端优化模式，通过以下两个机制最大化 prefix cache 命中率：

1. **StablePrefix**（稳定前缀）：system prompt + tool specs 序列化一次后冻结，后续轮次复用相同的字节序列
2. **AppendOnlyLog**（仅追加日志）：消息只增长，先前的轮次不重新序列化

两者结合，每轮只有用户的新消息部分是 cache miss，其他全部命中。

### 3.2 Crux 实现

Crux 的 `crux-agent-runtime/agent/append_only_context.go`（335 行）实现了这个模式：

```go
// StablePrefix 通过 sha256 fingerprint 检测变化
func (s *StablePrefix) Build(systemPrompt string, tools []ai.ToolSchema) bool {
    fp := computeFingerprint(systemPrompt, tools)
    if s.snapshot != nil && s.snapshot.Fingerprint == fp {
        return false  // 命中：未变化
    }
    s.snapshot = &StablePrefixSnapshot{
        SystemPrompt: systemPrompt,
        Tools:        append([]ai.ToolSchema{}, tools...),
        Fingerprint:  fp,
    }
    s.version++
    return true  // 未命中：已变化
}

// 计算指纹：system + tools（按名称排序）
func computeFingerprint(systemPrompt string, tools []ai.ToolSchema) string {
    h := sha256.New()
    h.Write([]byte(systemPrompt))
    h.Write([]byte{0})  // separator

    // 关键：按名称排序，避免工具顺序变化导致 cache miss
    sorted := make([]ai.ToolSchema, len(tools))
    copy(sorted, tools)
    sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

    for _, t := range sorted {
        h.Write([]byte(t.Name))
        h.Write([]byte{0})
        h.Write([]byte(t.Description))
        h.Write([]byte{0})
        if t.Parameters != nil {
            b, _ := json.Marshal(t.Parameters)  // 重新 marshal 保证 map 顺序稳定
            h.Write(b)
        }
        h.Write([]byte{0xff})  // tool separator
    }
    return hex.EncodeToString(h.Sum(nil))
}
```

**关键设计决策：**

1. **工具按名称排序**：避免 `tools` slice 顺序变化导致 cache miss
2. **JSON 重新 marshal**：`map[string]any` 的迭代顺序是随机的，必须重新 marshal 才能稳定
3. **sha256 而非简单 hash**：使用密码学哈希避免冲突
4. **AppendOnlyLog 容量限制**：默认 500 条消息，超出后保留最后 200 条 + 一个固定的 marker

```go
const (
    aocLogCap  = 500  // 超过这个就截断
    aocLogTail = 200  // 保留最后 200 条
)

func (a *AppendOnlyLog) enforceCapLocked() {
    if len(a.messages) <= aocLogCap {
        return
    }
    // 固定的 marker 不会导致 cache miss
    marker := ai.Message{
        Role:    "system",
        Content: "[truncated for prefix cache: older messages dropped, fingerprint stable]",
    }
    tailStart := len(a.messages) - aocLogTail
    tail := append([]ai.Message{}, a.messages[tailStart:]...)
    a.messages = append([]ai.Message{marker}, tail...)
}
```

### 3.3 Bench 验证

Crux 的 `bench_test.go` 验证了 StablePrefix 的效果：

```
Scenario:
  turn 0-9:   3 tools, sysA
  turn 10-19: 4 tools, sysA  (1 tool added at turn 10)
  turn 20-29: 4 tools, sysB  (sys prompt changed at turn 20)

Result:
  Without AppendOnlyContext: 30 misses
  With AppendOnlyContext:    27/30 hits (90% 命中率)
```

## 四、相关开源实现

### 4.1 oh-my-pi/agent（来源）

Crux 的实现是受 `oh-my-pi/agent/src/append-only-context.ts` 启发（BSD-3 许可）。这是最早提出 StablePrefix 概念的开源项目之一。

### 4.2 hermes-agent（Python 实现）

`/mnt/workspace/hermes-agent/agent/prompt_caching.py` 提供 Anthropic 专用实现：

```python
def apply_anthropic_cache_control(
    api_messages: List[Dict[str, Any]],
    cache_ttl: str = "5m",
) -> List[Dict[str, Any]]:
    """应用 system_and_3 缓存策略：在 system prompt + 最后 3 条消息上
    放置 4 个 cache_control breakpoint"""
    messages = copy.deepcopy(api_messages)
    if not messages:
        return messages

    marker = {"type": "ephemeral"}
    if cache_ttl == "1h":
        marker["ttl"] = "1h"

    # 1. system prompt
    if messages[0].get("role") == "system":
        _apply_cache_marker(messages[0], marker)
        breakpoints_used = 1

    # 2-4. 最后 3 条非 system 消息
    remaining = 4 - breakpoints_used
    non_sys = [i for i in range(len(messages)) if messages[i].get("role") != "system"]
    for idx in non_sys[-remaining:]:
        _apply_cache_marker(messages[idx], marker)

    return messages
```

**策略名称**：`system_and_3`（system + 最后 3 条消息，共 4 个 breakpoint）

**关键优化**（`system_prompt.py`）：

```python
# 日期级时间戳（而非分钟级）保持 system prompt 字节稳定
timestamp_line = f"Conversation started: {now.strftime('%A, %B %d, %Y')}"
# 关键注释：
# "Date-only (not minute-precision) so the system prompt is byte-stable
#  for the full day. Minute-precision changes invalidate prefix-cache KV
#  on every rebuild path."
```

### 4.3 对比

| 实现 | 方式 | 适用 Provider |
|------|------|---------------|
| Crux `StablePrefix` | 客户端 fingerprint + append-only log | 全部（Anthropic/OpenAI/DeepSeek） |
| hermes-agent `prompt_caching.py` | 服务端 cache_control 注入 | 仅 Anthropic |
| oh-my-pi `append-only-context.ts` | 与 Crux 类似 | 全部 |

## 五、pi-ai-go 现状

### 5.1 已实现的部分

pi-ai-go 已经在 provider 层跟踪 cache read tokens：

```go
// providers/anthropic/anthropic.go:491
msg.Usage.CacheRead = int(getFloat(usage, "cache_read_input_tokens"))

// providers/compat/compat.go:363-365
if v := getFloat(src, "prompt_tokens_details.cached_tokens"); v > 0 {
    dst.CacheRead = int(v)
} else if v := getFloat(src, "cached_tokens"); v > 0 {
    dst.CacheRead = int(v)
}
```

并且在 `core.Usage` 中已经定义 `CacheRead` 字段。

### 5.2 缺失的部分

pi-ai-go 目前**没有** StablePrefix 优化层：

- ❌ 没有 `StablePrefix` 类型
- ❌ 没有 `AppendOnlyLog`
- ❌ 没有 fingerprint 机制
- ❌ 没有显式 cache_control 注入（对 Anthropic 必需）
- ❌ 没有 append-only 保证

这意味着 pi-ai-go 的用户在多轮对话中可能因以下原因触发 cache miss：

1. `convertAssistantContent` 每次重新序列化 message（map 顺序随机）
2. System prompt 中包含时间戳等动态内容
3. Tool schema 顺序变化

## 六、建议的实现方案

如果要在 pi-ai-go 中实现 StablePrefix，可以参考 Crux 的设计：

### 6.1 核心组件

```go
// agent/stable_prefix.go
package agent

type StablePrefix struct {
    mu       sync.RWMutex
    snapshot *StablePrefixSnapshot
    version  int
}

type StablePrefixSnapshot struct {
    SystemPrompt string
    Tools        []core.Tool
    Fingerprint  string
}

func (s *StablePrefix) Build(systemPrompt string, tools []core.Tool) bool {
    fp := computeFingerprint(systemPrompt, tools)
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.snapshot != nil && s.snapshot.Fingerprint == fp {
        return false
    }
    s.snapshot = &StablePrefixSnapshot{...}
    s.version++
    return true
}
```

### 6.2 集成点

在 `agent/agent-loop.go` 的 `streamAssistantResponse` 中：

```go
func streamAssistantResponse(...) {
    // 1. 计算 fingerprint
    prefixChanged := aoc.Prefix.Build(config.SystemPrompt, config.Tools)
    if prefixChanged {
        // 前缀变了，整个 LLM 调用都算 cache miss
    }
    
    // 2. 构造稳定的 messages
    llmMessages := aoc.Log.Append(messages)
    
    // 3. 对 Anthropic，注入 cache_control markers
    if config.Model.API == core.APIAnthropicMessages {
        llmMessages = injectCacheControl(llmMessages)
    }
    
    // 4. 调用 LLM
    ...
}
```

### 6.3 Anthropic cache_control 注入

```go
func injectCacheControl(messages []core.Message) []core.Message {
    result := make([]core.Message, len(messages))
    copy(result, messages)
    
    breakpointsUsed := 0
    
    // 1. System prompt
    if len(result) > 0 && result[0].Role == "system" {
        result[0].CacheControl = &core.CacheControl{Type: "ephemeral"}
        breakpointsUsed = 1
    }
    
    // 2-4. 最后 3 条非 system 消息
    remaining := 4 - breakpointsUsed
    for i := len(result) - 1; i >= 0 && remaining > 0; i-- {
        if result[i].Role != "system" {
            result[i].CacheControl = &core.CacheControl{Type: "ephemeral"}
            remaining--
        }
    }
    
    return result
}
```

## 七、最佳实践总结

### 7.1 System Prompt 设计

✅ **应该：**
- 将动态内容（如时间戳）放在 message 内部，而不是 system prompt
- 使用日期级精度（"2026-06-15"）而非秒级精度
- 将 tool 描述写得稳定（不要在描述中嵌入时间戳）

❌ **不应该：**
- 在 system prompt 中包含 `Conversation started: 2026-06-15 10:30:45`
- 在 tool 描述中嵌入动态内容
- 每次重新生成 tool schema

### 7.2 Tool Schema 设计

✅ **应该：**
- 工具按名称排序（避免 slice 顺序变化）
- 工具定义缓存一次后复用
- 使用稳定的 JSON 序列化（`json.Marshal` 后再 hash）

❌ **不应该：**
- 每次调用都重新构造 tool slice
- 改变工具的参数顺序
- 在 description 中嵌入时间戳

### 7.3 Messages 管理

✅ **应该：**
- 只 append 新消息，不修改历史消息
- 压缩时用固定的 marker（如 `"[truncated]"`）而非动态内容
- 限制消息数量（避免无限增长）

❌ **不应该：**
- 在循环中重新序列化整个 messages slice
- 修改历史消息的内容（如修正拼写）
- 改变消息顺序

## 八、实际收益估算

以一个典型的 Claude Sonnet 多轮对话为例：

| 场景 | 输入 tokens | 单价 | 每轮成本 |
|------|------------|------|----------|
| 无 prefix cache | 5000 | $3/1M | $0.015 |
| 90% 命中 | 5000 (400 cached + 4600 fresh) | $0.30 + $3 per 1M | $0.0014 |
| 节省 | | | **90%** |

对于每天 100 万轮对话的服务：
- 无优化：$15,000/天
- 有优化：$1,500/天
- 节省：**$13,500/天**（$4M+/年）

## 九、参考资料

### 9.1 源码实现

- **Crux StablePrefix**：`/mnt/workspace/crux/crux/crux-agent-runtime/agent/append_only_context.go` (335 行)
- **Crux Bench**：`/mnt/workspace/crux/crux/crux-agent-runtime/agent/bench_test.go` (验证 27/30 命中率)
- **hermes-agent cache**：`/mnt/workspace/hermes-agent/agent/prompt_caching.py` (Anthropic 专用)
- **hermes-agent prompt**：`/mnt/workspace/hermes-agent/agent/system_prompt.py` (字节稳定设计)
- **oh-my-pi/agent**：`append-only-context.ts` (BSD-3, 原始来源)

### 9.2 pi-ai-go 现状

- **Cache token 跟踪**：`providers/anthropic/anthropic.go:491`、`providers/compat/compat.go:363-365`
- **Usage 定义**：`core.Usage.CacheRead` 字段
- **缺失**：StablePrefix、AppendOnlyLog、Anthropic cache_control 注入

### 9.3 官方文档

- Anthropic Prompt Caching: <https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching>
- OpenAI Prompt Caching: <https://platform.openai.com/docs/guides/prompt-caching>
- DeepSeek API Pricing: <https://api-docs.deepseek.com/quick_start/pricing>
