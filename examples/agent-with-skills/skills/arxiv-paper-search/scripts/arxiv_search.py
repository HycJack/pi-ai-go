"""
arXiv 论文搜索脚本
==================
通过 arXiv API 检索论文，解析 XML 并以结构化方式输出。

用法:
    python arxiv_search.py --query "all:agent AND all:LLM" --max 10 --sort submittedDate

依赖: Python 3.7+（纯标准库，无外部依赖）
"""

import argparse
import dataclasses
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET
from typing import List


# ── 数据结构 ──────────────────────────────────────────────


@dataclasses.dataclass
class Paper:
    title: str
    published: str       # YYYY-MM-DD
    authors: List[str]
    summary: str
    link: str

    def short_authors(self, max_n: int = 5) -> str:
        shown = self.authors[:max_n]
        suffix = "..." if len(self.authors) > max_n else ""
        return ", ".join(shown) + suffix

    def short_summary(self, max_len: int = 300) -> str:
        if len(self.summary) <= max_len:
            return self.summary
        return self.summary[:max_len] + "..."

    def to_markdown(self) -> str:
        return (
            f"### {self.title}\n"
            f"- 📅 {self.published}  |  👥 {self.short_authors()}\n"
            f"- {self.short_summary()}\n"
            f"- 🔗 {self.link}\n"
        )


# ── 核心流程 ──────────────────────────────────────────────


class ArxivSearcher:
    """arXiv API 搜索器，封装查询、解析、输出全流程。"""

    BASE_URL = "https://export.arxiv.org/api/query"
    NS = {
        "atom": "http://www.w3.org/2005/Atom",
        "arxiv": "http://arxiv.org/schemas/atom",
    }

    def __init__(self, query: str, max_results: int = 10, sort_by: str = "submittedDate"):
        self.query = query
        self.max_results = max_results
        self.sort_by = sort_by

    # ── Step 1: 构建请求 ──
    def build_url(self, start: int = 0) -> str:
        params = {
            "search_query": self.query,
            "sortBy": self.sort_by,
            "sortOrder": "descending",
            "start": start,
            "max_results": self.max_results,
        }
        return self.BASE_URL + "?" + urllib.parse.urlencode(params)

    # ── Step 2: 获取并解析 ──
    def fetch(self, start: int = 0) -> List[Paper]:
        url = self.build_url(start)
        print(f"\n[arXiv] 检索: {url}")

        with urllib.request.urlopen(url, timeout=30) as resp:
            content = resp.read().decode("utf-8")

        return self._parse(content)

    def _parse(self, xml_content: str) -> List[Paper]:
        root = ET.fromstring(xml_content)
        papers: List[Paper] = []

        for entry in root.findall("atom:entry", self.NS):
            title = (entry.find("atom:title", self.NS).text or "").strip().replace("\n", " ")
            published = (entry.find("atom:published", self.NS).text or "")[:10]
            authors = [
                (a.find("atom:name", self.NS).text or "")
                for a in entry.findall("atom:author", self.NS)
            ]
            summary = (entry.find("atom:summary", self.NS).text or "").strip().replace("\n", " ")
            link = (entry.find("atom:id", self.NS).text or "").strip()

            papers.append(Paper(
                title=title,
                published=published,
                authors=authors,
                summary=summary,
                link=link,
            ))

        return papers

    # ── Step 3: 输出 ──
    @staticmethod
    def print_text(papers: List[Paper]):
        print(f"\n{'='*80}")
        print(f"[arXiv] 共找到 {len(papers)} 篇论文")
        print(f"{'='*80}\n")
        for i, p in enumerate(papers, 1):
            print(f"【{i}】{p.title}")
            print(f"    日期: {p.published}  |  作者: {p.short_authors()}")
            print(f"    摘要: {p.short_summary()}")
            print(f"    链接: {p.link}")
            print("-" * 80)

    @staticmethod
    def print_markdown(papers: List[Paper]):
        print(f"\n## [arXiv] ArXiv 检索结果（共 {len(papers)} 篇）\n")
        for p in papers:
            print(p.to_markdown())


# ── 入口 ──────────────────────────────────────────────────


def main():
    parser = argparse.ArgumentParser(description="arXiv 论文搜索工作流")
    parser.add_argument("--query", "-q", default="all:agent AND all:LLM AND all:autonomous",
                        help="检索关键词（支持 AND / OR 语法）")
    parser.add_argument("--max", "-n", type=int, default=10,
                        help="最大返回数量（默认 10）")
    parser.add_argument("--sort", "-s", default="submittedDate",
                        choices=["relevance", "lastUpdatedDate", "submittedDate"],
                        help="排序方式（默认 submittedDate）")
    parser.add_argument("--markdown", "-m", action="store_true",
                        help="以 Markdown 格式输出")
    parser.add_argument("--start", type=int, default=0,
                        help="分页起始位置")
    args = parser.parse_args()

    searcher = ArxivSearcher(
        query=args.query,
        max_results=args.max,
        sort_by=args.sort,
    )

    papers = searcher.fetch(start=args.start)

    if args.markdown:
        ArxivSearcher.print_markdown(papers)
    else:
        ArxivSearcher.print_text(papers)


if __name__ == "__main__":
    main()
