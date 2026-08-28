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
import { Button } from './ui/button';
import { ScrollArea } from './ui/scroll-area';
import { Badge } from './ui/badge';
import {
  ChevronLeft,
  ChevronRight,
  X,
  GitPullRequest,
  CircleDot,
  ExternalLink,
  Loader2,
} from 'lucide-react';

function useIsDarkTheme(): boolean {
  const [isDark, setIsDark] = useState(
    typeof document !== 'undefined' ? !document.documentElement.classList.contains('light') : true
  );
  useEffect(() => {
    const observer = new MutationObserver(() => {
      setIsDark(!document.documentElement.classList.contains('light'));
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class'],
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
      <div className="mb-2 rounded-md border border-border overflow-hidden">
        <div className="flex items-center justify-between bg-muted px-3 py-2 border-b border-border">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium">{file.filename}</span>
            <Badge
              variant={file.status === 'added' ? 'success' : file.status === 'removed' ? 'destructive' : 'secondary'}
              className="text-xs"
            >
              {file.status}
            </Badge>
          </div>
          <div className="flex items-center gap-2 text-xs">
            <span className="text-success">+{file.additions}</span>
            <span className="text-destructive">−{file.deletions}</span>
          </div>
        </div>
        <div className="flex flex-col">
          {parsed.hunks.length > 0
            ? parsed.hunks.map((hunk, i) => (
                <div key={i} className={i < parsed.hunks.length - 1 ? 'border-b border-border' : undefined}>
                  <div className="bg-muted/50 px-2 py-1 text-xs font-mono text-primary/70">
                    {hunk.header}
                  </div>
                  <div className="flex gap-0">
                    <div className="flex-1 min-w-0">
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
                        style={{ fontSize: 11, fontFamily: 'var(--font-mono)', height: 'auto' }}
                      />
                    </div>
                    <div className="w-1 bg-border" />
                    <div className="flex-1 min-w-0">
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
                        style={{ fontSize: 11, fontFamily: 'var(--font-mono)', height: 'auto' }}
                      />
                    </div>
                  </div>
                </div>
              ))
            : null}
        </div>
      </div>
    );
  }

  return (
    <div className="mb-2 rounded-md border border-border overflow-hidden">
      <div className="flex items-center justify-between bg-muted px-3 py-2 border-b border-border">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium">{file.filename}</span>
          <Badge
            variant={file.status === 'added' ? 'success' : file.status === 'removed' ? 'destructive' : 'secondary'}
            className="text-xs"
          >
            {file.status}
          </Badge>
        </div>
        <div className="flex items-center gap-2 text-xs">
          <span className="text-success">+{file.additions}</span>
          <span className="text-destructive">−{file.deletions}</span>
        </div>
      </div>
      <div className="overflow-x-auto font-mono text-xs">
        {file.patch.split('\n').map((line, i) => {
          let cls = 'px-2 py-0.5 text-secondary';
          if (line.startsWith('+')) cls = 'px-2 py-0.5 bg-success/10 text-success';
          else if (line.startsWith('-')) cls = 'px-2 py-0.5 bg-destructive/10 text-destructive';
          else if (line.startsWith('@@')) cls = 'px-2 py-0.5 bg-primary/8 text-primary font-medium';
          const prefix = line.charAt(0);
          const content = line.startsWith('@@') ? line : line.slice(1);
          return (
            <div key={i} className={`flex whitespace-pre ${cls}`}>
              <span className="w-3.5 shrink-0 text-right text-muted-foreground select-none">
                {line.startsWith('@@') ? ' ' : prefix === ' ' ? ' ' : prefix}
              </span>
              <span className="flex-1 break-all">{content}</span>
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
      <div className="markdown-body py-2">
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
          } catch {
            /* ignore */
          }
        })
        .catch(() => {
          if (cancelled) return;
          if (item._diff) {
            setRawDiff(item._diff as string);
          } else {
            API.GetPRDiff(item.repo, item.number)
              .then((d: string) => {
                if (!cancelled) setRawDiff(d);
              })
              .catch(() => {
                /* ignore */
              });
          }
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
    } else if (item._type === 'pr-raw' && item._diff) {
      setRawDiff(item._diff as string);
    }
    return () => {
      cancelled = true;
    };
  }, [item]);

  if (!open || !item) return null;

  const pr = item as PullRequest;
  const issue = item as Issue;
  const notif = item as Notification;

  const renderDiff = () => {
    if (loading) {
      return (
        <div className="flex items-center justify-center py-8">
          <Loader2 className="h-6 w-6 animate-spin text-primary" />
        </div>
      );
    }
    if (diffFiles.length > 0) {
      const totalAdditions = diffFiles.reduce((s, f) => s + f.additions, 0);
      const totalDeletions = diffFiles.reduce((s, f) => s + f.deletions, 0);
      return (
        <div>
          <div className="mb-2 flex flex-wrap items-center justify-between gap-3 text-xs">
            <span className="text-muted-foreground">
              {diffFiles.length} files changed ·{' '}
              <span className="text-success">+{totalAdditions}</span>{' '}
              <span className="text-destructive">−{totalDeletions}</span>
            </span>
            <div className="flex gap-1">
              <Button
                variant={viewMode === 'unified' ? 'default' : 'ghost'}
                size="sm"
                className="h-6 text-xs"
                onClick={() => setViewMode('unified')}
              >
                Unified
              </Button>
              <Button
                variant={viewMode === 'split' ? 'default' : 'ghost'}
                size="sm"
                className="h-6 text-xs"
                onClick={() => setViewMode('split')}
              >
                Split (code-highlighted)
              </Button>
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
        <div className="rounded-md border border-border overflow-hidden">
          <div className="bg-muted px-3 py-2 text-sm font-medium border-b border-border">
            Raw diff
          </div>
          <div className="max-h-[600px] overflow-auto whitespace-pre p-3 font-mono text-xs">
            {rawDiff}
          </div>
        </div>
      );
    }
    return (
      <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
        <p>No diff available</p>
      </div>
    );
  };

  const renderDetail = () => {
    if (item._type === 'pr') {
      return (
        <div>
          <div className="mb-4 space-y-2">
            <div className="text-lg font-semibold leading-tight">
              <GitPullRequest
                className={`mr-2 inline h-4 w-4 ${
                  pr.draft ? 'text-muted-foreground' : pr.state === 'open' ? 'text-success' : 'text-destructive'
                }`}
              />
              {pr.title}
            </div>
            <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
              <span>#{pr.number}</span>
              <span>by {pr.user}</span>
              <span>{formatRelativeTime(pr.createdAt)}</span>
              <span>{pr.repo}</span>
            </div>
          </div>

          {pr.labels.length > 0 && (
            <div className="mb-4 flex flex-wrap gap-1">
              {pr.labels.map((l, i) => (
                <Badge
                  key={i}
                  variant="outline"
                  className="text-xs"
                  style={{
                    background: `#${l.color}22`,
                    borderColor: `#${l.color}44`,
                    color: `#${l.color}`,
                  }}
                >
                  {l.name}
                </Badge>
              ))}
            </div>
          )}

          <div className="mb-4">
            <Button
              variant="outline"
              size="sm"
              onClick={() => API.OpenExternal(`https://github.com/${pr.repo}/pull/${pr.number}`)}
            >
              <ExternalLink className="h-3.5 w-3.5" />
              Open on GitHub
            </Button>
          </div>

          {pr.body && renderMarkdownBody(pr.body)}

          <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Changes
          </h3>
          {renderDiff()}
        </div>
      );
    }

    if (item._type === 'issue') {
      return (
        <div>
          <div className="mb-4 space-y-2">
            <div className="text-lg font-semibold leading-tight">
              <CircleDot
                className={`mr-2 inline h-4 w-4 ${
                  issue.state === 'open' ? 'text-success' : 'text-destructive'
                }`}
              />
              {issue.title}
            </div>
            <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
              <span>#{issue.number}</span>
              <span>by {issue.user}</span>
              <span>{formatRelativeTime(issue.createdAt)}</span>
              <span>{issue.repo}</span>
            </div>
          </div>
          {issue.labels.length > 0 && (
            <div className="mb-4 flex flex-wrap gap-1">
              {issue.labels.map((l, i) => (
                <Badge
                  key={i}
                  variant="outline"
                  className="text-xs"
                  style={{
                    background: `#${l.color}22`,
                    borderColor: `#${l.color}44`,
                    color: `#${l.color}`,
                  }}
                >
                  {l.name}
                </Badge>
              ))}
            </div>
          )}
          <div className="mb-4">
            <Button
              variant="outline"
              size="sm"
              onClick={() => API.OpenExternal(`https://github.com/${issue.repo}/issues/${issue.number}`)}
            >
              <ExternalLink className="h-3.5 w-3.5" />
              Open on GitHub
            </Button>
          </div>
          {issue.body && renderMarkdownBody(issue.body)}
        </div>
      );
    }

    if (item._type === 'notification') {
      return (
        <div>
          <div className="mb-4 space-y-2">
            <div className="text-lg font-semibold leading-tight">{notif.title}</div>
            <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
              <span>{notif.repo}</span>
              <span>{notif.type}</span>
              <span>{formatRelativeTime(notif.updatedAt)}</span>
            </div>
          </div>
          {notif.url && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => API.OpenExternal(notif.url)}
            >
              <ExternalLink className="h-3.5 w-3.5" />
              Open on GitHub
            </Button>
          )}
        </div>
      );
    }

    return (
      <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
        <h3 className="text-base font-semibold">No details available</h3>
      </div>
    );
  };

  return (
    <div className="flex h-full w-[380px] min-w-[380px] flex-col border-l border-border bg-background overflow-hidden">
      <div className="flex h-12 items-center gap-2 border-b border-border px-4 shrink-0">
        <Button variant="ghost" size="icon" className="h-8 w-8" onClick={onClose}>
          <ChevronLeft className="h-4 w-4" />
        </Button>
        <h3 className="flex-1 text-sm font-semibold truncate">Preview</h3>
      </div>
      <ScrollArea className="flex-1 p-4">
        {renderDetail()}
      </ScrollArea>
    </div>
  );
}
