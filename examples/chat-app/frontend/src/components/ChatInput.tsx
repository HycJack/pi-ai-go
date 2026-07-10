import { Send, Square, Settings2, Brain } from 'lucide-react';
import { useState, useEffect, useRef, useCallback, KeyboardEvent } from 'react';

interface ModelInfo {
  id: string;
  name: string;
  reasoning?: boolean;
  thinkingLevelMap?: Record<string, string>;
}

interface ChatInputProps {
  onSend: (message: string, model?: string, thinkingLevel?: string) => void;
  onCancel?: () => void;
  disabled?: boolean;
  isLoading?: boolean;
  placeholder?: string;
  models?: ModelInfo[];
  currentModel?: string;
  currentThinkingLevel?: string;
  onModelChange?: (model: string) => void;
  onThinkingLevelChange?: (level: string) => void;
}

const THINKING_LEVELS = [
  { value: '', label: 'Off' },
  { value: 'low', label: 'Low' },
  { value: 'medium', label: 'Medium' },
  { value: 'high', label: 'High' },
];

export default function ChatInput({
  onSend,
  onCancel,
  disabled,
  isLoading,
  placeholder,
  models,
  currentModel,
  currentThinkingLevel = '',
  onModelChange,
  onThinkingLevelChange,
}: ChatInputProps) {
  const [value, setValue] = useState('');
  const [showModelPicker, setShowModelPicker] = useState(false);
  const [pickerDropdown, setPickerDropdown] = useState<'top' | 'bottom'>('top');
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const modelPickerRef = useRef<HTMLDivElement>(null);

  // Auto-resize textarea
  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = '0px';
    const scrollH = el.scrollHeight;
    el.style.height = Math.min(scrollH, 160) + 'px';
  }, [value]);

  // Close model picker on outside click
  useEffect(() => {
    if (!showModelPicker) return;
    const handler = (e: MouseEvent) => {
      if (modelPickerRef.current && !modelPickerRef.current.contains(e.target as Node)) {
        setShowModelPicker(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [showModelPicker]);

  const submit = useCallback(() => {
    if (isLoading) {
      onCancel?.();
      return;
    }
    const text = value.trim();
    if (!text) return;
    setValue('');
    onSend(text, currentModel, currentThinkingLevel);
  }, [value, isLoading, onSend, onCancel, currentModel, currentThinkingLevel]);

  const handleKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  };

  const modelLabel = models?.find((m) => m.id === currentModel)?.name || currentModel || 'Select model';
  const currentModelInfo = models?.find((m) => m.id === currentModel);
  const supportsReasoning = currentModelInfo?.reasoning;

  const availableLevels = (() => {
    if (!currentModelInfo?.thinkingLevelMap) return THINKING_LEVELS;
    const map = currentModelInfo.thinkingLevelMap;
    return THINKING_LEVELS.filter((l) => l.value === '' || map[l.value]);
  })();

  const showThinking = supportsReasoning && onThinkingLevelChange && availableLevels.length > 1;

  return (
    <div className="pi-composer">
      <div className={`pi-composer-card ${isLoading ? 'is-loading' : ''}`}>
        <div className="pi-composer-card-inner">

        {/* Textarea */}
        <textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={handleKey}
          placeholder={placeholder ?? 'Message AI Assistant…'}
          disabled={isLoading}
          rows={1}
          className="pi-composer-input"
        />

        {/* Toolbar */}
        <div className="pi-composer-toolbar">
          <div className="pi-composer-toolbar-left">
            {/* Model selector */}
            {models && models.length > 0 && (
              <div className="pi-model-picker-wrap" ref={modelPickerRef}>
                <button
                  type="button"
                  className="pi-model-picker-btn"
                  onClick={() => {
                    // 检测可用空间，决定 dropdown 展开方向
                    if (modelPickerRef.current) {
                      const rect = modelPickerRef.current.getBoundingClientRect();
                      const spaceAbove = rect.top;
                      setPickerDropdown(spaceAbove < 300 ? 'bottom' : 'top');
                    }
                    setShowModelPicker(!showModelPicker);
                  }}
                  title={`Current model: ${modelLabel}`}
                >
                  <Settings2 size={13} />
                  <span>{modelLabel}</span>
                </button>
                {showModelPicker && (
                  <>
                    <div className="pi-model-picker-overlay" onClick={() => setShowModelPicker(false)} />
                    <div className={`pi-model-picker-dropdown ${pickerDropdown === 'bottom' ? 'bottom' : ''}`}>
                      <div className="pi-model-picker-header">Switch model</div>
                      {models.map((m) => (
                        <button
                          key={m.id}
                          className={`pi-model-picker-item ${currentModel === m.id ? 'active' : ''}`}
                          onClick={() => {
                            onModelChange?.(m.id);
                            setShowModelPicker(false);
                          }}
                        >
                          <span className="pi-model-picker-name">{m.name}</span>
                          <span className="pi-model-picker-id">{m.id}</span>
                        </button>
                      ))}
                    </div>
                  </>
                )}
              </div>
            )}

            {/* Thinking level */}
            {showThinking && (
              <div className="pi-thinking-control" title="Thinking depth">
                <Brain size={13} />
                <div className="pi-thinking-levels">
                  {availableLevels.map((lvl) => (
                    <button
                      key={lvl.value}
                      type="button"
                      className={`pi-thinking-level-btn ${currentThinkingLevel === lvl.value ? 'active' : ''}`}
                      onClick={() => onThinkingLevelChange?.(lvl.value)}
                    >
                      {lvl.label}
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>

          <div className="pi-composer-toolbar-right">
            <button
              type="button"
              className={`pi-send-btn ${isLoading ? 'is-loading' : ''}`}
              onClick={submit}
              disabled={!value.trim() && !isLoading}
              aria-label={isLoading ? 'Stop' : 'Send'}
              title={isLoading ? 'Stop generating' : 'Send'}
            >
              {isLoading ? <Square size={14} /> : <Send size={14} />}
            </button>
          </div>
        </div>

        </div>
      </div>

      <p className="pi-composer-footer-text">
        AI Assistant can make mistakes. Consider checking important information.
      </p>
    </div>
  );
}
