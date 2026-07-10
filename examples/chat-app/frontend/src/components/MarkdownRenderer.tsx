import { memo, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { InlineMath, BlockMath } from 'react-katex';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { Copy, Check } from 'lucide-react';

interface MarkdownRendererProps {
  content: string;
}

function MathNode({ value }: { value: string }) {
  if (value.startsWith('$$') && value.endsWith('$$')) {
    return (
      <div className="math-display my-2">
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

function CodeBlock({ language, value }: { language: string; value: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = () => {
    navigator.clipboard.writeText(value);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };
  return (
    <div className="code-block">
      <div className="code-block-header">
        <span className="code-block-lang">{language}</span>
        <button
          onClick={handleCopy}
          className="code-block-copy"
          title="复制代码"
        >
          {copied ? (
            <><Check className="w-3.5 h-3.5 text-green-400" /><span className="text-green-400">已复制</span></>
          ) : (
            <><Copy className="w-3.5 h-3.5" /><span>复制</span></>
          )}
        </button>
      </div>
      <SyntaxHighlighter
        style={vscDarkPlus}
        language={language}
        PreTag="pre"
        customStyle={{
          margin: 0,
          padding: '1rem',
          background: 'rgb(2 6 23)',
          fontSize: '0.8125rem',
          borderBottomLeftRadius: '0.5rem',
          borderBottomRightRadius: '0.5rem',
        }}
      >
        {value}
      </SyntaxHighlighter>
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

                // 判断行内 vs 代码块（与 ref 项目一致）
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
                    <pre className="code-block code-block-raw">
                      <code>{text}</code>
                    </pre>
                  );
                }

                const language = match ? match[1] : 'text';
                return (
                  <CodeBlock language={language} value={text} />
                );
              },
              hr() {
                return <hr className="border-t border-slate-700 my-3" />;
              },
              table({ children }: any) {
                return (
                  <div className="overflow-x-auto rounded-lg border border-slate-700 my-2">
                    <table className="w-full">{children}</table>
                  </div>
                );
              },
              th({ children }: any) {
                return (
                  <th className="bg-slate-800 px-4 py-2 text-left font-semibold border-b border-slate-700">
                    {children}
                  </th>
                );
              },
              td({ children }: any) {
                return (
                  <td className="px-4 py-2 border-b border-slate-700">{children}</td>
                );
              },
              blockquote({ children }: any) {
                return (
                  <blockquote className="border-l-4 border-blue-500 pl-4 italic text-slate-400 my-2">
                    {children}
                  </blockquote>
                );
              },
              a({ href, children }: any) {
                return (
                  <a href={href} className="text-blue-400 hover:text-blue-300 underline" target="_blank" rel="noopener noreferrer">
                    {children}
                  </a>
                );
              },
            }}
          >
            {part.content}
          </ReactMarkdown>
        ),
      )}
    </div>
  );
}
