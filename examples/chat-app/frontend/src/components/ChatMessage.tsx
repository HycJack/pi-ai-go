import { memo, useState } from 'react';
import { Bot, CheckOutlined, CopyOutlined, UserOutlined } from '../icons';
import MarkdownRenderer from './MarkdownRenderer';
import ThinkingBlock from './ThinkingBlock';
import ToolCallBlock from './ToolCallBlock';
import AgentSteps from './AgentSteps';
import type { ToolCall, ImageAttachment, AgentStep } from '../types';
import { useT } from '../i18n';

interface ChatMessageProps {
  role: 'user' | 'assistant';
  content: string;
  timestamp: string;
  isLoading?: boolean;
  thinking?: string;
  toolCalls?: ToolCall[];
  steps?: AgentStep[];
  images?: ImageAttachment[];
  onSpeak?: () => void;
  onStopSpeak?: () => void;
  isSpeaking?: boolean;
}

function ChatMessageInner({
  role,
  content,
  timestamp,
  isLoading,
  thinking,
  toolCalls,
  steps,
  images,
  onSpeak,
  onStopSpeak,
  isSpeaking,
}: ChatMessageProps) {
  const t = useT();
  const [copied, setCopied] = useState(false);
  const [previewImg, setPreviewImg] = useState<string | null>(null);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(content);
    } catch { /* clipboard might be blocked */ }
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  if (role === 'user') {
    return (
      <div className="msg-row msg-row-user">
        <div className="msg-stack">
          <div className="msg-meta">
            <span className="msg-author">{t('msg.you')}</span>
            <span className="msg-time">{timestamp}</span>
          </div>
          {images && images.length > 0 && (
            <div className="msg-images">
              {images.map((img, idx) => (
                <img
                  key={idx}
                  src={img.data}
                  alt={img.name || `${t('input.altImage')}-${idx}`}
                  className="msg-image-thumb"
                  onClick={() => setPreviewImg(img.data)}
                />
              ))}
            </div>
          )}
          {content && (
            <div className="msg-bubble user">
              <MarkdownRenderer content={content} />
            </div>
          )}
          <div className="msg-actions">
            <button className="ghost-action" onClick={handleCopy}>
              {copied ? <CheckOutlined size={12} /> : <CopyOutlined size={12} />}
              <span>{copied ? t('msg.copied') : t('msg.copy')}</span>
            </button>
          </div>
        </div>
        <div className="msg-avatar user">
          <UserOutlined size={16} />
        </div>
        {previewImg && (
          <div className="msg-image-preview-overlay" onClick={() => setPreviewImg(null)}>
            <img src={previewImg} alt={t('input.altPreview')} className="msg-image-preview" />
          </div>
        )}
      </div>
    );
  }

  return (
    <div className="msg-row msg-row-assistant">
      <div className="msg-avatar assistant">
        <Bot size={18} />
      </div>
      <div className="msg-stack">
        <div className="msg-meta">
          <span className="msg-author">Pi-AI</span>
          <span className="msg-time">{timestamp || (isLoading ? t('msg.generating') : '')}</span>
        </div>
        <div className="msg-bubble assistant">
          {steps && steps.length > 0 ? (
            <AgentSteps steps={steps} isLoading={isLoading} />
          ) : (
            <>
              <ThinkingBlock content={thinking || ''} />
              <ToolCallBlock toolCalls={toolCalls || []} isLoading={isLoading} />
            </>
          )}
          {content ? (
            <MarkdownRenderer content={content} />
          ) : (
            isLoading &&
            !thinking &&
            (!toolCalls || toolCalls.length === 0) &&
            (!steps || steps.length === 0) && (
              <div className="generating-row">
                <span className="status-spinner" />
                <span>{t('msg.thinking')}</span>
              </div>
            )
          )}
        </div>
        {!isLoading && (content || toolCalls) && (
          <div className="msg-actions">
            <button className="ghost-action" onClick={handleCopy}>
              {copied ? <CheckOutlined size={12} /> : <CopyOutlined size={12} />}
              <span>{copied ? t('msg.copied') : t('msg.copy')}</span>
            </button>
            {content && (onSpeak || onStopSpeak) && (
              <button className={`ghost-action ${isSpeaking ? 'is-active' : ''}`} onClick={isSpeaking ? onStopSpeak : onSpeak}>
                <span className="status-dot" style={{ background: isSpeaking ? 'var(--n-error)' : 'var(--n-primary)' }} />
                <span>{isSpeaking ? t('msg.stopSpeak') : t('msg.speak')}</span>
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

export default memo(ChatMessageInner);
