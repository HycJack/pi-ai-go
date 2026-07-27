import { useRef, useEffect, useState } from 'react';
import { SendOutlined, StopOutlined, PlayOutlined, DeleteOutlined } from '../icons';

interface ChatAreaProps {
  prompt: string;
  onPromptChange: (val: string) => void;
  generatedCode: string;
  isStreaming: boolean;
  isLoading: boolean;
  onGenerate: () => void;
  onRun: () => void;
  onCancel: () => void;
  onClear: () => void;
  onToggleCodePanel: () => void;
  isCodePanelOpen: boolean;
}

const SUGGESTIONS = [
  '画一个正弦波和余弦波的对比图',
  '画一个3D曲面图，z=sin(sqrt(x²+y²))',
  '用散点图展示随机数据的相关性',
  '画一个柱状图对比不同国家的GDP',
  '画一个饼图展示市场份额占比',
  '画一个热力图展示相关矩阵',
];

export default function ChatArea({
  prompt,
  onPromptChange,
  generatedCode,
  isStreaming,
  isLoading,
  onGenerate,
  onRun,
  onCancel,
  onClear,
  onToggleCodePanel,
  isCodePanelOpen,
}: ChatAreaProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [showSuggestions, setShowSuggestions] = useState(true);

  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = '0px';
    el.style.height = Math.min(el.scrollHeight, 160) + 'px';
  }, [prompt]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      if (prompt.trim() && !isStreaming && !isLoading) {
        onGenerate();
        setShowSuggestions(false);
      }
    }
  };

  const handleSuggestion = (suggestion: string) => {
    onPromptChange(suggestion);
    setShowSuggestions(false);
  };

  const hasCode = generatedCode.trim().length > 0;
  const canGenerate = prompt.trim().length > 0 && !isStreaming && !isLoading;

  return (
    <section className="chat-area">
      <div className="chat-scroll">
        <div className="chat-inner">
          {showSuggestions && !hasCode && (
            <div className="hero-section">
              <div className="hero-mark">
                <CodeOutlined size={32} />
              </div>
              <h1 className="hero-title">用 AI 生成 Python 绘图代码</h1>
              <p className="hero-subtitle">
                描述你想要的图表，AI 将生成完整的 Python matplotlib 代码，
                运行后会在 Qt5 窗口中显示交互图表
              </p>
              <div className="suggestion-grid">
                {SUGGESTIONS.map((s) => (
                  <button
                    key={s}
                    className="suggestion-card"
                    onClick={() => handleSuggestion(s)}
                  >
                    <span className="suggestion-icon">
                      <Sparkles size={16} />
                    </span>
                    <span className="suggestion-text">{s}</span>
                  </button>
                ))}
              </div>
            </div>
          )}

          {hasCode && (
            <div className="code-actions">
              <button
                className="action-btn run"
                onClick={onRun}
                disabled={isLoading}
              >
                <PlayOutlined size={16} />
                <span>运行代码</span>
              </button>
              <button
                className="action-btn secondary"
                onClick={onClear}
              >
                <DeleteOutlined size={16} />
                <span>清除</span>
              </button>
            </div>
          )}
        </div>
      </div>

      <div className="composer">
        <div className="composer-card">
          <textarea
            ref={textareaRef}
            value={prompt}
            onChange={(e) => onPromptChange(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="描述你想要的图表，例如：画一个正弦波..."
            disabled={isStreaming || isLoading}
            rows={1}
            className="composer-input"
          />
          <div className="composer-toolbar">
            <div className="composer-toolbar-left">
              {hasCode && (
                <button
                  className="toolbar-toggle-btn"
                  onClick={onToggleCodePanel}
                  title={isCodePanelOpen ? '收起代码预览' : '展开代码预览'}
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
                    <polyline points={isCodePanelOpen ? "15 18 9 12 15 6" : "9 18 15 12 9 6"} />
                  </svg>
                  <span>{isCodePanelOpen ? '收起' : '代码'}</span>
                </button>
              )}
            </div>
            <div className="composer-toolbar-right">
              {generatedCode && !isStreaming && !isLoading && (
                <button
                  className="send-btn run-btn"
                  onClick={onRun}
                  title="运行代码"
                >
                  <PlayOutlined size={16} />
                </button>
              )}
              {(isStreaming || isLoading) ? (
                <button
                  className="send-btn stop-btn"
                  onClick={onCancel}
                  title="停止"
                >
                  <StopOutlined size={16} />
                </button>
              ) : (
                <button
                  className={`send-btn ${canGenerate ? '' : 'disabled'}`}
                  onClick={onGenerate}
                  disabled={!canGenerate}
                  title="生成代码"
                >
                  <SendOutlined size={16} />
                </button>
              )}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

function CodeOutlined(props: { size: number }) {
  return (
    <svg width={props.size} height={props.size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <polyline points="16 18 22 12 16 6" />
      <polyline points="8 6 2 12 8 18" />
    </svg>
  );
}

function Sparkles(props: { size: number }) {
  return (
    <svg width={props.size} height={props.size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M12 3l1.5 5.5L19 10l-5.5 1.5L12 17l-1.5-5.5L5 10l5.5-1.5z" />
      <path d="M18 14l1 2 2 1-2 1-1 2-1-2-2-1 2-1z" />
    </svg>
  );
}
