import { KeyboardEvent, useMemo, useState, useRef, useEffect, useCallback } from 'react';
import { SendOutlined, StopOutlined, Settings2, Brain, Camera, Paperclip, CloseCircle } from '../icons';
import type { ImageAttachment } from '../types';
import { useT } from '../i18n';

interface ModelInfo {
  id: string;
  name: string;
  reasoning?: boolean;
  thinkingLevelMap?: Record<string, string>;
}

interface ChatInputProps {
  onSend: (message: string, model?: string, thinkingLevel?: string, images?: ImageAttachment[]) => void;
  onStop?: () => void;
  disabled?: boolean;
  placeholder?: string;
  models?: ModelInfo[];
  currentModel?: string;
  currentThinkingLevel?: string;
  onModelChange?: (model: string) => void;
  onThinkingLevelChange?: (level: string) => void;
  /** Optional handler to capture a screenshot via the backend. Defaults to wails CaptureScreen. */
  onCaptureScreen?: () => Promise<string | null>;
}

const THINKING_LEVELS = [
  { value: '', labelKey: 'thinking.off' },
  { value: 'low', labelKey: 'thinking.low' },
  { value: 'medium', labelKey: 'thinking.medium' },
  { value: 'high', labelKey: 'thinking.high' },
];

const MAX_IMAGE_BYTES = 8 * 1024 * 1024; // 8MB

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
  onCaptureScreen,
}: ChatInputProps) {
  const t = useT();
  const [value, setValue] = useState('');
  const [showModelPicker, setShowModelPicker] = useState(false);
  const [pickerDropdown, setPickerDropdown] = useState<'top' | 'bottom'>('top');
  const [attachments, setAttachments] = useState<ImageAttachment[]>([]);
  const [capturing, setCapturing] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const modelPickerRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

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

  // Read a File into a base64 data URL ImageAttachment
  const fileToAttachment = useCallback((file: File): Promise<ImageAttachment> => {
    return new Promise((resolve, reject) => {
      if (file.size > MAX_IMAGE_BYTES) {
        reject(new Error(`Image too large (max ${MAX_IMAGE_BYTES / 1024 / 1024}MB)`));
        return;
      }
      const reader = new FileReader();
      reader.onload = () => {
        const data = reader.result as string;
        resolve({ data, mimeType: file.type || 'image/png', name: file.name });
      };
      reader.onerror = () => reject(new Error('Failed to read file'));
      reader.readAsDataURL(file);
    });
  }, []);

  const addFiles = useCallback(async (files: FileList | File[]) => {
    const arr = Array.from(files).filter((f) => f.type.startsWith('image/'));
    const next: ImageAttachment[] = [];
    for (const f of arr) {
      try {
        next.push(await fileToAttachment(f));
      } catch (e) {
        // skip invalid files
      }
    }
    if (next.length > 0) {
      setAttachments((prev) => [...prev, ...next]);
    }
  }, [fileToAttachment]);

  const removeAttachment = useCallback((idx: number) => {
    setAttachments((prev) => prev.filter((_, i) => i !== idx));
  }, []);

  // Paste images from clipboard
  const handlePaste = useCallback((e: React.ClipboardEvent) => {
    const items = e.clipboardData?.items;
    if (!items) return;
    const files: File[] = [];
    for (let i = 0; i < items.length; i++) {
      const it = items[i];
      if (it.kind === 'file' && it.type.startsWith('image/')) {
        const f = it.getAsFile();
        if (f) files.push(f);
      }
    }
    if (files.length > 0) {
      e.preventDefault();
      addFiles(files);
    }
  }, [addFiles]);

  const handleScreenshot = useCallback(async () => {
    setCapturing(true);
    try {
      let dataUrl: string | null = null;
      if (onCaptureScreen) {
        dataUrl = await onCaptureScreen();
      } else {
        // Lazy import to avoid breaking SSR / non-wails environments
        const { CaptureScreen } = await import('../../wailsjs/go/main/App');
        dataUrl = await CaptureScreen(0);
      }
      if (dataUrl) {
        const url: string = dataUrl;
        setAttachments((prev) => [...prev, {
          data: url,
          mimeType: 'image/png',
          name: `screenshot-${Date.now()}.png`,
        }]);
      }
    } catch (e: any) {
      console.error('screenshot failed', e);
    } finally {
      setCapturing(false);
    }
  }, [onCaptureScreen]);

  const submit = () => {
    if (disabled) {
      onStop?.();
      return;
    }
    const text = value.trim();
    if (!text && attachments.length === 0) return;
    setValue('');
    const imgs = attachments.length > 0 ? attachments : undefined;
    setAttachments([]);
    onSend(text, currentModel, currentThinkingLevel, imgs);
  };

  const handleKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  };

  const modelLabel = models?.find((m) => m.id === currentModel)?.name || currentModel || t('input.selectModel');
  const currentModelInfo = models?.find((m) => m.id === currentModel);
  const supportsReasoning = currentModelInfo?.reasoning;

  const availableLevels = useMemo(() => {
    if (!currentModelInfo?.thinkingLevelMap) return THINKING_LEVELS;
    const map = currentModelInfo.thinkingLevelMap;
    return THINKING_LEVELS.filter((l) => l.value === '' || map[l.value]);
  }, [currentModelInfo]);

  const showThinking = supportsReasoning && onThinkingLevelChange && availableLevels.length > 1;
  const canSend = (value.trim().length > 0 || attachments.length > 0) && !disabled;

  return (
    <div className="composer">
      <div className={`composer-card ${disabled ? 'is-disabled' : ''}`}>
        {attachments.length > 0 && (
          <div className="composer-attachments">
            {attachments.map((att, idx) => (
              <div key={idx} className="composer-attachment" title={att.name || t('input.altImage')}>
                <img src={att.data} alt={att.name || t('input.altAttachment')} className="composer-attachment-thumb" />
                <button
                  type="button"
                  className="composer-attachment-remove"
                  onClick={() => removeAttachment(idx)}
                  aria-label={t('input.removeAttachment')}
                >
                  <CloseCircle size={12} />
                </button>
              </div>
            ))}
          </div>
        )}

        <textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={handleKey}
          onPaste={handlePaste}
          placeholder={placeholder ?? t('input.placeholder')}
          disabled={disabled}
          rows={1}
          className="composer-input"
        />

        <div className="composer-toolbar">
          <div className="composer-toolbar-left">
            <button
              type="button"
              className="composer-icon-btn"
              onClick={handleScreenshot}
              disabled={disabled || capturing}
              title={capturing ? t('input.capturing') : t('input.captureScreen')}
            >
              <Camera size={14} />
            </button>
            <button
              type="button"
              className="composer-icon-btn"
              onClick={() => fileInputRef.current?.click()}
              disabled={disabled}
              title={t('input.attachImage')}
            >
              <Paperclip size={14} />
            </button>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*"
              multiple
              style={{ display: 'none' }}
              onChange={(e) => {
                if (e.target.files) addFiles(e.target.files);
                e.target.value = '';
              }}
            />

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
                  title={`${t('input.currentModel')}: ${modelLabel}`}
                >
                  <Settings2 size={13} />
                  <span>{modelLabel}</span>
                </button>
                {showModelPicker && (
                  <>
                    <div className="model-picker-overlay" onClick={() => setShowModelPicker(false)} />
                    <div className={`model-picker-dropdown ${pickerDropdown === 'bottom' ? 'bottom' : ''}`}>
                      <div className="model-picker-header">{t('input.switchModel')}</div>
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
              <div className="thinking-control" title={t('thinking.depth')}>
                <Brain size={13} />
                <div className="thinking-levels">
                  {availableLevels.map((lvl) => (
                    <button
                      key={lvl.value}
                      type="button"
                      className={`thinking-level-btn ${currentThinkingLevel === lvl.value ? 'active' : ''}`}
                      onClick={() => onThinkingLevelChange?.(lvl.value)}
                    >
                      {t(lvl.labelKey)}
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
              disabled={!canSend && !disabled}
              aria-label={disabled ? t('input.stop') : t('input.send')}
          title={disabled ? t('input.generating') : t('input.send')}
            >
              {disabled ? <StopOutlined size={14} /> : <SendOutlined size={14} />}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
