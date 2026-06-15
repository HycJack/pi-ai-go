import { Send, Paperclip, Smile } from 'lucide-react';
import { useState, KeyboardEvent } from 'react';

interface ChatInputProps {
  onSend: (message: string) => void;
  disabled?: boolean;
}

export default function ChatInput({ onSend, disabled }: ChatInputProps) {
  const [message, setMessage] = useState('');

  const handleSubmit = () => {
    if (message.trim() && !disabled) {
      onSend(message.trim());
      setMessage('');
    }
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  return (
    <div className="border-t border-slate-700 bg-slate-900 p-4">
      <div className="max-w-3xl mx-auto">
        <div className="flex items-end gap-3 bg-slate-800 rounded-xl border border-slate-600 p-2">
          <button className="p-2 hover:bg-slate-700 rounded-lg transition-colors flex-shrink-0">
            <Paperclip className="w-5 h-5 text-slate-400" />
          </button>

          <textarea
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Message AI Assistant..."
            disabled={disabled}
            className="flex-1 bg-transparent resize-none focus:outline-none p-2 text-slate-200 placeholder-slate-400 text-sm max-h-32"
            rows={1}
          />

          <button className="p-2 hover:bg-slate-700 rounded-lg transition-colors flex-shrink-0">
            <Smile className="w-5 h-5 text-slate-400" />
          </button>

          <button
            onClick={handleSubmit}
            disabled={!message.trim() || disabled}
            className="p-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed rounded-lg transition-colors flex-shrink-0"
          >
            <Send className="w-5 h-5 text-white" />
          </button>
        </div>

        <p className="text-xs text-slate-500 text-center mt-3">
          AI Assistant can make mistakes. Consider checking important information.
        </p>
      </div>
    </div>
  );
}
