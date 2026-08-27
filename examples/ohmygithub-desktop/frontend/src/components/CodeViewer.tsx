import React, { useMemo, useEffect, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import { API } from '../lib/api';
import remarkGfm from 'remark-gfm';
import rehypeKatex from 'rehype-katex';
import rehypeHighlight from 'rehype-highlight';
import CodeMirror from '@uiw/react-codemirror';
import { githubDark, githubLight } from '@uiw/codemirror-theme-github';
import { javascript } from '@codemirror/lang-javascript';
import { python } from '@codemirror/lang-python';
import { go } from '@codemirror/lang-go';
import { rust } from '@codemirror/lang-rust';
import { java } from '@codemirror/lang-java';
import { cpp } from '@codemirror/lang-cpp';
import { css } from '@codemirror/lang-css';
import { html } from '@codemirror/lang-html';
import { json } from '@codemirror/lang-json';
import { sql } from '@codemirror/lang-sql';
import { markdown } from '@codemirror/lang-markdown';
import { yaml } from '@codemirror/lang-yaml';
import { php } from '@codemirror/lang-php';
import MermaidBlock from './MermaidBlock';

interface CodeViewerProps {
  fileName: string;
  content: string;
}

// 根据扩展名返回 CodeMirror 语言扩展
function getLanguageExtension(fileName: string): any[] {
  const ext = fileName.split('.').pop()?.toLowerCase() || '';
  const lower = fileName.toLowerCase();

  // 特殊文件名
  if (lower === 'dockerfile' || lower.startsWith('dockerfile.')) return [];
  if (lower === 'makefile') return [];

  const map: Record<string, () => any[]> = {
    js: () => [javascript({ jsx: false })],
    mjs: () => [javascript({ jsx: false })],
    cjs: () => [javascript({ jsx: false })],
    jsx: () => [javascript({ jsx: true })],
    ts: () => [javascript({ jsx: false, typescript: true })],
    mts: () => [javascript({ jsx: false, typescript: true })],
    cts: () => [javascript({ jsx: false, typescript: true })],
    tsx: () => [javascript({ jsx: true, typescript: true })],
    py: () => [python()],
    go: () => [go()],
    rs: () => [rust()],
    java: () => [java()],
    c: () => [cpp()],
    h: () => [cpp()],
    cpp: () => [cpp()],
    cc: () => [cpp()],
    cxx: () => [cpp()],
    hpp: () => [cpp()],
    hxx: () => [cpp()],
    cs: () => [cpp()], // C# 无独立包，用 cpp 近似
    css: () => [css()],
    scss: () => [css()],
    sass: () => [css()],
    less: () => [css()],
    html: () => [html()],
    htm: () => [html()],
    xml: () => [html()],
    svg: () => [html()],
    vue: () => [html()],
    svelte: () => [html()],
    json: () => [json()],
    jsonc: () => [json()],
    sql: () => [sql()],
    md: () => [markdown()],
    markdown: () => [markdown()],
    mdown: () => [markdown()],
    mkd: () => [markdown()],
    yml: () => [yaml()],
    yaml: () => [yaml()],
    toml: () => [yaml()], // TOML 无独立包，用 yaml 近似
    php: () => [php()],
  };
  return map[ext]?.() || [];
}

// 根据扩展名推断语言名（用于显示标签）
function getLanguageLabel(fileName: string): string {
  const ext = fileName.split('.').pop()?.toLowerCase() || '';
  const map: Record<string, string> = {
    js: 'JavaScript', mjs: 'JavaScript', cjs: 'JavaScript',
    jsx: 'JSX', ts: 'TypeScript', mts: 'TypeScript', cts: 'TypeScript',
    tsx: 'TSX', py: 'Python', go: 'Go', rs: 'Rust', java: 'Java',
    c: 'C', h: 'C', cpp: 'C++', cc: 'C++', cxx: 'C++', hpp: 'C++', hxx: 'C++',
    cs: 'C#', fs: 'F#', swift: 'Swift', kt: 'Kotlin', rb: 'Ruby', php: 'PHP',
    sh: 'Shell', bash: 'Bash', zsh: 'Zsh', fish: 'Fish',
    yml: 'YAML', yaml: 'YAML', toml: 'TOML', ini: 'INI', cfg: 'INI',
    json: 'JSON', jsonc: 'JSON', xml: 'XML', html: 'HTML', htm: 'HTML',
    css: 'CSS', scss: 'SCSS', sass: 'Sass', less: 'Less',
    sql: 'SQL', graphql: 'GraphQL', gql: 'GraphQL',
    md: 'Markdown', markdown: 'Markdown',
    dockerfile: 'Dockerfile', makefile: 'Makefile',
    lua: 'Lua', r: 'R', vue: 'Vue', svelte: 'Svelte',
    mmd: 'Mermaid', mermaid: 'Mermaid',
    csv: 'CSV', tsv: 'TSV',
  };
  return map[ext] || 'Text';
}

function isMarkdown(fileName: string): boolean {
  const ext = fileName.split('.').pop()?.toLowerCase() || '';
  return ['md', 'markdown', 'mdown', 'mkd', 'mdx'].includes(ext);
}

function isMermaid(fileName: string): boolean {
  const ext = fileName.split('.').pop()?.toLowerCase() || '';
  return ['mmd', 'mermaid'].includes(ext);
}

// Markdown 内嵌 pre 组件：拦截 mermaid 代码块用 MermaidBlock 渲染
function MarkdownPre({ children, ...props }: any) {
  const childArray = React.Children.toArray(children);
  const firstChild = childArray[0];
  if (React.isValidElement(firstChild)) {
    const codeProps: any = firstChild.props || {};
    const className: string = codeProps.className || '';
    if (/language-mermaid/.test(className)) {
      const code = String(codeProps.children ?? '').replace(/\n$/, '');
      return <MermaidBlock chart={code} />;
    }
  }
  return <pre {...props}>{children}</pre>;
}

// 监听主题变化，返回当前是否为暗色主题
function useIsDarkTheme(): boolean {
  const [isDark, setIsDark] = useState(
    document.documentElement.getAttribute('data-theme') !== 'light'
  );
  useEffect(() => {
    const observer = new MutationObserver(() => {
      setIsDark(document.documentElement.getAttribute('data-theme') !== 'light');
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-theme'],
    });
    return () => observer.disconnect();
  }, []);
  return isDark;
}

// CodeMirror 代码视图（只读，带行号、折叠、语法高亮）
function CodeMirrorViewer({ fileName, content }: { fileName: string; content: string }) {
  const isDark = useIsDarkTheme();
  const extensions = useMemo(() => getLanguageExtension(fileName), [fileName]);
  const label = getLanguageLabel(fileName);
  const lineCount = content.split('\n').length;

  return (
    <div style={{ padding: 8 }}>
      <div style={{
        fontSize: 11, color: 'var(--text-tertiary)', marginBottom: 8,
        fontFamily: 'var(--font-mono)', display: 'flex', gap: 12,
      }}>
        <span>{label}</span>
        <span>{lineCount} lines</span>
        <span>{content.length < 1024 ? `${content.length} B` : `${(content.length / 1024).toFixed(1)} KB`}</span>
      </div>
      <CodeMirror
        value={content}
        extensions={extensions}
        theme={isDark ? githubDark : githubLight}
        editable={false}
        basicSetup={{
          lineNumbers: true,
          highlightActiveLine: false,
          highlightActiveLineGutter: false,
          foldGutter: true,
          highlightSpecialChars: true,
          autocompletion: false,
          searchKeymap: true,
        }}
        style={{
          fontSize: 12,
          fontFamily: 'var(--font-mono)',
        }}
      />
    </div>
  );
}

// CodeViewer 根据文件类型选择渲染方式：
// - Markdown：react-markdown + GFM + KaTeX + 代码高亮 + mermaid 内嵌
// - Mermaid：直接渲染图表
// - 其它文本：CodeMirror 语法高亮 + 行号 + 折叠
export default function CodeViewer({ fileName, content }: CodeViewerProps) {
  if (isMermaid(fileName)) {
    return (
      <div style={{ padding: 16 }}>
        <MermaidBlock chart={content} />
      </div>
    );
  }

  if (isMarkdown(fileName)) {
    return (
      <div className="markdown-body">
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          rehypePlugins={[rehypeKatex, [rehypeHighlight, { detect: true, ignoreMissing: true }]]}
          components={{
            pre: MarkdownPre,
            a: ({ href, children }) => (
              <a
                href={href}
                onClick={(e) => {
                  e.preventDefault();
                  if (href) API.OpenExternal(href);
                }}
              >
                {children}
              </a>
            ),
          }}
        >
          {content}
        </ReactMarkdown>
      </div>
    );
  }

  return <CodeMirrorViewer fileName={fileName} content={content} />;
}
