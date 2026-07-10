import { KeyboardEvent, useMemo, useState, useRef, useEffect } from 'react';
import { SendOutlined, StopOutlined, Settings2, Brain } from '../icons';

interface ModelInfo {
  id: string;
  name: string;
  reasoning?: boolean;
  thinkingLevelMap?: Record<string, string>;
}

interface ChatInputProps {
  onSend: (message: string, model?: string, thinkingLevel?: string) => void;
  onStop?: () => void;
  disabled?: boolean;
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
  onStop,
  disabled,
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

  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = '0px';
    const scrollH = el.scrollHeight;
    el.style.height = Math.min(scrollH, 160) + 'px';
  }, [value]);

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

  const submit = () => {
    if (disabled) {
      onStop?.();
      return;
    }
    const text = value.trim();
    if (!text) return;
    setValue('');
    onSend(text, currentModel, currentThinkingLevel);
  };

  const handleKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  };

  const modelLabel = models?.find((m) => m.id === currentModel)?.name || currentModel || 'Select model';
  const currentModelInfo = models?.find((m) => m.id === currentModel);
  const supportsReasoning = currentModelInfo?.reasoning;

  const availableLevels = useMemo(() => {
    if (!currentModelInfo?.thinkingLevelMap) return THINKING_LEVELS;
    const map = currentModelInfo.thinkingLevelMap;
    return THINKING_LEVELS.filter((l) => l.value === '' || map[l.value]);
  }, [currentModelInfo]);

  const showThinking = supportsReasoning && onThinkingLevelChange && availableLevels.length > 1;

  return (
    <div className="composer">
      <div className={`composer-card ${disabled ? 'is-disabled' : ''}`}>
        <textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={handleKey}
          placeholder={placeholder ?? 'Message Pi-AI…'}
          disabled={disabled}
          rows={1}
          className="composer-input"
        />

        <div className="composer-toolbar">
          <div className="composer-toolbar-left">
            {models && models.length > 0 && (
              <div className="model-picker-wrap" ref={modelPickerRef}>
                <button
                  type="button"
                  className="model-picker-btn"
                  onClick={() => {
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
                    <div className="model-picker-overlay" onClick={() => setShowModelPicker(false)} />
                    <div className={`model-picker-dropdown ${pickerDropdown === 'bottom' ? 'bottom' : ''}`}>
                      <div className="model-picker-header">Switch model</div>
                      {models.map((m) => (
                        <button
                          key={m.id}
                          className={`model-picker-item ${currentModel === m.id ? 'active' : ''}`}
                          onClick={() => {
                            onModelChange?.(m.id);
                            setShowModelPicker(false);
                          }}
                        >
                          <span className="model-picker-name">{m.name}</span>
                          <span className="model-picker-id">{m.id}</span>
                        </button>
                      ))}
                    </div>
                  </>
                )}
              </div>
            )}

            {showThinking && (
              <div className="thinking-control" title="Thinking depth">
                <Brain size={13} />
                <div className="thinking-levels">
                  {availableLevels.map((lvl) => (
                    <button
                      key={lvl.value}
                      type="button"
                      className={`thinking-level-btn ${currentThinkingLevel === lvl.value ? 'active' : ''}`}
                      onClick={() => onThinkingLevelChange?.(lvl.value)}
                    >
                      {lvl.label}
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>

          <div className="composer-toolbar-right">
            <button
              type="button"
              className={`send-btn ${disabled ? 'is-loading' : ''}`}
              onClick={submit}
              disabled={!value.trim() && !disabled}
              aria-label={disabled ? 'Stop' : 'Send'}
              title={disabled ? 'Generating…' : 'Send'}
            >
              {disabled ? <StopOutlined size={14} /> : <SendOutlined size={14} />}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
