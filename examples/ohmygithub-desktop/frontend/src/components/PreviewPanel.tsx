import React, { useState, useEffect, useMemo } from 'react';
import { API, PullRequest, Issue, Notification, formatRelativeTime } from '../lib/api';
import ReactMarkdown from 'react-markdown';
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

function useIsDarkTheme(): boolean {
  const [isDark, setIsDark] = useState(
    typeof document !== 'undefined' ? document.documentElement.getAttribute('data-theme') !== 'light' : true
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

interface PreviewPanelProps {
  open: boolean;
  item: any;
  onClose: () => void;
}

interface DiffFile {
  filename: string;
  status: string;
  additions: number;
  deletions: number;
  patch: string;
}

function getDiffLanguageExtension(fileName: string): any[] {
  const ext = fileName.split('.').pop()?.toLowerCase() || '';
  const lower = fileName.toLowerCase();
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
    cs: () => [cpp()],
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
    toml: () => [yaml()],
    php: () => [php()],
  };
  return map[ext]?.() || [];
}

interface ParsedPatch {
  removed: string;
  added: string;
  hunks: { header: string; removed: string[]; added: string[] }[];
}

function parsePatch(patch: string): ParsedPatch {
  const lines = patch.split('\n');
  const hunks: ParsedPatch['hunks'] = [];
  const removedLines: string[] = [];
  const addedLines: string[] = [];
  let currentHunk: { header: string; removed: string[]; added: string[] } | null = null;

  for (const line of lines) {
    if (line.startsWith('@@')) {
      currentHunk = { header: line, removed: [], added: [] };
      hunks.push(currentHunk);
      continue;
    }
    if (!currentHunk) continue;
    if (line.startsWith('-')) {
      currentHunk.removed.push(line.slice(1));
      removedLines.push(line.slice(1));
    } else if (line.startsWith('+')) {
      currentHunk.added.push(line.slice(1));
      addedLines.push(line.slice(1));
    } else if (line.startsWith(' ') || line === '') {
      currentHunk.removed.push(line.slice(1));
      currentHunk.added.push(line.slice(1));
    }
  }

  return {
    removed: removedLines.join('\n'),
    added: addedLines.join('\n'),
    hunks,
  };
}

function DiffFileView({ file, viewMode }: { file: DiffFile; viewMode: 'unified' | 'split' }) {
  const isDark = useIsDarkTheme();
  const extensions = useMemo(() => getDiffLanguageExtension(file.filename), [file.filename]);
  const parsed = useMemo(() => parsePatch(file.patch), [file.patch]);

  if (viewMode === 'split' && (parsed.removed.length > 0 || parsed.added.length > 0)) {
    return (
      <div className="code-block" style={{ marginBottom: 8 }}>
        <div className="code-header">
          <span style={{ fontWeight: 500, color: 'var(--text-primary)' }}>{file.filename}</span>
          <span style={{ marginLeft: 8, fontSize: 11 }} className={`status-badge ${file.status === 'added' ? 'success' : file.status === 'removed' ? 'failure' : 'neutral'}`}>{file.status}</span>
          <span style={{ marginLeft: 'auto', color: 'var(--text-success)' }}>+{file.additions}</span>
          <span style={{ color: 'var(--text-danger)' }}>−{file.deletions}</span>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column' }}>
          {parsed.hunks.length > 0 ? parsed.hunks.map((hunk, i) => (
            <div key={i} style={{ borderBottom: i < parsed.hunks.length - 1 ? '1px solid var(--border-subtle)' : undefined }}>
              <div style={{
                padding: '4px 8px', fontSize: 11, fontFamily: 'var(--font-mono)',
                background: 'var(--bg-overlay)', color: 'var(--text-tertiary)',
              }}>
                {hunk.header}
              </div>
              <div style={{ display: 'flex', gap: 0 }}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <CodeMirror
                    value={hunk.removed.join('\n')}
                    extensions={extensions}
                    theme={isDark ? githubDark : githubLight}
                    editable={false}
                    basicSetup={{
                      lineNumbers: false,
                      highlightActiveLine: false,
                      highlightActiveLineGutter: false,
                      foldGutter: false,
                      autocompletion: false,
                      searchKeymap: false,
                    }}
                    style={{
                      fontSize: 11,
                      fontFamily: 'var(--font-mono)',
                      height: 'auto',
                    }}
                  />
                </div>
                <div style={{ width: 4, background: 'var(--border-muted)' }} />
                <div style={{ flex: 1, minWidth: 0 }}>
                  <CodeMirror
                    value={hunk.added.join('\n')}
                    extensions={extensions}
                    theme={isDark ? githubDark : githubLight}
                    editable={false}
                    basicSetup={{
                      lineNumbers: false,
                      highlightActiveLine: false,
                      highlightActiveLineGutter: false,
                      foldGutter: false,
                      autocompletion: false,
                      searchKeymap: false,
                    }}
                    style={{
                      fontSize: 11,
                      fontFamily: 'var(--font-mono)',
                      height: 'auto',
                    }}
                  />
                </div>
              </div>
            </div>
          )) : null}
        </div>
      </div>
    );
  }

  return (
    <div className="code-block" style={{ marginBottom: 8 }}>
      <div className="code-header">
        <span style={{ fontWeight: 500, color: 'var(--text-primary)' }}>{file.filename}</span>
        <span style={{ marginLeft: 8, fontSize: 11 }} className={`status-badge ${file.status === 'added' ? 'success' : file.status === 'removed' ? 'failure' : 'neutral'}`}>{file.status}</span>
        <span style={{ marginLeft: 'auto', color: 'var(--text-success)' }}>+{file.additions}</span>
        <span style={{ color: 'var(--text-danger)' }}>−{file.deletions}</span>
      </div>
      <div className="diff-content">
        {file.patch.split('\n').map((line, i) => {
          let cls = 'diff-line context';
          if (line.startsWith('+')) cls = 'diff-line added';
          else if (line.startsWith('-')) cls = 'diff-line removed';
          else if (line.startsWith('@@')) cls = 'diff-line hunk';
          const prefix = line.charAt(0);
          const content = line.startsWith('@@') ? line : line.slice(1);
          return (
            <div key={i} className={cls}>
              <span className="diff-line-no">{line.startsWith('@@') ? ' ' : prefix === ' ' ? ' ' : prefix}</span>
              <span className="diff-line-content">{content}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export default function PreviewPanel({ open, item, onClose }: PreviewPanelProps) {
  const [diffFiles, setDiffFiles] = useState<DiffFile[]>([]);
  const [rawDiff, setRawDiff] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [viewMode, setViewMode] = useState<'unified' | 'split'>('split');

  const MarkdownPre = React.useCallback(({ children, ...props }: any) => {
    const childArray = React.Children.toArray(children);
    const firstChild = childArray[0];
    if (React.isValidElement(firstChild)) {
      const codeProps: any = firstChild.props || {};
      const className: string = codeProps.className || '';
      const match = /language-mermaid/.exec(className);
      if (match) {
        const code = String(codeProps.children ?? '').replace(/\n$/, '');
        return <MermaidBlock chart={code} />;
      }
    }
    return <pre {...props}>{children}</pre>;
  }, []);

  const renderMarkdownBody = React.useCallback((body: string) => {
    if (!body) return null;
    return (
      <div className="markdown-body" style={{ padding: '8px 0' }}>
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
          {body}
        </ReactMarkdown>
      </div>
    );
  }, [MarkdownPre]);

  useEffect(() => {
    setDiffFiles([]);
    setRawDiff('');
    if (!item) return;

    let cancelled = false;
    if (item._type === 'pr') {
      setLoading(true);
      API.GetPRFiles(item.repo, item.number)
        .then((str: string) => {
          if (cancelled) return;
          try {
            const files = JSON.parse(str) as DiffFile[];
            setDiffFiles(files);
          } catch { /* ignore */ }
        })
        .catch(() => {
          if (cancelled) return;
          if (item._diff) {
            setRawDiff(item._diff as string);
          } else {
            API.GetPRDiff(item.repo, item.number)
              .then((d: string) => { if (!cancelled) setRawDiff(d); })
              .catch(() => { /* ignore */ });
          }
        })
        .finally(() => { if (!cancelled) setLoading(false); });
    } else if (item._type === 'pr-raw' && item._diff) {
      setRawDiff(item._diff as string);
    }
    return () => { cancelled = true; };
  }, [item]);

  if (!open || !item) return null;

  const pr = item as PullRequest;
  const issue = item as Issue;
  const notif = item as Notification;

  const renderDiff = () => {
    if (loading) {
      return <div className="loading-spinner"><div className="spinner" /></div>;
    }
    if (diffFiles.length > 0) {
      const totalAdditions = diffFiles.reduce((s, f) => s + f.additions, 0);
      const totalDeletions = diffFiles.reduce((s, f) => s + f.deletions, 0);
      return (
        <div>
          <div style={{
            marginBottom: 8, fontSize: 12, display: 'flex', alignItems: 'center',
            justifyContent: 'space-between', gap: 12, flexWrap: 'wrap',
          }}>
            <span style={{ color: 'var(--text-secondary)' }}>
              {diffFiles.length} files changed ·{' '}
              <span style={{ color: 'var(--text-success)' }}>+{totalAdditions}</span>{' '}
              <span style={{ color: 'var(--text-danger)' }}>−{totalDeletions}</span>
            </span>
            <div style={{ display: 'flex', gap: 4 }}>
              <button
                className={`btn btn-ghost btn-sm ${viewMode === 'unified' ? 'btn-primary' : ''}`}
                style={{ fontSize: 11, padding: '2px 8px' }}
                onClick={() => setViewMode('unified')}
              >Unified</button>
              <button
                className={`btn btn-ghost btn-sm ${viewMode === 'split' ? 'btn-primary' : ''}`}
                style={{ fontSize: 11, padding: '2px 8px' }}
                onClick={() => setViewMode('split')}
              >Split (code-highlighted)</button>
            </div>
          </div>
          {diffFiles.map((f, i) => (
            <DiffFileView key={i} file={f} viewMode={viewMode} />
          ))}
        </div>
      );
    }
    if (rawDiff) {
      return (
        <div className="code-block">
          <div className="code-header">Raw diff</div>
          <div className="code-content" style={{ maxHeight: 600, overflow: 'auto', whiteSpace: 'pre' }}>
            {rawDiff}
          </div>
        </div>
      );
    }
    return <div className="empty-state" style={{ padding: 16 }}><p>No diff available</p></div>;
  };

  const renderDetail = () => {
    if (item._type === 'pr') {
      return (
        <div>
          <div className="detail-header">
            <div className="detail-title">
              <span className={`state-icon ${pr.draft ? 'draft' : pr.state === 'open' ? 'open' : 'closed'}`} style={{ marginRight: 8 }}>
                <svg viewBox="0 0 16 16" fill="currentColor" width="16" height="16">
                  <path d="M7.177 3.073L9.573.677A.25.25 0 0110 .854v4.792a.25.25 0 01-.427.177L7.177 3.427a.25.25 0 010-.354zM3.75 2.5a.75.75 0 100 1.5.75.75 0 000-1.5zM2.5 3.25a1.25 1.25 0 112.5 0 1.25 1.25 0 01-2.5 0zM3.75 12a.75.75 0 100 1.5.75.75 0 000-1.5zM2.5 12.75a1.25 1.25 0 112.5 0 1.25 1.25 0 01-2.5 0zM10 12a.75.75 0 100 1.5.75.75 0 000-1.5zM8.75 12.75a1.25 1.25 0 112.5 0 1.25 1.25 0 01-2.5 0z"/>
                </svg>
              </span>
              {pr.title}
            </div>
            <div className="detail-meta">
              <span>#{pr.number}</span>
              <span>by {pr.user}</span>
              <span>{formatRelativeTime(pr.createdAt)}</span>
              <span>{pr.repo}</span>
            </div>
          </div>

          {pr.labels.length > 0 && (
            <div style={{ marginBottom: 16, display: 'flex', gap: 4, flexWrap: 'wrap' }}>
              {pr.labels.map((l, i) => (
                <span key={i} className="label" style={{
                  background: `#${l.color}22`,
                  borderColor: `#${l.color}44`,
                  color: `#${l.color}`,
                }}>
                  {l.name}
                </span>
              ))}
            </div>
          )}

          <div style={{ marginBottom: 16, display: 'flex', gap: 8 }}>
            <button className="btn btn-sm" onClick={() => API.OpenExternal(`https://github.com/${pr.repo}/pull/${pr.number}`)}>
              Open on GitHub
            </button>
          </div>

          {pr.body && renderMarkdownBody(pr.body)}

          <h3 style={{ fontSize: 13, fontWeight: 600, marginBottom: 8, color: 'var(--text-secondary)' }}>Changes</h3>
          {renderDiff()}
        </div>
      );
    }

    if (item._type === 'issue') {
      return (
        <div>
          <div className="detail-header">
            <div className="detail-title">
              <span className={`state-icon ${issue.state === 'open' ? 'open' : 'closed'}`} style={{ marginRight: 8 }}>
                <svg viewBox="0 0 16 16" fill="currentColor" width="16" height="16">
                  <path d="M8 9.5a1.5 1.5 0 100-3 1.5 1.5 0 000 3zM8 0a8 8 0 110 16A8 8 0 018 0zM1.5 8a6.5 6.5 0 1013 0 6.5 6.5 0 00-13 0z"/>
                </svg>
              </span>
              {issue.title}
            </div>
            <div className="detail-meta">
              <span>#{issue.number}</span>
              <span>by {issue.user}</span>
              <span>{formatRelativeTime(issue.createdAt)}</span>
              <span>{issue.repo}</span>
            </div>
          </div>
          {issue.labels.length > 0 && (
            <div style={{ marginBottom: 16, display: 'flex', gap: 4, flexWrap: 'wrap' }}>
              {issue.labels.map((l, i) => (
                <span key={i} className="label" style={{
                  background: `#${l.color}22`,
                  borderColor: `#${l.color}44`,
                  color: `#${l.color}`,
                }}>
                  {l.name}
                </span>
              ))}
            </div>
          )}
          <div style={{ marginBottom: 16, display: 'flex', gap: 8 }}>
            <button className="btn btn-sm" onClick={() => API.OpenExternal(`https://github.com/${issue.repo}/issues/${issue.number}`)}>
              Open on GitHub
            </button>
          </div>
          {issue.body && renderMarkdownBody(issue.body)}
        </div>
      );
    }

    if (item._type === 'notification') {
      return (
        <div>
          <div className="detail-header">
            <div className="detail-title">{notif.title}</div>
            <div className="detail-meta">
              <span>{notif.repo}</span>
              <span>{notif.type}</span>
              <span>{formatRelativeTime(notif.updatedAt)}</span>
            </div>
          </div>
          {notif.url && (
            <button className="btn btn-sm" onClick={() => API.OpenExternal(notif.url)}>
              Open on GitHub
            </button>
          )}
        </div>
      );
    }

    return <div className="empty-state"><h3>No details available</h3></div>;
  };

  return (
    <div className="preview-panel">
      <div className="preview-header">
        <button className="btn btn-ghost btn-sm btn-icon" onClick={onClose}>
          <svg viewBox="0 0 24 24" fill="currentColor" width="16" height="16">
            <path d="M15.41 7.41L14 6l-6 6 6 6 1.41-1.41L10.83 12z"/>
          </svg>
        </button>
        <h3>Preview</h3>
      </div>
      <div className="preview-body" style={{ overflow: 'auto' }}>
        {renderDetail()}
      </div>
    </div>
  );
}
