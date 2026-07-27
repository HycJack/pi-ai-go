import { useState, useEffect, useMemo } from 'react';
import Editor, { loader } from '@monaco-editor/react';
import { useT } from '../i18n';

// Configure Monaco to load from CDN
loader.config({ paths: { vs: 'https://cdn.jsdelivr.net/npm/monaco-editor@0.52.2/min/vs' } });

interface FilePreviewProps {
  filePath: string;
}

/** Map file extension to Monaco language id */
function guessLanguage(filePath: string): string {
  const ext = filePath.split('.').pop()?.toLowerCase() || '';
  const map: Record<string, string> = {
    ts: 'typescript',
    tsx: 'typescript',
    js: 'javascript',
    jsx: 'javascript',
    mjs: 'javascript',
    cjs: 'javascript',
    json: 'json',
    jsonc: 'json',
    md: 'markdown',
    mdx: 'markdown',
    css: 'css',
    scss: 'scss',
    less: 'less',
    html: 'html',
    htm: 'html',
    xml: 'xml',
    svg: 'xml',
    yaml: 'yaml',
    yml: 'yaml',
    py: 'python',
    rb: 'ruby',
    rs: 'rust',
    go: 'go',
    java: 'java',
    kt: 'kotlin',
    swift: 'swift',
    c: 'c',
    h: 'c',
    cpp: 'cpp',
    hpp: 'cpp',
    cc: 'cpp',
    cxx: 'cpp',
    cs: 'csharp',
    fs: 'fsharp',
    php: 'php',
    sh: 'shell',
    bash: 'shell',
    zsh: 'shell',
    ps1: 'powershell',
    sql: 'sql',
    r: 'r',
    dart: 'dart',
    lua: 'lua',
    clj: 'clojure',
    cljs: 'clojure',
    scala: 'scala',
    erl: 'erlang',
    ex: 'elixir',
    exs: 'elixir',
    vue: 'html',
    svelte: 'html',
    graphql: 'graphql',
    gql: 'graphql',
    toml: 'ini',
    ini: 'ini',
    cfg: 'ini',
    conf: 'ini',
    dockerfile: 'dockerfile',
    makefile: 'makefile',
    mk: 'makefile',
    diff: 'diff',
    patch: 'diff',
    bat: 'bat',
    cmd: 'bat',
    tex: 'latex',
    bib: 'latex',
    txt: 'plaintext',
    log: 'plaintext',
    env: 'ini',
    gitignore: 'plaintext',
  };
  return map[ext] || 'plaintext';
}

export default function FilePreview({ filePath }: FilePreviewProps) {
  const t = useT();
  const [content, setContent] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const language = useMemo(() => guessLanguage(filePath), [filePath]);
  const fileName = useMemo(() => filePath.split('/').pop() || filePath, [filePath]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setContent('');
    setError('');

    const loadFile = async () => {
      try {
        const { ReadTextFile } = await import('../../wailsjs/go/main/App');
        const text = await ReadTextFile(filePath);
        if (!cancelled) {
          setContent(text);
        }
      } catch (e: any) {
        if (!cancelled) {
          setError(String(e?.message || e));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    loadFile();
    return () => { cancelled = true; };
  }, [filePath]);

  if (error) {
    return <div className="file-preview-inline-error">{error}</div>;
  }

  return (
    <div className="file-preview-editor">
      <Editor
        key={filePath}
        language={language}
        value={content}
        theme="vs-dark"
        loading={
          <div className="file-preview-inline-loading">
            <span className="status-spinner" />
            <span>{t('app.loading')}</span>
          </div>
        }
        options={{
          readOnly: true,
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          fontSize: 13,
          lineNumbers: 'on',
          renderLineHighlight: 'line',
          folding: true,
          foldingStrategy: 'indentation',
          autoIndent: 'full',
          formatOnPaste: true,
          tabSize: 2,
          wordWrap: 'off',
          overviewRulerLanes: 0,
          hideCursorInOverviewRuler: true,
          overviewRulerBorder: false,
          smoothScrolling: true,
          cursorBlinking: 'solid',
          cursorStyle: 'line',
          renderWhitespace: 'selection',
          bracketPairColorization: { enabled: true },
          padding: { top: 8 },
        }}
      />
    </div>
  );
}
