# AGENTS.md — geogebra-app

## 构建和运行

```bash
# 安装前端依赖
cd frontend && npm install && cd ..

# 开发模式（热重载）
wails dev

# 构建
wails build

# 仅编译 Go（不打包前端）
go build ./...
```

Go 1.25+，Wails v2。唯一外部依赖 `pi-ai-go`（通过 `replace pi-ai-go => ../../` 指向项目根）。

## 架构

Wails 桌面应用，Go 后端 + React 前端。

```
main.go          — Wails 入口，embed 前端 dist
app.go           — App 生命周期、设置加载/保存、模型解析
geogebra.go      — LLM 流式调用 + GeoGebra 结果解析
conversations.go — 会话持久化（JSON 文件）
models.go        — 模型列表获取（OpenAI/Anthropic API）
logger.go        — 文件日志（按日轮转）
types.go         — App 结构体、请求/响应类型
```

### 前端

```
frontend/src/
  App.tsx                  — 主界面，会话管理 + GeoGebra 交互
  components/
    GeoGebraWorkspace.tsx  — 主画布（2D/3D 切换）
    GeogebraRunner.tsx     — 右侧运行器面板（GGB/HTML/SVG 标签页）
    GeoGebra.tsx           — 独立 GeoGebra 组件
    ChatMessage.tsx        — 聊天消息渲染
    ChatInput.tsx          — 输入框
    ScriptEditor.tsx       — GGB 脚本编辑器
    SettingsPanel.tsx      — 设置面板
    Sidebar.tsx            — 会话列表
  lib/geogebra-lint/       — GeoGebra 命令 lint（509 条命令签名）
  types/index.ts           — 前端类型定义
```

### 数据流

1. 前端 `GeogebraMessage(payload)` → Go 后端
2. Go 构建提示词（skills 加载 + `session.BuildSystemPrompt`）→ `llm.StreamSimpleWithContext`
3. SSE 流式返回 → 前端 `geogebra-text-delta` 事件
4. 完成后 `geogebra-done` 事件携带解析后的 GGB 代码和 HTML
5. 前端 lint 校验 GGB 代码，有错误则自动调 `GeogebraValidateAndRegenerate` 重试（最多 2 次）

## Skills 机制

提示词通过 `session.LoadSkills("skills/")` 从 `skills/geogebra-commands/SKILL.md` 加载，用 `session.BuildSystemPrompt` 拼接。Skills 懒加载一次后缓存（`sync.Once`）。

添加新 skill：在 `skills/` 下新建 `<name>/SKILL.md`，无需改 Go 代码。

## GeoGebra 加载

`deployggb.js` 已下载到 `frontend/public/deployggb.js`，优先本地加载，CDN 作为 fallback。三个组件均使用此策略：`GeoGebraWorkspace`、`GeoGebra`、`GeogebraRunner`。

## 画布重置

`reset()` 通过 `getAllObjectNames()` + `deleteObject(name)` 逐个删除对象。`DeleteAll` 命令不存在于 GeoGebra API。新建会话和切换会话时调用 `reset()`。

## 会话持久化

会话存储在 `~/.geogebra-app/conversations/<id>.json`，设置在 `~/.geogebra-app/settings.json`，日志在 `~/.geogebra-app/logs/`。

## 注意事项

- System prompt 必须用 `core.Context.SystemPrompt` 字段，不能放 `core.SystemMessage` 在 messages 数组里（Anthropic provider 不扫描 messages 里的 system message）
- GeoGebra 命令区分大小写，`Polygon` 不是 `polygon`
- `commandSignatures.json`（491KB，509 条命令）从 `frontend/src/lib/geogebra-lint/specs/` 复制到 skill references
- 前端 lint 校验在 `lib/geogebra-lint/validator.ts`，规则在 `rules/` 下
