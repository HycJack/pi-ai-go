# arXiv API 参考

## API 端点

```
https://export.arxiv.org/api/query
```

## 查询参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `search_query` | 搜索关键词，支持 AND/OR | `all:agent AND all:LLM` |
| `sortBy` | 排序字段 | `submittedDate`, `relevance`, `lastUpdatedDate` |
| `sortOrder` | 排序方向 | `ascending`, `descending` |
| `start` | 分页偏移 | `0` |
| `max_results` | 返回数量上限（≤100） | `10` |

## 搜索前缀

| 前缀 | 含义 |
|------|------|
| `all` | 搜索所有字段 |
| `ti` | 仅搜索标题 |
| `au` | 仅搜索作者 |
| `abs` | 仅搜索摘要 |

## XML 响应结构

```
feed
└── entry
    ├── id          → Paper.link
    ├── title       → Paper.title
    ├── published   → Paper.published
    ├── author/name → Paper.authors[*]
    └── summary     → Paper.summary
```

## 已知问题

| 问题 | 影响 | 处理方式 |
|------|------|----------|
| 标题含换行符 | 输出格式被破坏 | 脚本自动替换 `\n` 为空格 |
| 摘要过长 | 终端输出拥挤 | `short_summary()` 默认截断 300 字符 |
| 网络超时 | 请求失败 | 默认 30s 超时，报错提示用户重试 |
| API 限速 | 短时间内大量请求被限制 | 建议两次请求间隔 ≥3 秒 |
