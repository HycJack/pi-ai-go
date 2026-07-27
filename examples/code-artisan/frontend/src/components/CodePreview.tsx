import { useRef, useEffect } from 'react';
import { CopyOutlined, CheckOutlined } from '../icons';
import { useState } from 'react';

interface CodePreviewProps {
  code: string;
  isStreaming: boolean;
}

export default function CodePreview({ code, isStreaming }: CodePreviewProps) {
  const [copied, setCopied] = useState(false);
  const codeRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (isStreaming && codeRef.current) {
      codeRef.current.scrollTop = codeRef.current.scrollHeight;
    }
  }, [code, isStreaming]);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch { /* ignore */ }
  };

  if (!code && !isStreaming) {
    return (
      <aside className="code-preview empty">
        <div className="code-preview-placeholder">
          <div className="placeholder-icon">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" opacity="0.3">
              <polyline points="16 18 22 12 16 6" />
              <polyline points="8 6 2 12 8 18" />
            </svg>
          </div>
          <p>生成的代码将显示在这里</p>
          <p className="placeholder-hint">
            在左侧输入描述，AI 将为你生成 Python matplotlib 绘图代码
          </p>
        </div>
      </aside>
    );
  }

  const lines = code.split('\n');
  const lineCount = lines.length;

  return (
    <aside className={`code-preview ${isStreaming ? 'streaming' : ''}`}>
      <div className="code-preview-header">
        <div className="code-preview-title">
          <span className="code-lang-badge">Python</span>
          {isStreaming && <span className="streaming-indicator">生成中...</span>}
        </div>
        <button className="copy-btn" onClick={handleCopy} title="复制代码">
          {copied ? <CheckOutlined size={14} /> : <CopyOutlined size={14} />}
          <span>{copied ? '已复制' : '复制'}</span>
        </button>
      </div>
      <div className="code-preview-body" ref={codeRef}>
        <div className="code-grid">
          <div className="line-numbers">
            {Array.from({ length: lineCount }, (_, i) => (
              <div key={i} className="line-number">{i + 1}</div>
            ))}
          </div>
          <pre className="code-content"><code>{code}</code></pre>
        </div>
      </div>
    </aside>
  );
}
