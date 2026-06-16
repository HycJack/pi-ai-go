# arXiv Paper Search 规范

## 意图

通过 arXiv API 搜索学术论文并格式化输出的自动化工作流。将检索、解析、输出三步流程封装为可复用的 CLI 脚本，同时提供结构化数据模型供编程调用。

## 范围

范围内：
- arXiv API 查询构建与参数编码
- XML 响应解析为 Paper 数据类
- 纯文本 + Markdown 双格式输出
- 命令行参数：关键词、数量、排序、分页

范围外：
- arXiv 以外的学术搜索源
- 论文全文下载或分析
- GUI 或 Web 界面
- 持久化存储或数据库写入

## 用户与触发情境

- 主要用户：研究者、开发者、AI 工程师
- 常见请求："搜索 arXiv 论文"、"找 Agent 相关论文"、"文献调研 LLM"、"最新 SOTA 论文"
- 不应触发：数据库查询、代码搜索、GitHub 搜索、本地文件检索

## 运行时契约

- 必需第一步：解析 CLI 参数构建 ArxivSearcher 实例
- 必需输出：论文列表（含标题、作者、日期、摘要、链接）
- 不可协商约束：脚本非交互式，输出到 stdout
- 预期运行时文件：`scripts/arxiv_search.py`、`references/api-reference.md`

## 来源与证据模型

权威来源：
- arXiv API 官方文档 (https://arxiv.org/help/api)

改进来源：
- 正面示例：多关键词组合查询、分页翻页、Markdown 输出
- 负面示例：未处理 XML 换行符导致格式损坏、未截断摘要导致输出冗长

不得存储的数据：
- 用户私密 API 密钥
- 账号信息

## 参考架构

- `SKILL.md` 包含：快速开始、步骤概览、参数表、依赖声明
- `references/api-reference.md` 包含：API 端点、查询参数、XML 结构、已知问题
- `scripts/arxiv_search.py` 包含：Paper 数据类、ArxivSearcher 类、CLI 入口

## 验证

- 轻量验证：`python scripts/arxiv_search.py -n 1` 确认能返回结果
- 深入验证：对比返回结果数量与 `max_results` 参数是否一致

## 已知限制

- 单次最多返回 100 篇，超过需分页
- 依赖网络，离线不可用
- arXiv API 有非官方限速，高频请求可能被拒

## 维护说明

- 何时更新 `SKILL.md`：新增参数、修改默认查询、调整输出格式
- 何时更新 `references/api-reference.md`：arXiv API 变更或发现新问题
