import { memo, useState, useEffect, useRef, useCallback } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { InlineMath, BlockMath } from 'react-katex';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneLight } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { CheckOutlined, CopyOutlined } from '../icons';
import { useT } from '../i18n';

interface MarkdownRendererProps {
  content: string;
}

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

// ── Copy button for code blocks ──
function CopyButton({ text }: { text: string }) {
  const t = useT();
  const [copied, setCopied] = useState(false);
  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(text);
    } catch { /* clipboard might be blocked */ }
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }, [text]);

  return (
    <button className="code-copy-btn" onClick={handleCopy} title={t('code.copy')}>
      {copied ? <CheckOutlined size={12} /> : <CopyOutlined size={12} />}
      <span>{copied ? t('code.copied') : t('code.copy')}</span>
    </button>
  );
}

// ── HTML code / preview component ──
function HtmlBlock({ html }: { html: string }) {
  const t = useT();
  const [showPreview, setShowPreview] = useState(true);

  return (
    <div className="code-block">
      <div className="code-block-header">
        <span>html</span>
        <div className="html-block-actions">
          <button
            className={`preview-toggle-btn ${showPreview ? 'active' : ''}`}
            onClick={() => setShowPreview((value) => !value)}
          >
            {showPreview ? t('code.viewSource') : t('code.viewPreview')}
          </button>
          <CopyButton text={html} />
        </div>
      </div>
      {showPreview ? (
        <iframe
          className="html-preview-iframe"
          sandbox="allow-scripts"
          title={t('code.htmlPreview')}
          srcDoc={html}
        />
      ) : (
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
      )}
    </div>
  );
}

// ── Mermaid diagram component ──
function MermaidDiagram({ chart }: { chart: string }) {
  const t = useT();
  const containerRef = useRef<HTMLDivElement>(null);
  const [svg, setSvg] = useState<string>('');
  const [error, setError] = useState<string>('');
  const [expanded, setExpanded] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const mermaid = (await import('mermaid')).default;
        mermaid.initialize({ startOnLoad: false, theme: 'default' });
        const id = `mermaid-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
        const { svg: rendered } = await mermaid.render(id, chart);
        if (!cancelled) {
          setSvg(rendered);
          setError('');
        }
      } catch (e: any) {
        if (!cancelled) {
          setError(e?.message ?? String(e));
          setSvg('');
        }
      }
    })();
    return () => { cancelled = true; };
  }, [chart]);

  if (error) {
    return (
      <div className="mermaid-error">
        <div className="mermaid-error-header">{t('code.mermaidError')}</div>
        <pre>{chart}</pre>
        <div className="mermaid-error-msg">{error}</div>
      </div>
    );
  }

  if (svg) {
    return (
      <>
        <div
          ref={containerRef}
          className="mermaid-container has-preview"
          onClick={() => setExpanded(true)}
          title={t('code.clickToExpand')}
          dangerouslySetInnerHTML={{ __html: svg }}
        />
        {expanded && (
          <div className="mermaid-preview-overlay" onClick={() => setExpanded(false)}>
            <div
              className="mermaid-preview-content"
              onClick={(event) => event.stopPropagation()}
              dangerouslySetInnerHTML={{ __html: svg }}
            />
          </div>
        )}
      </>
    );
  }

  return (
    <div ref={containerRef} className="mermaid-container">
      <div className="mermaid-loading">{t('code.mermaidLoading')}</div>
    </div>
  );
}

export default memo(function MarkdownRenderer({ content }: MarkdownRendererProps) {
  return <MarkdownRendererInner content={content} />;
});

function MarkdownRendererInner({ content }: MarkdownRendererProps) {
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

                if ((!match && isInParagraph) || (!match && isShortNoNewline)) {
                  return (
                    <code className="md-inline-code" {...props}>
                      {children}
                    </code>
                  );
                }

                if (!match && !isInParagraph && text.includes('```')) {
                  return (
                    <div className="code-block">
                      <div className="code-block-header">
                        <span>text</span>
                        <CopyButton text={text} />
                      </div>
                      <pre className="code-block-raw">
                        <code>{text}</code>
                      </pre>
                    </div>
                  );
                }

                const language = match ? match[1] : 'text';

                // Mermaid diagram
                if (language === 'mermaid') {
                  return (
                    <div className="code-block">
                      <div className="code-block-header">
                        <span>mermaid</span>
                        <CopyButton text={text} />
                      </div>
                      <MermaidDiagram chart={text} />
                    </div>
                  );
                }

                // HTML preview / source (mutually exclusive)
                if (language === 'html') {
                  return <HtmlBlock html={text} />;
                }

                // Regular code block with copy button
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
}
