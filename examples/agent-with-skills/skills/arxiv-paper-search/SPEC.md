# arXiv Paper Search 规范

## 意图

通过 arXiv API 搜索学术论文并格式化输出。将检索、解析、输出三步流程封装为 CLI 脚本，提供 Paper 数据类供编程调用。

## 范围

| 在范围内 | 不在范围内 |
|----------|------------|
| arXiv API 查询构建与参数编码 | arXiv 以外的学术搜索源 |
| XML 响应解析为 Paper 数据类 | 论文全文下载或分析 |
| 纯文本 + Markdown 双格式输出 | GUI 或 Web 界面 |
| 命令行参数：关键词、数量、排序、分页 | 持久化存储或数据库写入 |

## 用户与触发情境

- 主要用户：研究者、开发者、AI 工程师
- 应触发："搜索 arXiv 论文"、"找 Agent 相关论文"、"文献调研 LLM"、"最新 SOTA 论文"
- 不应触发：数据库查询、代码搜索、GitHub 搜索、本地文件检索

## 运行时契约

- 脚本非交互式，输出到 stdout
- 单次最多返回 100 篇，超过需分页
- 依赖网络，离线不可用

## 参考架构

- `SKILL.md`：快速开始、步骤概览、参数表、依赖声明
- `references/api-reference.md`：API 端点、查询参数、XML 结构、已知问题
- `scripts/arxiv_search.py`：Paper 数据类、ArxivSearcher 类、CLI 入口

## 维护说明

| 何时更新 | 文件 |
|----------|------|
| 新增参数、修改默认查询、调整输出格式 | `SKILL.md` |
| arXiv API 变更或发现新问题 | `references/api-reference.md` |
| 脚本逻辑变更 | `scripts/arxiv_search.py` |
