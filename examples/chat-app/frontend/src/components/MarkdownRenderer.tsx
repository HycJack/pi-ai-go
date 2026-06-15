import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { InlineMath, BlockMath } from 'react-katex';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';

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

export default function MarkdownRenderer({ content }: MarkdownRendererProps) {
  const parts = extractMathContent(content);

  return (
    <div className="markdown-content prose prose-invert max-w-none prose-p:my-1 prose-headings:my-2 prose-ul:my-1 prose-ol:my-1 prose-li:my-0.5 prose-blockquote:my-2 prose-pre:my-2 prose-code:my-0 prose-hr:my-3">
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
                const isInline = !match && !node?.tagName;
                
                if (isInline) {
                  return (
                    <code className="bg-slate-700 px-1.5 py-0.5 rounded text-sm" {...props}>
                      {children}
                    </code>
                  );
                }
                
                const language = match ? match[1] : 'text';
                return (
                  <pre className="bg-slate-800 rounded-lg overflow-x-auto p-0">
                    <SyntaxHighlighter
                      style={vscDarkPlus}
                      language={language}
                      PreTag="div"
                      customStyle={{
                        margin: 0,
                        padding: '1rem',
                        background: 'rgb(30 41 59)',
                        fontSize: '0.875rem',
                      }}
                    >
                      {String(children).replace(/\n$/, '')}
                    </SyntaxHighlighter>
                  </pre>
                );
              },
              hr({ children }: any) {
                return (
                  <hr className="border-t border-slate-600 my-3" />
                );
              },
              table({ children }: any) {
                return (
                  <div className="overflow-x-auto rounded-lg border border-slate-700 my-2">
                    <table className="w-full">
                      {children}
                    </table>
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
                  <td className="px-4 py-2 border-b border-slate-700">
                    {children}
                  </td>
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
                  <a
                    href={href}
                    className="text-blue-400 hover:text-blue-300 underline"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    {children}
                  </a>
                );
              },
              p({ children }: any) {
                return (
                  <p className="my-1">{children}</p>
                );
              },
              ul({ children }: any) {
                return (
                  <ul className="my-1 pl-6 list-disc">{children}</ul>
                );
              },
              ol({ children }: any) {
                return (
                  <ol className="my-1 pl-6 list-decimal">{children}</ol>
                );
              },
              li({ children }: any) {
                return (
                  <li className="my-0.5">{children}</li>
                );
              },
              h1({ children }: any) {
                return <h1 className="text-2xl font-bold my-2">{children}</h1>;
              },
              h2({ children }: any) {
                return <h2 className="text-xl font-bold my-2">{children}</h2>;
              },
              h3({ children }: any) {
                return <h3 className="text-lg font-bold my-2">{children}</h3>;
              },
              h4({ children }: any) {
                return <h4 className="text-base font-bold my-1">{children}</h4>;
              },
            }}
          >
            {part.content}
          </ReactMarkdown>
        )
      )}
    </div>
  );
}
