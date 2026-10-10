# spec: 完善 pi-ai-go 的 ai 模块（参考 PiG 实现)

- 日期: 2025-06-12
- 模块: `github.com/HycJack/pi-ai-go`
- 范围: `core` + `llm` + `providers/*`（"ai 模块"）
- 定位: 务实子集。把行为正确性对齐 Palatine 参考实现 PiG；保留 pi-ai-go 的 SDK 分层与清爽 API，接受破坏性改动（升主版本至 v1.0.0）。

## 1. 背景

pi-ai-go 是一个统一多模型 Go SDK，分层为 `core`（类型/事件流/注册表/错误）→ `llm`（公开 API/模型目录）→ `providers/*`（各实现）。它与 PiG（Palatine 的 Go 移植）同源于上游 pi 的 `packages/ai`，但作为独立 SDK 走了一条更轻的路线，在行为可靠性上与 PiG 存在差距。

经逐项对比（provider 接口、事件流语义、transcript 处理、模型目录、错误面、可观测性），核心差距集中在三处 P0 行为问题，另有若干 P1 增量。

## 2. 目标

- **P0（必须）**：修复 EventStream 背压丢事件；新增 transcript 归一化层；新增事件序列校验。
- **P1（增量）**：模型目录与能力推导；Options/钩子增强；错误分类与重试判定。
- **非目标**：不照搬 PiG 的结构（单大包、每请求构造 provider、`ProviderStreams`/`ModelsProvider` 运行时）；不做 `agent`/`session` 的功能扩展；不做 2.9MB 生成式模型目录。

## 3. 现状（数据已核对）

- 请求上下文: `core.Context{SystemPrompt string; Messages []Message; Tools []Tool}`，值传递，无深拷贝、无校验。
- 事件流: `core.EventStream[T, R]` 缓冲 64 的 channel；`Push` 缓冲满即丢事件并触发 `OnDropped`。
- 事件: 扁平结构（`EventTextDelta{Type, Delta}`），**无** `Partial` 快照、**无** `ContentIndex`、**无**序列校验。
- StreamOptions: 紧凑；`OnPayload func(any)` / `OnResponse func(any)` 单参，无替换能力；无 `Fetch *http.Client`；无 `OnProviderStreamEvent`。
- Provider: `core.APIProvider{Stream(ctx, model, llmCtx Context, opts StdOptions); StreamSimple(...)}`，注册表按 `KnownAPI` 实例注册；`compat.Router` 按 `model.Provider` 路由。

## 4. 架构分层（保留）

保留 `core → llm → providers` 三层。引入不透明 `TranscriptContext`（在 `core` 内，作为归一化的产物类型），把归一化作为 `llm` 公开入口的门面步骤。

```
core    types / errors / registry / EventStream / transcript(NEW) / catalog(NEW)
llm     公开 API（Stream/Complete/...）+ 模型目录查询 + 归一化入口门面
providers/*  各 provider 实现（收到归一化后的不可变 Transcript）
```

## 5. P0 行为修正

### 5.1 EventStream 永不丢事件（破坏性）

**参考**: PiG 的 `AssistantMessageEventStream`（无界 FIFO 队列 + waiter；`Result` 与迭代解耦；取消只摘 waiter 不留 goroutine；`Push` 校验并返回 error）。

**pi-ai-go 现状**: 缓冲 64 channel，`Push` 满即丢 + `OnDropped` 回调。这是可靠性头号缺陷。

**设计**:

- 内部改为**无界 FIFO 队列 + waiter（通知 channel）**，删除固定容量和丢事件路径。
- `Push(event T) error`：入队；返回校验错误（见 5.3）；终止后返回 nil（忽略，对齐 PiG）。
- `End(result R)` / `Error(err error)`：设置**单一终止结果**，关闭队列（消费者看到结束）。
- `Result() (R, error)`：阻塞等待终止，与迭代解耦；取消（`Stop`/ctx）负责释放等待，不泄漏 goroutine。
- `ForEach(ctx, fn)`：迭代消费；ctx 取消中止迭代并释放等待。
- 删除 `OnDropped`（背压语义消失）。

**破坏面/迁移**:

| 项 | 旧 | 新 |
|---|---|---|
| `Push` | `bool` | `error` |
| `OnDropped` | 存在 | 删除 |
| `Events()`/内部 `ch` | channel | 队列 + waiter（内部类型，对外 `for range`/`ForEach` 语义保持） |
| `End/Error` | channel close | 单一终止 + 队列结束 |

- 连带: `agent` 包 `AgentEventStream = core.EventStream[AgentEvent, []Message]`，**自动受益**（不丢事件）。需按 `Push error` 适配调用点。
- context 取消: 保持 `ForEach(ctx, ...)` 取消语义。

### 5.2 Transcript 归一化层（新增，非破坏）

**参考**: PiG `NormalizeContext` → 不透明 `TranscriptContext`（深拷贝、校验、SystemPrompt+Tools 折进首条 SystemMessage、缺失 content 归一化为空块数组）。

**pi-ai-go 现状**: `Context` 原样透传，无校验、深拷贝、可变。

**设计**:

- `core/transcript.go`:
  - `func NormalizeContext(ctx Context) (Context, error)`：**校验**（role 与内容形状，非法即报错）、**深拷贝**（Messages、Content 切片、Tools，隔离调用方后续修改）、**空内容归一化**（nil user content → 空字符串；nil assistant/tool-result content → 空块数组）。
  - **SystemPrompt + Tools 保留原样，不折叠进 Messages**（务实偏差，见 §11）：pi-ai-go 的 provider 自行把 `SystemPrompt` 注入请求体，且 `providers/openai/convert.Messages` **不渲染**位于 Messages 数组内的 `SystemMessage`（会静默丢弃）。折叠会重复注入或丢失系统提示，故不做。
- `llm.Stream/Complete/StreamSimple/StreamSimpleWithContext/CompleteSimple` 等**公开入口先调 `NormalizeContext`**，provider 收到不可变快照。对外调用方仍传 `Context`，不改参数。
- 校验在公开边界拒绝：非法 role、非法内容类型。

### 5.3 事件序列校验（增强）

**参考**: PiG `Push` 对封闭联合的校验。

**设计**: 在 5.1 的新 `Push` 中校验：

- 事件顺序: `start → textStart/thinkingStart/toolCallStart → ...delta... → end`。
- 单终止结果: `End`/`Error` 只能一次；终止后 `Push` 忽略。
- 拒绝: 缺失必要字段、非有限数值、非法封闭联合成员 → 返回 `error`。

**有意省略（二期可选）**: `Partial *AssistantMessage` 快照与 `ContentIndex` 字段。原因是波及每个 provider 的增量构建路径，务实子集先不引入；如需与 PiG 事件形状完全一致再评估。

## 6. P1 增量

### 6.1 模型目录与能力推导（新增，兼容）

- `llm/models_generated.go` 扩为**可查询种子目录**：`LookupModel("provider/id")`、`LookupModelExact("provider/id")`、`ListModels(provider)`、`ToCapabilities()`。
- 借鉴 PiG `GeneratedModel`/registry 的*概念*，不做生成式大目录——种子目录约 100 行 + 支持注册扩展模型。
- **保留**手工 `core.Model{...}` 构造（已有调用方与 README 不破坏）。

### 6.2 Options/钩子增强

- ✅ `Fetch *http.Client`：已实现。新增 `core.StreamOptions.Fetch` + `core.RequestClient(opts)`；已接入所有 SSE provider（anthropic/compat/bedrock/google/vertex/mistral/openai-responses）替换 `SSEClient.Do`。纯增量、不破坏。
- ⏳ `OnPayload` / `OnResponse` 增加 `model` 参数与**载荷替换能力**：**暂缓**（破坏性签名变更，波及 7 个 provider 调用点，且 `examples/kimi-deepseek` 有真实外部调用方 `func(data any)`）。
- ⏳ `OnProviderStreamEvent`：provider 原始流事件观察点：**暂缓**（需各 provider 暴露其原始 SSE 事件，工作量较大且价值可替代）。

### 6.3 错误面

- 保留 `core` 的 typed error（`AuthError`/`RateLimitError`/`ServerError`/`NetworkError`/`TimeoutError` 等）。
- **已有实现，无需新增**：`core/retry.go` 已提供完整的重试判定（`IsRetryableError`）与重试循环（`Retry` + 指数退避 + `RateLimitError.RetryAfter` + ctx 取消）。语义**正确**：RateLimit/Server/Network → 可重试；Abort/Canceled/Timeout/Overflow → 不重试；**Auth → 不重试**（401/403 重试无意义，规范初稿误写为"Auth 可重试"，已按正统语义修正，且现有实现本就如此）。
- 事件终止语义：`EventError` 归一化为统一终止结果（阶段 1 已实现）。

## 7. Provider 接口（务实微调，几乎不动）

**不照搬** PiG 的"每请求构造 provider / `ProviderStreams` / `ModelsProvider`"，那属于产品/SDK 层，超出"ai 模块"且不符务实子集。

仅两处微调：

1. 公开入口按 5.2 归一化后再交给 provider 实现（接口签名不变，语义增强）。
2. 保留 `compat.Router` 按 `model.Provider` 路由的机制。

回归风险低，现有 provider 实现基本不动，只需在 `llm` 层接入归一化。

## 8. 测试策略（对齐 PiG 验证纪律）

- 红色先行：为每个 P0 写失败测试——丢事件（断言新实现不丢）、序列错乱（应报错）、空/缺 content 归一化、取消竞态（`ForEach` 中取消）、并发 `Push`/迭代/终止（race detector）。
- provider 载荷用**表驱动断言序列化请求形状**（含调用 `NormalizeContext` 后的产物），不只断言返回成功。
- 破坏点同步更新现有测试。
- `go test -race ./...`、`go vet ./...` 全绿为准。

## 9. 分阶段实施

| 阶段 | 内容 | 退出条件 |
|---|---|---|
| 1 | P0-5.1 EventStream 重写 + 迁移 `agent` 用法 | `go test -race ./...` 全绿，丢事件测试红→绿 |
| 2 | P0-5.2 Transcript 归一化 + 接入 `llm` 入口 | 覆盖 provider（anthropic/openai/google/mistral/compat）的归一化用例 |
| 3 | P0-5.3 事件序列校验 | 序列错乱/单终止/终止后忽略 用例通过 |
| 4 | P1：模型目录、Options/钩子、错误分类 | 各增量单测通过，`go vet` 干净 |
| 5 | 文档/迁移说明 + `core.Version` → v1.0.0 | README 更新破坏面与迁移路径 |

## 10. 破坏面与迁移（汇总）

确切破坏:

1. `core.EventStream.Push` 返回 `bool → error`；`OnDropped` 删除。
2. `OnPayload` / `OnResponse` 签名（单参 → 参考 PiG 签名）。
3. 内部 `Events()`/channel 结构（对外迭代语义保持）。
4. 任何依赖"缓冲满丢事件"行为的调用方（不存在于可靠场景，理论上不应有）。

保留不变: `ForEach`/`Result()` 用法、`Context` 传参、`APIProvider` 接口两方法、手工 `Model{}` 构造、`KnownAPI`/`KnownProvider` 常量、typed error。

版本: `core.Version` 从 `v0.0.1` 升为 `v1.0.0`（破坏性）。

## 11. 参考实现对照

| 能力 | pi-ai-go 现状 | PiG 参考 | 本方案 |
|---|---|---|---|
| 事件背压 | 缓冲 64，丢事件 | 无界，永不丢 | 无界，永不丢（5.1） |
| transcript | 原样透传 | NormalizeContext → 不透明 TranscriptContext | 归一化门面（5.2）；**偏差**：不折叠 SystemPrompt/Tools（provider 自行注入 SystemPrompt，convert.Messages 不渲染 SystemMessage，折叠会重复或丢失），改为校验 + 深拷贝 + 空内容归一化 |
| 事件校验 | 无 | Push 封闭联合校验 | 序列校验（5.3） |
| Partial 快照 | 无 | ContentIndex + Partial | 二期可选 |
| 模型目录 | 手工构造 | GeneratedModel + registry | 种子目录 + 查询（6.1） |
| 钩子 | OnPayload/OnResponse 单参 | 带替换/观察能力 | Fetch 已实现；签名变更暂缓（6.2） |
| Provider 形状 | 接口 + 实例注册表 | ProviderStreams/每请求构造 | 保留（7） |

## 12. 实施状态

| 阶段 | 内容 | 状态 |
|---|---|---|
| 1 | 5.1 EventStream 永不丢事件 + `Push bool→error` + 删 `OnDropped` | ✅ 完成（core/events.go；10 个事件测试绿） |
| 2 | 5.2 Transcript 归一化 + llm 入口接入 | ✅ 完成（core/transcript.go；6 个归一化测试绿） |
| 3 | 5.3 事件序列校验 | ⏳ 暂缓（务实子集：partial 快照与序列校验列为二期） |
| 4a | 6.1 模型目录：`LookupModel`/`LookupModelExact`/`ListModels`/`ToCapabilities` | ✅ 完成（llm/models.go；7 个查找测试绿） |
| 4b | 6.2 `Fetch *http.Client` + `RequestClient` | ✅ 完成（core/httpclient.go + 7 个 SSE provider 接线） |
| 4c | 6.2 `OnPayload`/`OnResponse` 签名变更、`OnProviderStreamEvent` | ⏳ 暂缓（破坏性/工作量大） |
| 4d | 6.3 错误分类/重试判定 | ✅ 已有实现（core/retry.go），核实无需新增 |
| 5 | 文档/迁移说明（§10 破坏面）+ `core.Version` → v1.0.0 | ⏳ 未开始 |