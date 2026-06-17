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

## 精准搜索策略

### 问题：`all:` 搜索噪音大

arXiv API 的 `all:` 前缀匹配论文的**所有字段**（标题、摘要、作者、评论等）。搜索 `all:agent` 会返回大量仅在某处提及 "agent" 的无关论文，如自动驾驶（autonomous agent）、药物设计（molecular agent）、BI 数据分析（intelligent agent）等。

### 核心技巧

| 技巧 | 效果 |
|------|------|
| **用 `ti:` 替代 `all:`** | 大幅减少噪音——标题不含关键词的论文直接过滤 |
| **加排除词** `-ti:xxx` | 排除已知的噪声领域 |
| **用 `abs:` 精确补充** | 需要语义匹配时用摘要搜索，比 `all:` 干净 |
| **引号短语匹配** | `ti:"multi-agent system"` 比 `ti:multi AND ti:agent` 准确得多 |
| **限定日期** | `AND submittedDate:[2025 TO 2026]` 排除旧论文 |

### 示例对比

```
# ❌ 宽泛：100+ 结果，前10篇可能只有3篇相关
all:agent AND all:self-evolution

# ✅ 精准：标题必须同时含两词，排除自动驾驶/游戏/药物
ti:agent AND ti:self-evolution -ti:autonomous -ti:driving -ti:game -ti:drug -ti:chemistry
```

### 按领域预设排除词

| 目标领域 | 排除 |
|----------|------|
| LLM Agent | `-ti:autonomous -ti:driving -ti:vehicle -ti:slam -ti:robotics -ti:drug -ti:disease -ti:clinical -ti:business -ti:SQL` |
| NLP / 语言模型 | `-ti:slam -ti:robotics -ti:driving -ti:drug -ti:chemistry -ti:biology` |
| 多智能体系统 | `-ti:game -ti:simulation -ti:chess -ti:RL -ti:reinforcement` |
| 通用 AI 论文 | `-ti:drug -ti:molecule -ti:disease -ti:clinical -ti:patient -ti:gene` |

## 已知问题

| 问题 | 影响 | 处理方式 |
|------|------|----------|
| 标题含换行符 | 输出格式被破坏 | 脚本自动替换 `\n` 为空格 |
| 摘要过长 | 终端输出拥挤 | `short_summary()` 默认截断 300 字符 |
| 网络超时 | 请求失败 | 默认 30s 超时，报错提示用户重试 |
| API 限速 | 短时间内大量请求被限制 | 建议两次请求间隔 ≥3 秒 |
| `all:` 搜索噪音大 | 返回大量无关论文 | 用 `ti:` + 排除词，见上方「精准搜索策略」 |
| Windows cmd 参数含空格 | 引号内参数被截断 | 直接在 Python 中调用 `ArxivSearcher`，或使用 Git Bash |
