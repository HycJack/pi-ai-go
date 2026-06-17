---
name: arxiv-paper-search
description: "通过 arXiv API 搜索学术论文，解析 XML 并格式化输出。当用户需要搜索最新论文、查找 AI/LLM/Agent 等研究领域文献、或进行文献调研时使用。支持中文和英文关键词，可按日期或相关性排序。"
---

# arXiv 论文搜索

通过 arXiv API 检索论文，结构化解析 XML 响应，输出纯文本或 Markdown 格式结果。

## 步骤

| 步骤 | 操作 | 说明 |
|------|------|------|
| 1 | 构建请求 URL | `ArxivSearcher.build_url()` 拼接查询参数 |
| 2 | 获取并解析 XML | `ArxivSearcher.fetch()` → `_parse()` |
| 3 | 格式化输出 | `print_text()` 或 `print_markdown()` |

## 查询语法

### 基本规则

| 规则 | 示例 | 说明 |
|------|------|------|
| 字段前缀 | `ti:agent` | 仅搜索标题，`all:`=全局，`abs:`=摘要，`au:`=作者 |
| AND | `ti:agent AND ti:self-evolution` | 同时匹配多个条件 |
| 短语精确匹配 | `all:"reinforcement learning"` | 双引号包围多词短语，避免拆词 |
| 排除无关结果 | `-ti:slam -ti:robotics` | 减号排除不相关的领域词 |
| 日期范围 | `AND submittedDate:[2025 TO 2026]` | 限定时间窗口 |

### 精准查询模板

**推荐做法**：用 `ti:` 缩小到标题、加排除词、限定日期：

```bash
# ✅ 精准：标题必须含 agent+LLM，排除自动驾驶/机器人
python scripts/arxiv_search.py -q "ti:agent AND ti:LLM -ti:autonomous -ti:slam -ti:robotics AND submittedDate:[2025 TO 2026]" -n 10 -m

# ✅ 精准：用短语匹配 + 双条件
python scripts/arxiv_search.py -q "ti:\"multi-agent\" AND all:cooperation -ti:game -ti:simulation" -n 10 -m

# ❌ 太宽泛：all:agent 会匹配大量无关论文
python scripts/arxiv_search.py -q "all:agent AND all:LLM" -n 10 -m
```

### 常见排除词

| 领域 | 排除词 |
|------|--------|
| 自动驾驶 | `-ti:autonomous -ti:driving -ti:vehicle -ti:lane -ti:traffic` |
| 机器人 / SLAM | `-ti:slam -ti:robotics -ti:robot -ti:grasping -ti:manipulation` |
| 游戏 / 模拟 | `-ti:game -ti:simulation -ti:chess -ti:RL -ti:reinforcement` |
| 药物 / 化学 | `-ti:drug -ti:molecule -ti:chemistry -ti:disease -ti:clinical` |
| BI / 数据分析 | `-ti:business -ti:analytics -ti:SQL -ti:database` |
| 教育 | `-ti:education -ti:student -ti:classroom -ti:course` |

## 命令行

```bash
python scripts/arxiv_search.py --query "ti:agent AND ti:self-evolution" --max 10 --sort submittedDate
```

| 参数 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| `--query` | `-q` | `ti:agent AND ti:LLM` | 检索关键词 |
| `--max` | `-n` | `10` | 最大返回数量（≤100） |
| `--sort` | `-s` | `submittedDate` | `relevance` / `lastUpdatedDate` / `submittedDate` |
| `--markdown` | `-m` | `false` | Markdown 格式输出 |
| `--start` | — | `0` | 分页起始位置 |
| `--json` | — | `None` | JSON 字符串或 .json 文件路径，绕过 shell 空格截断 |

### `--json` 参数（推荐）

**在 Windows 上，含空格的查询参数会被 shell 截断。** 使用 JSON 文件方式完全绕过：

```bash
# ✅ 推荐：JSON 文件方式（Windows / 跨平台通用，绕过所有 shell 转义）
# 先写参数到临时文件，再传入脚本
python scripts/arxiv_search.py --json params.json

# ⚠️ JSON 字符串方式可能在 Windows PowerShell 下仍然被截断
python scripts/arxiv_search.py --json '{"query":"ti:agent AND ti:self-evolution","max":10,"markdown":true}'
```

JSON 文件示例 `params.json`：

```json
{"query": "ti:agent AND ti:self-evolution -ti:game -ti:driving", "max": 10, "markdown": true}
```

支持的字段：`query`、`max` / `max_results`、`sort`、`markdown`、`start`。

## 依赖

Python 3.7+，纯标准库（无外部依赖）。

## 参考

| 打开时机 | 文件 |
|----------|------|
| 查看 API 端点、查询参数、XML 结构、已知问题 | `references/api-reference.md` |
