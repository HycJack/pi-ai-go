import { KeyboardEvent, useRef, useEffect } from 'react';
import { Play, StopCircle } from 'lucide-react';

interface ChatInputProps {
  value: string;
  onChange: (value: string) => void;
  onSubmit: () => void;
  onCancel: () => void;
  isLoading: boolean;
  placeholder?: string;
}

export default function ChatInput({
  value,
  onChange,
  onSubmit,
  onCancel,
  isLoading,
  placeholder,
}: ChatInputProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = '0px';
    el.style.height = Math.min(el.scrollHeight, 120) + 'px';
  }, [value]);

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      if (isLoading) {
        onCancel();
      } else {
        onSubmit();
      }
    }
  };

  const canSend = value.trim().length > 0;

  return (
    <div className="input-row">
      <textarea
        ref={textareaRef}
        className="input-textarea"
        placeholder={placeholder || '描述你要绘制的图形，例如：画一个边长为 5 的正方形，并标出其内切圆和外接圆'}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={handleKeyDown}
        rows={1}
        disabled={isLoading}
      />
      <div className="input-actions">
        <button
          className={`input-btn send ${isLoading ? 'stop' : ''}`}
          onClick={isLoading ? onCancel : onSubmit}
          disabled={!isLoading && !canSend}
          title={isLoading ? '停止' : '发送'}
        >
          {isLoading ? <StopCircle size={20} /> : <Play size={20} />}
        </button>
      </div>
    </div>
  );
}
