import { Bot, User, Copy, Check, Loader2, Volume2, VolumeX } from 'lucide-react';
import { useState } from 'react';
import MarkdownRenderer from './MarkdownRenderer';
import ThinkingBlock from './ThinkingBlock';
import ToolCallBlock from './ToolCallBlock';

interface ToolCall {
  id: string;
  name: string;
  arguments: string;
}

interface ChatMessageProps {
  role: 'user' | 'assistant';
  content: string;
  timestamp: string;
  isLoading?: boolean;
  thinking?: string;
  toolCalls?: ToolCall[];
  onSpeak?: () => void;
  onStopSpeak?: () => void;
  isSpeaking?: boolean;
}

export default function ChatMessage({ role, content, timestamp, isLoading, thinking, toolCalls, onSpeak, onStopSpeak, isSpeaking }: ChatMessageProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(content);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (role === 'user') {
    return (
      <div className="flex justify-end p-4 hover:bg-slate-800/30 transition-colors">
        <div className="flex gap-3 max-w-[70%]">
          <div className="flex-1 min-w-0">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-medium text-slate-300">You</span>
              <span className="text-xs text-slate-500">{timestamp}</span>
            </div>
            <div className="bg-blue-600 rounded-2xl rounded-br-sm p-4">
              <MarkdownRenderer content={content} />
            </div>
            <button
              onClick={handleCopy}
              className="mt-1.5 flex items-center gap-1.5 text-xs text-slate-400 hover:text-slate-200 transition-colors"
            >
              {copied ? (
                <>
                  <Check className="w-3.5 h-3.5" />
                  <span>Copied!</span>
                </>
              ) : (
                <>
                  <Copy className="w-3.5 h-3.5" />
                  <span>Copy</span>
                </>
              )}
            </button>
          </div>
          <div className="flex-shrink-0 w-10 h-10 rounded-full bg-blue-600 flex items-center justify-center">
            <User className="w-5 h-5 text-white" />
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex justify-start p-4 hover:bg-slate-800/30 transition-colors">
      <div className="flex gap-3 max-w-[70%]">
        <div className="flex-shrink-0 w-10 h-10 rounded-full bg-slate-700 flex items-center justify-center">
          <Bot className="w-5 h-5 text-slate-300" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-slate-300">AI Assistant</span>
            <span className="text-xs text-slate-500">{timestamp}</span>
          </div>
          <div className="bg-slate-800 rounded-2xl rounded-bl-sm p-4 space-y-4">
            <ThinkingBlock content={thinking || ''} />
            <ToolCallBlock toolCalls={toolCalls || []} />
            
            {content ? (
              <MarkdownRenderer content={content} />
            ) : isLoading && !thinking && (!toolCalls || toolCalls.length === 0) ? (
              <div className="flex items-center gap-2">
                <Loader2 className="w-5 h-5 animate-spin text-slate-400" />
                <span className="text-slate-400">Thinking...</span>
              </div>
            ) : null}
          </div>
          {!isLoading && (
            <div className="mt-1.5 flex items-center gap-3">
              <button
                onClick={handleCopy}
                className="flex items-center gap-1.5 text-xs text-slate-400 hover:text-slate-200 transition-colors"
              >
                {copied ? (
                  <>
                    <Check className="w-3.5 h-3.5" />
                    <span>Copied!</span>
                  </>
                ) : (
                  <>
                    <Copy className="w-3.5 h-3.5" />
                    <span>Copy</span>
                  </>
                )}
              </button>
              {content && (onSpeak || onStopSpeak) && (
                <button
                  onClick={isSpeaking ? onStopSpeak : onSpeak}
                  className={`flex items-center gap-1.5 text-xs transition-colors ${
                    isSpeaking ? 'text-blue-400' : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  {isSpeaking ? (
                    <>
                      <VolumeX className="w-3.5 h-3.5" />
                      <span>Stop</span>
                    </>
                  ) : (
                    <>
                      <Volume2 className="w-3.5 h-3.5" />
                      <span>Speak</span>
                    </>
                  )}
                </button>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
