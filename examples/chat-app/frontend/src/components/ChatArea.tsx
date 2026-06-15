import { Bot, Sparkles } from 'lucide-react';
import ChatMessage from './ChatMessage';
import ChatInput from './ChatInput';
import { useRef, useEffect } from 'react';

interface ToolCall {
  id: string;
  name: string;
  arguments: string;
}

interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: string;
  thinking?: string;
  toolCalls?: ToolCall[];
}

interface ChatAreaProps {
  messages: Message[];
  isLoading: boolean;
  onSendMessage: (message: string) => void;
  onSpeak: (text: string, messageId: string) => void;
  onStopSpeak: () => void;
  speakingMessageId: string | null;
}

export default function ChatArea({ messages, isLoading, onSendMessage, onSpeak, onStopSpeak, speakingMessageId }: ChatAreaProps) {
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const messagesContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, isLoading]);

  useEffect(() => {
    const container = messagesContainerRef.current;
    if (container) {
      container.style.overflowY = 'auto';
      container.style.overflowX = 'hidden';
    }
  }, []);

  if (messages.length === 0) {
    return (
      <div className="flex-1 flex flex-col bg-slate-900">
        <div className="flex-1 flex flex-col items-center justify-center px-8">
          <div className="w-20 h-20 bg-gradient-to-br from-blue-500 to-purple-600 rounded-2xl flex items-center justify-center mb-6">
            <Bot className="w-10 h-10 text-white" />
          </div>
          
          <h1 className="text-2xl font-bold text-white mb-2">
            AI Assistant
          </h1>
          
          <p className="text-slate-400 text-center max-w-md mb-8">
            How can I help you today?
          </p>

          <div className="grid grid-cols-2 md:grid-cols-3 gap-3 w-full max-w-lg">
            {[
              { title: 'Explain quantum computing', icon: Sparkles },
              { title: 'Write a poem', icon: Sparkles },
              { title: 'Help with coding', icon: Sparkles },
              { title: 'Plan a trip', icon: Sparkles },
              { title: 'Learn something new', icon: Sparkles },
              { title: 'Generate ideas', icon: Sparkles },
            ].map((item, index) => (
              <button
                key={index}
                onClick={() => onSendMessage(item.title)}
                className="flex items-center gap-2 px-4 py-3 bg-slate-800 hover:bg-slate-700 rounded-lg transition-colors text-left"
              >
                <item.icon className="w-4 h-4 text-blue-400" />
                <span className="text-sm text-slate-300">{item.title}</span>
              </button>
            ))}
          </div>
        </div>

        <ChatInput onSend={onSendMessage} disabled={isLoading} />
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col bg-slate-900 overflow-hidden">
      <div
        ref={messagesContainerRef}
        className="flex-1 overflow-y-auto overflow-x-hidden"
        style={{
          height: 'calc(100vh - 8rem)',
        }}
      >
        {messages.map((msg) => (
          <ChatMessage
            key={msg.id}
            role={msg.role}
            content={msg.content}
            timestamp={msg.timestamp}
            isLoading={msg.role === 'assistant' && isLoading && msg.id === messages[messages.length - 1].id}
            thinking={msg.thinking}
            toolCalls={msg.toolCalls}
            onSpeak={msg.role === 'assistant' && msg.content && speakingMessageId !== msg.id ? () => onSpeak(msg.content, msg.id) : undefined}
            onStopSpeak={msg.role === 'assistant' && msg.content && speakingMessageId === msg.id ? onStopSpeak : undefined}
            isSpeaking={speakingMessageId === msg.id}
          />
        ))}
        <div ref={messagesEndRef} />
      </div>

      <ChatInput onSend={onSendMessage} disabled={isLoading} />
    </div>
  );
}
