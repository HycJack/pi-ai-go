import { memo, useState, useEffect, useRef, useCallback } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { InlineMath, BlockMath } from 'react-katex';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneLight } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { Copy, Check } from 'lucide-react';
import 'katex/dist/katex.min.css';

interface MarkdownRendererProps {
  content: string;
}

// ── Math support ──

function MathNode({ value }: { value: string }) {
  if (value.startsWith('$$') && value.endsWith('$$')) {
    return (
      <div className="math-display">
        <BlockMath math={value.slice(2, -2)} />
      </div>
    );
  }
  return <InlineMath math={value} />;
}

function extractMathContent(text: string): { content: string; isMath: boolean }[] {
  const result: { content: string; isMath: boolean }[] = [];
  const regex = /(\$\$[\s\S]*?\$\$|\$[^$]+\$)/g;
  let lastIndex = 0;
  let match;

  while ((match = regex.exec(text)) !== null) {
    if (match.index > lastIndex) {
      result.push({ content: text.slice(lastIndex, match.index), isMath: false });
    }
    result.push({ content: match[0], isMath: true });
    lastIndex = regex.lastIndex;
  }

  if (lastIndex < text.length) {
    result.push({ content: text.slice(lastIndex), isMath: false });
  }

  return result;
}

// ── Copy button ──

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(text);
    } catch { /* ignore */ }
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }, [text]);

  return (
    <button className="code-copy-btn" onClick={handleCopy} title="复制">
      {copied ? <Check size={12} /> : <Copy size={12} />}
      <span>{copied ? '已复制' : '复制'}</span>
    </button>
  );
}

// ── HTML code block with preview toggle ──

function HtmlBlock({ html, onPreviewHTML }: { html: string; onPreviewHTML?: (html: string) => void }) {
  const [showSource, setShowSource] = useState(false);

  return (
    <div className="code-block">
      <div className="code-block-header">
        <span>html</span>
        <div className="html-block-actions">
          {onPreviewHTML && (
            <button
              className="preview-toggle-btn"
              onClick={() => onPreviewHTML(html)}
            >
              预览课件
            </button>
          )}
          <button
            className={`preview-toggle-btn ${showSource ? 'active' : ''}`}
            onClick={() => setShowSource(!showSource)}
          >
            {showSource ? '预览' : '查看源码'}
          </button>
          <CopyButton text={html} />
        </div>
      </div>
      {showSource ? (
        <SyntaxHighlighter
          style={oneLight}
          language="html"
          PreTag="pre"
          customStyle={{
            margin: 0,
            padding: '1rem',
            background: 'transparent',
            fontSize: '0.875rem',
          }}
        >
          {html}
        </SyntaxHighlighter>
      ) : (
        <iframe
          className="html-preview-iframe"
          sandbox="allow-scripts"
          title="HTML Preview"
          srcDoc={html}
        />
      )}
    </div>
  );
}

// ── MarkdownRenderer ──

export default memo(function MarkdownRenderer({ content }: MarkdownRendererProps) {
  const parts = extractMathContent(content);

  return (
    <div className="markdown-content">
      {parts.map((part, index) =>
        part.isMath ? (
          <MathNode key={index} value={part.content} />
        ) : (
          <ReactMarkdown
            key={index}
            remarkPlugins={[remarkGfm]}
            components={{
              code({ node, className, children, ...props }: any) {
                const match = /language-(\w+)/.exec(className || '');
                const text = String(children).replace(/\n$/, '');

                const parentTag = node?.parent?.tagName;
                const isInParagraph = parentTag === 'p' || parentTag === 'li' || parentTag === 'td' || parentTag === 'th';
                const isShortNoNewline = !text.includes('\n') && text.length < 80;

                // Inline code
                if ((!match && isInParagraph) || (!match && isShortNoNewline)) {
                  return (
                    <code className="md-inline-code" {...props}>
                      {children}
                    </code>
                  );
                }

                const language = match ? match[1] : 'text';

                // HTML with preview
                if (language === 'html') {
                  return <HtmlBlock html={text} />;
                }

                // Regular code block
                return (
                  <div className="code-block">
                    <div className="code-block-header">
                      <span>{language}</span>
                      <CopyButton text={text} />
                    </div>
                    <SyntaxHighlighter
                      style={oneLight}
                      language={language}
                      PreTag="pre"
                      customStyle={{
                        margin: 0,
                        padding: '1rem',
                        background: 'transparent',
                        fontSize: '0.875rem',
                        borderBottomLeftRadius: 'var(--radius-md)',
                        borderBottomRightRadius: 'var(--radius-md)',
                      }}
                    >
                      {text}
                    </SyntaxHighlighter>
                  </div>
                );
              },
              hr() { return <hr />; },
              table({ children }: any) {
                return <div className="md-table-wrap"><table>{children}</table></div>;
              },
              th({ children }: any) { return <th>{children}</th>; },
              td({ children }: any) { return <td>{children}</td>; },
              blockquote({ children }: any) { return <blockquote>{children}</blockquote>; },
              a({ href, children }: any) {
                return <a href={href} target="_blank" rel="noopener noreferrer">{children}</a>;
              },
              p({ children }: any) { return <p>{children}</p>; },
              ul({ children }: any) { return <ul>{children}</ul>; },
              ol({ children }: any) { return <ol>{children}</ol>; },
              li({ children }: any) { return <li>{children}</li>; },
              h1({ children }: any) { return <h1>{children}</h1>; },
              h2({ children }: any) { return <h2>{children}</h2>; },
              h3({ children }: any) { return <h3>{children}</h3>; },
              h4({ children }: any) { return <h4>{children}</h4>; },
            }}
          >
            {part.content}
          </ReactMarkdown>
        ),
      )}
    </div>
  );
});
