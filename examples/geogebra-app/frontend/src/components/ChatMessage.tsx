import { useState } from 'react';
import { Triangle, Copy, Check, Download, Play, Eye } from 'lucide-react';
import MarkdownRenderer from './MarkdownRenderer';

interface GeogebraResult {
  text: string;
  ggbCode: string;
  html: string;
  svg: string;
}

interface ChatMessageProps {
  role: 'user' | 'assistant';
  content: string;
  timestamp?: string;
  isLoading?: boolean;
  html?: string;
  result?: GeogebraResult;
  onExecuteGGB?: (code: string) => void;
  onOpenHTMLPreview?: (html: string, ggbCode: string, svg: string) => void;
}

function saveFile(content: string, ext: string) {
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `geogebra-${Date.now()}.${ext}`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

export default function ChatMessage({
  role,
  content,
  timestamp,
  isLoading,
  html,
  result,
  onExecuteGGB,
  onOpenHTMLPreview,
}: ChatMessageProps) {
  const [copiedText, setCopiedText] = useState<string | null>(null);

  const copy = (text: string, key: string) => {
    navigator.clipboard.writeText(text).catch(() => {});
    setCopiedText(key);
    setTimeout(() => setCopiedText(null), 1500);
  };

  const effectiveHtml = html || result?.html;
  const effectiveGgb = result?.ggbCode;
  const effectiveSvg = result?.svg;

  // Open preview in right panel
  const openPreview = () => {
    if (effectiveSvg) {
      onOpenHTMLPreview?.(effectiveHtml || '', effectiveGgb || '', effectiveSvg);
    } else if (effectiveHtml) {
      onOpenHTMLPreview?.(effectiveHtml, effectiveGgb || '', '');
    } else if (effectiveGgb) {
      onExecuteGGB?.(effectiveGgb);
    }
  };

  if (role === 'user') {
    return (
      <div className="message user">
        <div className="message-header">
          <span className="user-label">你</span>
          {timestamp && <span className="message-time">{timestamp}</span>}
        </div>
        <div className="message-content">
          <MarkdownRenderer content={content} />
        </div>
      </div>
    );
  }

  return (
    <div className="message assistant">
      <div className="message-header">
        <span className="assistant-label">
          <Triangle size={12} style={{ marginRight: 4, verticalAlign: 'middle' }} />
          GeoGebra 助手
        </span>
        {timestamp && <span className="message-time">{timestamp}</span>}
        {isLoading && (
          <span className="loading-indicator">
            <span className="loading-dot" /><span className="loading-dot" /><span className="loading-dot" />
          </span>
        )}
      </div>

      {content && (
        <div className="message-content">
          <MarkdownRenderer content={content} />
        </div>
      )}

      {isLoading && !content && (
        <div className="message-content thinking-placeholder">生成中...</div>
      )}

      {/* GeoGebra commands block */}
      {effectiveGgb && (
        <div className="code-block">
          <div className="code-block-header">
            <span className="code-block-label">GeoGebra 命令</span>
            <div className="code-block-header-actions">
              <button
                className="code-exec-btn"
                onClick={() => onExecuteGGB?.(effectiveGgb)}
                title="在右侧面板的 GeoGebra 中执行"
              >
                <Play size={12} />
                执行
              </button>
              <button className="code-copy-btn" onClick={() => copy(effectiveGgb, 'ggb')}>
                {copiedText === 'ggb' ? <Check size={12} /> : <Copy size={12} />}
                {copiedText === 'ggb' ? '已复制' : '复制'}
              </button>
            </div>
          </div>
          <pre className="code-block-content">{effectiveGgb}</pre>
        </div>
      )}

      {/* HTML/SVG 课件 — 块状按钮（点击在右侧面板打开） */}
      {(effectiveHtml || effectiveSvg) && (
        <div className="geogebra-block">
          <button className="geogebra-block-btn" onClick={openPreview}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
            </svg>
            <span className="geogebra-block-text">
              {effectiveSvg ? 'SVG 图形' : 'GeoGebra 课件'}
            </span>
            <span className="geogebra-block-badge">{effectiveSvg ? 'SVG' : 'HTML'}</span>
            <Eye size={16} />
          </button>

          <div className="geogebra-block-actions">
            {(effectiveHtml || effectiveSvg) && (
              <button
                className="geogebra-action-btn"
                onClick={() => {
                  const content = effectiveSvg || effectiveHtml;
                  if (content) saveFile(content, effectiveSvg ? 'svg' : 'html');
                }}
                title="保存到本地"
              >
                <Download size={14} />
                保存
              </button>
            )}
            {(effectiveHtml || effectiveSvg) && (
              <button
                className="geogebra-action-btn"
                onClick={() => {
                  const content = effectiveSvg || effectiveHtml;
                  if (content) copy(content, 'doc');
                }}
              >
                {copiedText === 'doc' ? <Check size={14} /> : <Copy size={14} />}
                {copiedText === 'doc' ? '已复制' : '复制'}
              </button>
            )}
            {effectiveGgb && (
              <button
                className="geogebra-action-btn"
                onClick={() => onExecuteGGB?.(effectiveGgb)}
              >
                <Play size={14} />
                在 GeoGebra 中打开
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
