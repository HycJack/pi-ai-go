#!/usr/bin/env python3
"""arXiv API search script – zero-dependency, Python 3.7+."""

from __future__ import annotations

import argparse
import html
import re
import sys
import time
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET
from dataclasses import dataclass, field
from typing import List, Optional


# ---------------------------------------------------------------------------
# Data
# ---------------------------------------------------------------------------

@dataclass
class Paper:
    title: str
    summary: str
    published: str
    updated: str
    authors: List[str]
    link: str
    categories: List[str]

    def _date(self, raw: str) -> str:
        return raw[:10]  # yyyy-mm-dd

    @property
    def pub_date(self) -> str:
        return self._date(self.published)

    @property
    def upd_date(self) -> str:
        return self._date(self.updated)

    @staticmethod
    def _clean_title(raw: str) -> str:
        return " ".join(raw.split())

    @staticmethod
    def _clean_summary(raw: str) -> str:
        return " ".join(raw.split())

    def summary_truncated(self, limit: int = 300) -> str:
        s = self._clean_summary(self.summary)
        return s[:limit] + ("..." if len(s) > limit else "")

    def authors_str(self) -> str:
        if not self.authors:
            return "N/A"
        shown = self.authors[:5]
        suffix = "..." if len(self.authors) > 5 else ""
        return ", ".join(shown) + suffix

    def categories_str(self) -> str:
        return "，".join(self.categories) if self.categories else "N/A"


# ---------------------------------------------------------------------------
# Searcher
# ---------------------------------------------------------------------------

class ArxivSearcher:
    BASE = "https://export.arxiv.org/api/query"

    def __init__(
        self,
        query: str,
        max_results: int = 10,
        sort_by: str = "submittedDate",
        sort_order: str = "descending",
        start: int = 0,
    ):
        self.query = query
        self.max_results = min(max_results, 100)
        self.sort_by = sort_by
        self.sort_order = sort_order
        self.start = start

    def build_url(self) -> str:
        params = {
            "search_query": self.query,
            "sortBy": self.sort_by,
            "sortOrder": self.sort_order,
            "start": str(self.start),
            "max_results": str(self.max_results),
        }
        return self.BASE + "?" + urllib.parse.urlencode(params)

    def fetch(self) -> List[Paper]:
        url = self.build_url()
        req = urllib.request.Request(url, headers={"User-Agent": "arxiv-skill/1.0"})
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                raw = resp.read().decode("utf-8")
        except Exception as exc:
            print(f"请求失败: {exc}", file=sys.stderr)
            return []
        return self._parse(raw)

    def _parse(self, xml_text: str) -> List[Paper]:
        ns = {
            "atom": "http://www.w3.org/2005/Atom",
            "arxiv": "http://arxiv.org/schemas/atom",
        }
        root = ET.fromstring(xml_text)
        papers: List[Paper] = []
        for entry in root.findall("atom:entry", ns):
            title = self._text(entry, "atom:title", ns)
            summary = self._text(entry, "atom:summary", ns)
            published = self._text(entry, "atom:published", ns)
            updated = self._text(entry, "atom:updated", ns)

            authors = [
                self._text(a, "atom:name", ns)
                for a in entry.findall("atom:author", ns)
            ]

            link = ""
            for l in entry.findall("atom:link", ns):
                if l.attrib.get("title") == "pdf":
                    link = l.attrib.get("href", "")
                    break
            if not link:
                link = self._text(entry, "atom:id", ns)

            cats = [c.attrib.get("term", "") for c in entry.findall("atom:category", ns)]

            papers.append(
                Paper(
                    title=Paper._clean_title(title),
                    summary=summary,
                    published=published,
                    updated=updated,
                    authors=authors,
                    link=link,
                    categories=cats,
                )
            )
        return papers

    @staticmethod
    def _text(el: ET.Element, tag: str, ns: dict) -> str:
        child = el.find(tag, ns)
        return html.unescape(child.text or "").strip() if child is not None else ""


# ---------------------------------------------------------------------------
# Output
# ---------------------------------------------------------------------------

def print_text(papers: List[Paper]) -> None:
    if not papers:
        print("(无结果)")
        return
    for i, p in enumerate(papers, 1):
        print(f"{i}. {p.title}")
        print(f"   日期: {p.pub_date}  |  分类: {p.categories_str()}")
        print(f"   作者: {p.authors_str()}")
        print(f"   摘要: {p.summary_truncated(200)}")
        print(f"   PDF: {p.link}")
        print()


def print_markdown(papers: List[Paper]) -> None:
    if not papers:
        print("_无结果_")
        return
    for i, p in enumerate(papers, 1):
        print(f"### {i}. {p.title}")
        print()
        print(f"- **日期**: {p.pub_date}")
        print(f"- **分类**: {p.categories_str()}")
        print(f"- **作者**: {p.authors_str()}")
        print(f"- **PDF**: [{p.link}]({p.link})")
        print()
        print(f"> {p.summary_truncated(300)}")
        print()


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def main() -> None:
    parser = argparse.ArgumentParser(
        description="arXiv 论文搜索",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  python arxiv_search.py -q "ti:agent AND ti:self-evolution" -n 10 -m
  python arxiv_search.py --json '{"query":"ti:agent AND ti:self-evolution","max":10,"markdown":true}'
        """,
    )
    parser.add_argument("-q", "--query", default="ti:agent AND ti:self-evolution")
    parser.add_argument("-n", "--max", type=int, default=10, dest="max_results")
    parser.add_argument("-s", "--sort", default="submittedDate", dest="sort")
    parser.add_argument("-m", "--markdown", action="store_true")
    parser.add_argument("--start", type=int, default=0)
    parser.add_argument("--json", default=None, help="JSON 配置文件路径或 JSON 字符串，绕过 shell 转义问题")
    args = parser.parse_args()

    # --json 模式：从 JSON 文件或字符串加载参数，完全绕过 shell 转义
    if args.json is not None:
        import json
        json_text = args.json
        # 先尝试作为文件路径读取
        try:
            with open(json_text, "r", encoding="utf-8") as f:
                json_text = f.read()
        except (FileNotFoundError, OSError):
            pass  # 不是文件路径，当作 JSON 字符串处理
        params = json.loads(json_text)
        args.query = params.get("query", args.query)
        args.max_results = params.get("max", params.get("max_results", args.max_results))
        args.sort = params.get("sort", args.sort)
        args.markdown = args.markdown or params.get("markdown", False)
        args.start = params.get("start", args.start)

    searcher = ArxivSearcher(
        query=args.query,
        max_results=args.max_results,
        sort_by=args.sort,
        start=args.start,
    )

    papers = searcher.fetch()
    if not papers:
        print("未找到匹配论文或请求失败。", file=sys.stderr)
        sys.exit(1)

    # Windows 编码兼容
    if hasattr(sys.stdout, "encoding") and sys.stdout.encoding != "utf-8":
        try:
            sys.stdout.reconfigure(encoding="utf-8", errors="replace")
        except Exception:
            pass

    if args.markdown:
        print_markdown(papers)
    else:
        print_text(papers)


if __name__ == "__main__":
    main()
