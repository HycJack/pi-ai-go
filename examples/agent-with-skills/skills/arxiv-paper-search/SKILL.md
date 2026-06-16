---
name: arxiv-paper-search
description: "通过 arXiv API 搜索学术论文，解析 XML 并格式化输出。当用户需要搜索最新论文、查找 AI/LLM/Agent 等研究领域文献、或进行文献调研时使用。支持中文和英文关键词，可按日期或相关性排序。"
---

# arXiv 论文搜索

通过 arXiv API 检索论文，结构化解析 XML 响应，输出纯文本或 Markdown 格式结果。

## 快速开始

```bash
python scripts/arxiv_search.py --query "all:agent AND all:LLM" --max 10
```

## 步骤

| 步骤 | 操作 | 说明 |
|------|------|------|
| 1 | 构建请求 URL | `ArxivSearcher.build_url()` 拼接查询参数 |
| 2 | 获取并解析 XML | `ArxivSearcher.fetch()` → `_parse()` 提取结构化数据 |
| 3 | 格式化输出 | `ArxivSearcher.print_text()` 或 `print_markdown()` |

## 数据模型

```
Paper
├── title: str        # 论文标题
├── published: str    # 发布日期 (YYYY-MM-DD)
├── authors: List[str] # 作者列表
├── summary: str      # 摘要（自动截断）
└── link: str         # arXiv 链接
```

## 命令行参数

| 参数 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| `--query` | `-q` | `all:agent AND all:LLM AND all:autonomous` | 检索关键词 |
| `--max` | `-n` | `10` | 最大返回数量 |
| `--sort` | `-s` | `submittedDate` | 排序方式 |
| `--markdown` | `-m` | `false` | Markdown 格式输出 |
| `--start` | — | `0` | 分页起始位置 |

排序方式可选：`relevance`、`lastUpdatedDate`、`submittedDate`

## 依赖

- Python 3.7+
- 标准库（无外部依赖）

## 参考

| 打开时机 | 文件 |
|----------|------|
| 查看 arXiv API 参数细节和已知问题 | `references/api-reference.md` |
