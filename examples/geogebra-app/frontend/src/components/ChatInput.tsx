import { KeyboardEvent, useRef, useEffect, ChangeEvent } from 'react';
import { Play, StopCircle, Image as ImageIcon, X } from 'lucide-react';

interface ChatInputProps {
  value: string;
  onChange: (value: string) => void;
  onSubmit: () => void;
  onCancel: () => void;
  isLoading: boolean;
  placeholder?: string;
  selectedImage?: string | null;
  onImageUpload?: (e: ChangeEvent<HTMLInputElement>) => void;
  onRemoveImage?: () => void;
}

export default function ChatInput({
  value,
  onChange,
  onSubmit,
  onCancel,
  isLoading,
  placeholder,
  selectedImage,
  onImageUpload,
  onRemoveImage,
}: ChatInputProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

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
      <div className="input-textarea-wrap">
        {selectedImage && onRemoveImage && (
          <div className="input-image-preview">
            <img src={selectedImage} alt="Upload" />
            <button
              type="button"
              onClick={onRemoveImage}
              className="input-image-remove"
              title="移除图片"
            >
              <X size={12} />
            </button>
          </div>
        )}
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
      </div>
      <div className="input-actions">
        {onImageUpload && (
          <button
            type="button"
            className="input-btn upload"
            onClick={() => fileInputRef.current?.click()}
            disabled={isLoading}
            title="上传图片"
          >
            <ImageIcon size={20} />
          </button>
        )}
        <input
          ref={fileInputRef}
          type="file"
          accept="image/*"
          className="hidden"
          onChange={onImageUpload}
        />
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
