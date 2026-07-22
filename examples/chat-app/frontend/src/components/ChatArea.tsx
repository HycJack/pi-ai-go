import { useCallback, useEffect, useRef, useState } from 'react';
import { Bot, ChevronDownOutlined, CodeOutlined, WrenchOutlined, Sparkles } from '../icons';
import ChatInput from './ChatInput';
import ChatMessage from './ChatMessage';
import type { Message, ImageAttachment } from '../types';
import { useT } from '../i18n';

interface ModelInfo {
  id: string;
  name: string;
  reasoning?: boolean;
  thinkingLevelMap?: Record<string, string>;
}

interface ChatAreaProps {
  messages: Message[];
  isLoading: boolean;
  workingDir: string;
  onSendMessage: (message: string, model?: string, thinkingLevel?: string, images?: ImageAttachment[]) => void;
  onStop: () => void;
  onSpeak: (text: string, messageId: string) => void;
  onStopSpeak: () => void;
  speakingMessageId: string | null;
  models?: ModelInfo[];
  currentModel?: string;
  currentThinkingLevel?: string;
  onModelChange?: (model: string) => void;
  onThinkingLevelChange?: (level: string) => void;
  onCaptureScreen?: () => Promise<string | null>;
}

const SUGGESTION_KEYS = [
  'suggestion.quantum',
  'suggestion.poem',
  'suggestion.coding',
  'suggestion.trip',
  'suggestion.learn',
  'suggestion.ideas',
];

const NEAR_BOTTOM_PX = 120;

function scrollToBottom(container: HTMLDivElement | null, end: HTMLDivElement | null) {
  if (!container || !end) return;
  container.scrollTop = container.scrollHeight;
  end.scrollIntoView({ block: 'end' });
}

export default function ChatArea({
  messages,
  isLoading,
  workingDir,
  onSendMessage,
  onStop,
  onSpeak,
  onStopSpeak,
  speakingMessageId,
  models,
  currentModel,
  currentThinkingLevel,
  onModelChange,
  onThinkingLevelChange,
  onCaptureScreen,
}: ChatAreaProps) {
  const t = useT();
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [stuckToBottom, setStuckToBottom] = useState(true);
  const [pendingCount, setPendingCount] = useState(0);
  const lastMsgCountRef = useRef(0);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const onScroll = () => {
      const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
      const near = distanceFromBottom < NEAR_BOTTOM_PX;
      setStuckToBottom(near);
      if (near) setPendingCount(0);
    };
    el.addEventListener('scroll', onScroll, { passive: true });
    return () => el.removeEventListener('scroll', onScroll);
  }, []);

  const conversationKey = messages[0]?.id ?? null;
  useEffect(() => {
    lastMsgCountRef.current = messages.length;
    setPendingCount(0);
    setStuckToBottom(true);
    requestAnimationFrame(() => {
      scrollToBottom(containerRef.current, messagesEndRef.current);
    });
  }, [conversationKey]);

  useEffect(() => {
    const prevCount = lastMsgCountRef.current;
    const newCount = messages.length;
    lastMsgCountRef.current = newCount;
    if (newCount > prevCount) {
      setStuckToBottom(true);
      requestAnimationFrame(() => {
        scrollToBottom(containerRef.current, messagesEndRef.current);
      });
      return;
    }
    if (!stuckToBottom) {
      setPendingCount((c) => c + 1);
      return;
    }
    requestAnimationFrame(() => {
      const el = messagesEndRef.current;
      const container = containerRef.current;
      if (!el || !container) return;
      el.scrollIntoView({ behavior: 'smooth', block: 'end' });
    });
  }, [messages, stuckToBottom]);

  const jumpToBottom = useCallback(() => {
    setStuckToBottom(true);
    setPendingCount(0);
    requestAnimationFrame(() => {
      scrollToBottom(containerRef.current, messagesEndRef.current);
    });
  }, []);

  const handleLocalSend = useCallback(
    (message: string, modelOverride?: string, thinkingLevel?: string, images?: ImageAttachment[]) => {
      onSendMessage(message, modelOverride, thinkingLevel, images);
      requestAnimationFrame(() => {
        scrollToBottom(containerRef.current, messagesEndRef.current);
      });
    },
    [onSendMessage],
  );

  if (messages.length === 0) {
    return (
      <section className="stage stage-empty">
        <div className="stage-inner">
          <div className="hero-mark">
            <Bot size={32} />
          </div>
          <h1 className="hero-title">{t('empty.heroTitle')}</h1>
          <p className="hero-subtitle">
            {t('empty.heroSubtitle')}
          </p>

          <div className="suggestion-grid">
            {SUGGESTION_KEYS.map((key) => (
              <button key={key} className="suggestion-card" onClick={() => handleLocalSend(t(key))}>
                <span className="suggestion-icon">
                  <Sparkles size={16} />
                </span>
                <span className="suggestion-text">{t(key)}</span>
              </button>
            ))}
          </div>
        </div>
        <ChatInput
          onSend={handleLocalSend}
          onStop={onStop}
          disabled={isLoading}
          placeholder={t('input.placeholder')}
          models={models}
          currentModel={currentModel}
          currentThinkingLevel={currentThinkingLevel}
          onModelChange={onModelChange}
          onThinkingLevelChange={onThinkingLevelChange}
          onCaptureScreen={onCaptureScreen}
        />
      </section>
    );
  }

  const lastId = messages[messages.length - 1]?.id;

  return (
    <section className="stage">
      <div ref={containerRef} className="stage-scroll">
        <div className="stage-inner stage-inner-wide">
          {messages.map((msg, index) => (
            <ChatMessage
              key={`${msg.id}-${index}`}
              role={msg.role}
              content={msg.content}
              timestamp={msg.timestamp}
              isLoading={msg.role === 'assistant' && isLoading && msg.id === lastId}
              thinking={msg.thinking}
              toolCalls={msg.toolCalls}
              images={msg.images}
              onSpeak={
                msg.role === 'assistant' && msg.content && speakingMessageId !== msg.id
                  ? () => onSpeak(msg.content, msg.id)
                  : undefined
              }
              onStopSpeak={
                msg.role === 'assistant' && msg.content && speakingMessageId === msg.id ? onStopSpeak : undefined
              }
              isSpeaking={speakingMessageId === msg.id}
            />
          ))}
          <div ref={messagesEndRef} />
        </div>

        {!stuckToBottom && (
          <button className="scroll-to-bottom-pill" onClick={jumpToBottom} title={t('chat.jumpToLatest')}>
            <ChevronDownOutlined size={14} />
            <span>
              {pendingCount > 0
                ? `${pendingCount} ${t(pendingCount === 1 ? 'chat.newUpdate' : 'chat.newUpdates')}`
                : t('chat.jumpToLatest')}
            </span>
          </button>
        )}
      </div>

      <ChatInput
        onSend={handleLocalSend}
        onStop={onStop}
        disabled={isLoading}
        placeholder={t('input.placeholder')}
        models={models}
        currentModel={currentModel}
        currentThinkingLevel={currentThinkingLevel}
        onModelChange={onModelChange}
        onThinkingLevelChange={onThinkingLevelChange}
        onCaptureScreen={onCaptureScreen}
      />
    </section>
  );
}
