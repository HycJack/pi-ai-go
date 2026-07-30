import { useState, useCallback, useEffect, useRef } from 'react';
import {
  Conversation, Message, Settings, GeogebraResult,
  DEFAULT_SETTINGS, getCurrentProvider,
} from './types';
import Sidebar from './components/Sidebar';
import ChatMessage from './components/ChatMessage';
import ChatInput from './components/ChatInput';
import GeoGebraWorkspace, { GeoGebraRef } from './components/GeoGebraWorkspace';
import ScriptEditor from './components/ScriptEditor';
import SettingsPanel from './components/SettingsPanel';
import { log } from './utils/logger';
import {
  GeogebraMessage, CancelStream, GeogebraValidateAndRegenerate,
  GetSettings, SaveSettings,
  GetConversations, SaveConversation, DeleteConversation,
  GetModels,
} from '../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';
import { Triangle } from 'lucide-react';
import { validateGGB } from './lib/geogebra-lint/validator';

// ─── Helpers ───

function nextConvId(): string {
  return `conv_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

function nextMsgId(): string {
  return `msg_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

function serializeSettings(s: Settings): string {
  return JSON.stringify(s);
}

function formatTimestamp(): string {
  return new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
}

// ─── Example prompts ───

const EXAMPLES = [
  { icon: null, text: '画一个直角三角形 ABC，A(0,0), B(3,0), C(0,4)，标出直角标记' },
  { icon: null, text: '画一个正方形，边长为 4，标出对角线的交点' },
  { icon: null, text: '画出函数 y = x^2 和 y = 2x + 1 的图像，标出交点' },
  { icon: null, text: '画一个正五边形及其外接圆' },
];

// ─── App ───

function App() {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeConvId, setActiveConvId] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [input, setInput] = useState('');
  const [settings, setSettings] = useState<Settings>(DEFAULT_SETTINGS);
  const [settingsLoaded, setSettingsLoaded] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [models, setModels] = useState<{ id: string; name: string }[]>([]);
  const [perspective, setPerspective] = useState('2');
  const [ggbCode, setGgbCode] = useState('');
  const [scriptCode, setScriptCode] = useState('');
  const [loadingTip, setLoadingTip] = useState('');
  const [selectedImage, setSelectedImage] = useState<string | null>(null);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [rightPanelWidth, setRightPanelWidth] = useState(420);
  const [chatPanelHeight, setChatPanelHeight] = useState(300);
  const [chatCollapsed, setChatCollapsed] = useState(false);
  const isResizingRightRef = useRef(false);
  const isResizingChatRef = useRef(false);
  
  const retryCountRef = useRef(0);
  const lastPromptRef = useRef<string>('');
  const settingsRef = useRef(settings);
  settingsRef.current = settings;

  const loadingTips = [
    '正在分析题目内容...',
    '识别几何图形特征...',
    '构建数学模型...',
    '生成 GeoGebra 指令...',
    '正在绘制图形...',
    '即将完成...'
  ];

  // Auto-cycle loading tips
  useEffect(() => {
    if (!isLoading) return;
    let index = 0;
    setLoadingTip(loadingTips[0]);
    const interval = setInterval(() => {
      index = (index + 1) % loadingTips.length;
      setLoadingTip(loadingTips[index]);
    }, 2000);
    return () => clearInterval(interval);
  }, [isLoading, loadingTips]);

  // GeoGebra ref
  const ggbRef = useRef<GeoGebraRef>(null);

  // Track current streaming conversation context
  const currentStreamRef = useRef<{ convId: string } | null>(null);
  const eventCleanupRef = useRef<(() => void) | null>(null);

  // ─── Get active conversation ───

  const activeConv = conversations.find((c) => c.id === activeConvId) || null;
  const messages = activeConv?.messages || [];

  // ─── Load settings & conversations on mount ───

  useEffect(() => {
    GetSettings().then((str) => {
      try {
        const s = JSON.parse(str) as Settings;
        if (!s.providers || s.providers.length === 0) {
          setSettings(DEFAULT_SETTINGS);
        } else {
          setSettings({
            ...DEFAULT_SETTINGS,
            providers: s.providers,
            currentProviderIndex: s.currentProviderIndex ?? 0,
            model: s.model || DEFAULT_SETTINGS.model,
            maxTokens: s.maxTokens ?? DEFAULT_SETTINGS.maxTokens,
            temperature: s.temperature ?? DEFAULT_SETTINGS.temperature,
          });
        }
        setSettingsLoaded(true);
      } catch { /* ignore */ }
    }).catch(() => {
      setSettingsLoaded(true);
    });

    GetConversations().then((str) => {
      try {
        const convs = JSON.parse(str) as Conversation[];
        if (convs.length > 0) {
          setConversations(convs);
          setActiveConvId(convs[0].id);
        }
      } catch { /* ignore */ }
    }).catch(() => {});
  }, []);

  // Auto-save settings
  const settingsSaveRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (!settingsLoaded) return;
    if (settingsSaveRef.current) clearTimeout(settingsSaveRef.current);
    settingsSaveRef.current = setTimeout(() => {
      SaveSettings(serializeSettings(settings)).catch(() => {});
    }, 300);
    return () => {
      if (settingsSaveRef.current) clearTimeout(settingsSaveRef.current);
    };
  }, [settings, settingsLoaded]);

  // Auto-save conversations
  useEffect(() => {
    if (!settingsLoaded) return;
    const timeout = setTimeout(() => {
      conversations.forEach((c) => {
        SaveConversation(c.id, JSON.stringify(c)).catch(() => {});
      });
    }, 500);
    return () => clearTimeout(timeout);
  }, [conversations, settingsLoaded]);

  // Fetch models on provider change
  useEffect(() => {
    const cp = getCurrentProvider(settings);
    if (!cp) return;
    GetModels({ provider: cp.type, baseUrl: cp.baseUrl, apiKey: cp.apiKey })
      .then((list) => {
        if (list && list.length > 0) {
          setModels(list);
        }
      })
      .catch(() => {});
  }, [settings.currentProviderIndex, settings.providers]);

  // ─── Stream events registration ───

  const registerGeogebraEvents = useCallback((convId: string) => {
    const guard = () => {
      const cur = currentStreamRef.current;
      return cur && cur.convId === convId;
    };

    const cleanupFns: (() => void)[] = [];

    const on = (event: string, fn: (...args: any[]) => void) => {
      const wrapped = (...args: any[]) => {
        if (guard()) fn(...args);
      };
      EventsOn(event, wrapped);
      cleanupFns.push(() => EventsOff(event));
    };

    on('geogebra-text-delta', (delta: string) => {
      setConversations((prev) => prev.map((c) => {
        if (c.id !== convId) return c;
        const msgs = [...c.messages];
        const last = msgs[msgs.length - 1];
        if (last && last.role === 'assistant') {
          msgs[msgs.length - 1] = { ...last, content: last.content + delta };
        }
        return { ...c, messages: msgs };
      }));
    });

    on('geogebra-done', (data: string) => {
      const currentConvId = convId;
      try {
        const result = JSON.parse(data) as GeogebraResult;

        setConversations((prev) => prev.map((c) => {
          if (c.id !== currentConvId) return c;
          const msgs = [...c.messages];
          const last = msgs[msgs.length - 1];
          if (last && last.role === 'assistant') {
            msgs[msgs.length - 1] = {
              ...last,
              content: result.text || last.content,
              timestamp: formatTimestamp(),
            };
          }
          return {
            ...c,
            messages: msgs,
            result,
            timestamp: formatTimestamp(),
          };
        }));

        // Update script editor with GGB code
        if (result.ggbCode) {
          setScriptCode(result.ggbCode);
        }

        // Validate GGB code and auto-retry if errors found
        const s = settingsRef.current;
        if (result.ggbCode && retryCountRef.current < 2) {
          const validationErrors = validateGGB(result.ggbCode);
          if (validationErrors) {
            retryCountRef.current += 1;
            log('info', 'geogebra-lint', {
              convId: currentConvId,
              retryCount: retryCountRef.current,
              errors: validationErrors,
            });

            const lintUserMsg: Message = {
              id: nextMsgId(),
              role: 'user',
              content: `🔍 GeoGebra 命令校验不通过，报错如下：\n\n\`\`\`\n${validationErrors}\n\`\`\`\n\n请修正以上问题后重新生成。`,
              timestamp: formatTimestamp(),
            };
            const newAssistantMsg: Message = {
              id: nextMsgId(),
              role: 'assistant',
              content: '',
              timestamp: '',
            };

            setConversations((prev) => prev.map((c) => {
              if (c.id !== currentConvId) return c;
              return {
                ...c,
                messages: [...c.messages, lintUserMsg, newAssistantMsg],
                result: c.result,
                timestamp: formatTimestamp(),
              };
            }));

            lastPromptRef.current = lastPromptRef.current +
              `\n\n（之前的生成有校验错误，需要修正。错误：${validationErrors}）`;

            const cp = getCurrentProvider(s);
            const payload = JSON.stringify({
              message: '',
              originalMessage: lastPromptRef.current,
              ggbCode: result.ggbCode,
              validationErrors,
              provider: cp?.type || '',
              apiKey: cp?.apiKey || '',
              baseURL: cp?.baseUrl || '',
              model: s.model,
              maxTokens: s.maxTokens,
              temperature: s.temperature,
            });
            GeogebraValidateAndRegenerate(payload).catch((err: Error) => {
              log('error', 'geogebra-regen', { currentConvId, error: err.message });
              setIsLoading(false);
              currentStreamRef.current = null;
            });
            return;
          }
        }
      } catch (e) { /* ignore */ }
      setIsLoading(false);
      currentStreamRef.current = null;
    });

    on('geogebra-error', (error: string) => {
      log('error', 'geogebra-error', { convId, error });
      setConversations((prev) => prev.map((c) => {
        if (c.id !== convId) return c;
        const msgs = [...c.messages];
        const last = msgs[msgs.length - 1];
        if (last && last.role === 'assistant') {
          msgs[msgs.length - 1] = {
            ...last,
            content: last.content + `\n\n错误: ${error}`,
            timestamp: formatTimestamp(),
          };
        }
        return { ...c, messages: msgs };
      }));
      setIsLoading(false);
      currentStreamRef.current = null;
    });

    return () => cleanupFns.forEach((fn) => fn());
  }, []);

  // Cleanup events on unmount
  useEffect(() => {
    return () => {
      if (eventCleanupRef.current) {
        eventCleanupRef.current();
        eventCleanupRef.current = null;
      }
    };
  }, []);

  // ─── Execute GGB commands ───

  const handleExecuteGGB = useCallback((commands: string[]) => {
    if (!ggbRef.current) return;
    
    commands.forEach((cmd) => {
      const trimmed = cmd.trim();
      if (trimmed && !trimmed.startsWith('#') && !trimmed.startsWith('//')) {
        ggbRef.current?.executeCommand(trimmed);
      }
    });
  }, []);

  const handleResetGGB = useCallback(() => {
    ggbRef.current?.reset();
  }, []);

  // ─── Submit ───

  const handleSubmit = useCallback(() => {
    const text = input.trim();
    if (!text) return;

    let convId = activeConvId;
    if (!convId || (activeConv && activeConv.messages.length > 0 && !activeConv.result)) {
      convId = nextConvId();
    }

    const userMsg: Message = {
      id: nextMsgId(),
      role: 'user',
      content: text,
      timestamp: formatTimestamp(),
    };
    const assistantMsg: Message = {
      id: nextMsgId(),
      role: 'assistant',
      content: '',
      timestamp: '',
    };

    const newConv: Conversation = convId === activeConvId && activeConv
      ? {
          ...activeConv,
          messages: [...activeConv.messages, userMsg, assistantMsg],
          prompt: text,
        }
      : {
          id: convId,
          title: text.slice(0, 40) + (text.length > 40 ? '...' : ''),
          messages: [userMsg, assistantMsg],
          prompt: text,
          timestamp: formatTimestamp(),
        };

    if (eventCleanupRef.current) {
      eventCleanupRef.current();
    }

    retryCountRef.current = 0;
    lastPromptRef.current = text;

    setConversations((prev) => {
      const existing = prev.findIndex((c) => c.id === convId);
      if (existing >= 0) {
        const updated = [...prev];
        updated[existing] = newConv;
        return updated;
      }
      return [newConv, ...prev].slice(0, 50);
    });
    setActiveConvId(convId);
    setInput('');
    setIsLoading(true);

    currentStreamRef.current = { convId };
    eventCleanupRef.current = registerGeogebraEvents(convId);

    const cp = getCurrentProvider(settings);

    const historyMsgs = newConv.messages
      .filter((m) => m.id !== userMsg.id && m.id !== assistantMsg.id && m.content)
      .map((m) => ({ role: m.role, content: m.content }));

    const payload = JSON.stringify({
      message: text,
      historyMessages: historyMsgs,
      provider: cp?.type || '',
      apiKey: cp?.apiKey || '',
      baseURL: cp?.baseUrl || '',
      model: settings.model,
      maxTokens: settings.maxTokens,
      temperature: settings.temperature,
      perspective: perspective,
      image: selectedImage || undefined,
    });

    setSelectedImage(null);

    GeogebraMessage(payload).catch((err: Error) => {
      setIsLoading(false);
      currentStreamRef.current = null;
      setConversations((prev) => prev.map((c) => {
        if (c.id !== convId) return c;
        const msgs = [...c.messages];
        const last = msgs[msgs.length - 1];
        if (last && last.role === 'assistant') {
          msgs[msgs.length - 1] = { ...last, content: `错误: ${err.message}`, timestamp: formatTimestamp() };
        }
        return { ...c, messages: msgs };
      }));
    });
  }, [input, activeConvId, activeConv, settings, registerGeogebraEvents, perspective]);

  // ─── Cancel ───

  const handleCancel = useCallback(() => {
    CancelStream().catch(() => {});
    setIsLoading(false);
    currentStreamRef.current = null;
  }, []);

  // ─── Input change ───

  const handleInputChange = useCallback((value: string) => {
    setInput(value);
  }, []);

  const handleImageUpload = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      const reader = new FileReader();
      reader.onloadend = () => {
        setSelectedImage(reader.result as string);
      };
      reader.readAsDataURL(file);
    }
  }, []);

  const handleRemoveImage = useCallback(() => {
    setSelectedImage(null);
  }, []);

  // ─── New conversation ───

  const handleNewConversation = useCallback(() => {
    // Reset GeoGebra canvas
    ggbRef.current?.reset();
    // Reset the first-ready flag so saved script re-runs on next ready
    isGgbFirstReadyRef.current = true;
    setActiveConvId(null);
    setScriptCode('');
  }, []);

  // ─── Select conversation ───

  const handleSelectConversation = useCallback((id: string) => {
    setActiveConvId(id);
    const conv = conversations.find((c) => c.id === id);
    // Reset GeoGebra canvas
    ggbRef.current?.reset();
    isGgbFirstReadyRef.current = true;
    if (conv?.result?.ggbCode) {
      setScriptCode(conv.result.ggbCode);
    } else {
      setScriptCode('');
    }
  }, [conversations]);

  // ─── Delete conversation ───

  const handleDeleteConversation = useCallback((id: string) => {
    setConversations((prev) => prev.filter((c) => c.id !== id));
    DeleteConversation(id).catch(() => {});
    if (activeConvId === id) {
      setActiveConvId(null);
      setScriptCode('');
    }
  }, [activeConvId]);

  // ─── Save settings ───

  const handleSaveSettings = useCallback((s: Settings) => {
    setSettings(s);
  }, []);

  // ─── GeoGebra ready ───

  // Only auto-execute the saved script on the FIRST ready event.
  // When the user switches perspective, the applet is rebuilt and onReady
  // fires again — in that case we must NOT re-run the script automatically.
  const isGgbFirstReadyRef = useRef(true);
  const scriptCodeRef = useRef(scriptCode);
  scriptCodeRef.current = scriptCode;

  const handleGeoGebraReady = useCallback((api: any) => {
    if (isGgbFirstReadyRef.current) {
      isGgbFirstReadyRef.current = false;
      const code = scriptCodeRef.current;
      if (code) {
        const lines = code.split('\n').filter(line => line.trim());
        lines.forEach((cmd) => api.evalCommand(cmd));
      }
    }
  }, []);

  // ─── Script change ───

  const handleScriptChange = useCallback((code: string) => {
    setScriptCode(code);
  }, []);

  const handleScriptAiModify = useCallback(async (instruction: string, selectedText: string) => {
    // TODO: 接入后端 AI 修改脚本接口
    console.log('[AI Modify] instruction:', instruction, 'selected:', selectedText);
    // 占位：简单在选中文本前插入注释
    const marker = `// AI: ${instruction}\n`;
    setScriptCode((prev) => prev + '\n' + marker + selectedText);
  }, []);

  // ─── Resize handlers (follow GeogebraRunner pattern) ───

  const resizeApplet = useCallback((width: number, height: number) => {
    if (!ggbRef.current?.setSize) return;
    const w = Math.round(width);
    const h = Math.round(height);
    try {
      ggbRef.current.setSize(w, h);
    } catch {
      // ignore resize race
    }
  }, []);

  const rightPanelRef = useRef<HTMLDivElement>(null);
  const resizeObserverRef = useRef<ResizeObserver | null>(null);
  const resizeTimeoutRef = useRef<number | null>(null);
  const startXRef = useRef(0);
  const startWidthRef = useRef(0);

  const handleRightMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    isResizingRightRef.current = true;
    startXRef.current = e.clientX;
    startWidthRef.current = rightPanelWidth;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  }, [rightPanelWidth]);

  const handleChatMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    isResizingChatRef.current = true;
    document.body.style.cursor = 'row-resize';
    document.body.style.userSelect = 'none';
  }, []);

  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (isResizingRightRef.current) {
      const delta = startXRef.current - e.clientX;
      const newWidth = Math.max(360, Math.min(1000, startWidthRef.current + delta));
      setRightPanelWidth(newWidth);
    }

    if (isResizingChatRef.current) {
      const container = rightPanelRef.current as HTMLElement | null;
      if (!container) return;
      const rect = container.getBoundingClientRect();
      const newHeight = rect.bottom - e.clientY - 48;
      const clamped = Math.min(Math.max(newHeight, 120), rect.height - 200);
      setChatPanelHeight(clamped);
    }
  }, []);

  const handleMouseUp = useCallback(() => {
    if (isResizingRightRef.current) {
      isResizingRightRef.current = false;
      document.body.style.cursor = '';
      document.body.style.userSelect = '';

      // Force GeoGebra resize after drag ends
      requestAnimationFrame(() => {
        if (!rightPanelRef.current || !ggbRef.current?.setSize) return;
        const parent = rightPanelRef.current.parentElement;
        if (!parent) return;
        const rect = parent.getBoundingClientRect();
        resizeApplet(rect.width, rect.height);
      });
    }

    if (isResizingChatRef.current) {
      isResizingChatRef.current = false;
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    }
  }, [resizeApplet]);

  useEffect(() => {
    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseup', handleMouseUp);
    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', handleMouseUp);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };
  }, [handleMouseMove, handleMouseUp]);

  // Setup ResizeObserver on right panel parent (workspace area) for GGB
  useEffect(() => {
    if (!rightPanelRef.current) return;

    const parent = rightPanelRef.current.parentElement;
    if (!parent) return;

    const observer = new ResizeObserver((entries) => {
      if (!ggbRef.current?.setSize) return;
      for (const entry of entries) {
        const rect = parent.getBoundingClientRect();
        const width = rect.width;
        const height = rect.height;
        if (width > 0 && height > 0) {
          resizeApplet(width, height);
        }
      }
    });

    observer.observe(parent);
    resizeObserverRef.current = observer;

    return () => {
      observer.disconnect();
      resizeObserverRef.current = null;
    };
  }, [resizeApplet]);

  // Listen to window resize as fallback
  useEffect(() => {
    const handleResize = () => {
      if (!rightPanelRef.current || !ggbRef.current?.setSize) return;
      const parent = rightPanelRef.current.parentElement;
      if (!parent) return;

      requestAnimationFrame(() => {
        const rect = parent.getBoundingClientRect();
        if (rect.width > 0 && rect.height > 0) {
          resizeApplet(rect.width, rect.height);
        }
      });
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [resizeApplet]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (resizeTimeoutRef.current !== null) {
        window.clearTimeout(resizeTimeoutRef.current);
        resizeTimeoutRef.current = null;
      }
    };
  }, []);

  // Show empty state? (no conversations and not loading)
  const showEmpty = !isLoading && messages.length === 0;

  const cp = getCurrentProvider(settings);

  return (
    <div
      className={`app-shell ${sidebarCollapsed ? 'sidebar-collapsed' : ''}`}
      style={{ '--right-width': `${rightPanelWidth}px` } as React.CSSProperties}
    >
      {/* Main Content */}
      <div className="main-container">
        {/* Sidebar */}
        <Sidebar
          conversations={conversations}
          activeConversationId={activeConvId}
          onSelectConversation={handleSelectConversation}
          onNewConversation={handleNewConversation}
          onDeleteConversation={handleDeleteConversation}
          onOpenSettings={() => setShowSettings(true)}
          collapsed={sidebarCollapsed}
          onToggleCollapse={() => setSidebarCollapsed((v) => !v)}
        />

        {/* Center: GeoGebra Workspace */}
        <GeoGebraWorkspace
          ref={ggbRef}
          perspective={perspective}
          onPerspectiveChange={setPerspective}
          onReady={handleGeoGebraReady}
        />

        {/* Right: Script + Chat */}
        <div className="right-panel" ref={rightPanelRef}>
          {/* Horizontal resize handle */}
          <div
            className="resize-handle resize-handle-horizontal"
            onMouseDown={handleRightMouseDown}
          />

          <div className="script-panel">
            <ScriptEditor
              initialCode={scriptCode}
              onSave={handleScriptChange}
              onExecute={handleExecuteGGB}
              onReset={handleResetGGB}
              onAiModify={handleScriptAiModify}
            />
            {/* Vertical resize handle */}
            <div
              className="resize-handle resize-handle-vertical"
              onMouseDown={handleChatMouseDown}
            />
          </div>

          <div className="chat-panel" style={{ height: chatCollapsed ? 32 : chatPanelHeight }}>
            <div className="chat-panel-header">
              <span className="chat-panel-title">对话</span>
              <button
                className="chat-panel-toggle"
                onClick={() => setChatCollapsed((v) => !v)}
                title={chatCollapsed ? '展开' : '收起'}
              >
                {chatCollapsed ? '▲' : '▼'}
              </button>
            </div>
            {!chatCollapsed && (
              <div className="chat-panel-body">
                {showEmpty && (
                  <div className="empty-state">
                    <div className="empty-icon">
                      <Triangle size={28} />
                    </div>
                    <h2 className="empty-title">GeoGebra 指令生成器</h2>
                    <p className="empty-sub">输入你的几何、代数或函数描述</p>
                    <div className="example-grid">
                      {EXAMPLES.map((ex, i) => (
                        <button
                          key={i}
                          className="example-card"
                          onClick={() => setInput(ex.text)}
                        >
                          {ex.text}
                        </button>
                      ))}
                    </div>
                  </div>
                )}

                {!showEmpty && (
                  <div className="messages-container">
                    <div className="messages-list">
                      {messages.map((msg) => (
                        <ChatMessage
                          key={msg.id}
                          role={msg.role}
                          content={msg.content}
                          timestamp={msg.timestamp}
                          isLoading={msg.role === 'assistant' && isLoading && msg.id === messages[messages.length - 1]?.id}
                        />
                      ))}
                    </div>
                  </div>
                )}

                <div className="input-area">
                  <ChatInput
                    value={input}
                    onChange={handleInputChange}
                    onSubmit={handleSubmit}
                    onCancel={handleCancel}
                    isLoading={isLoading}
                    selectedImage={selectedImage}
                    onImageUpload={handleImageUpload}
                    onRemoveImage={handleRemoveImage}
                  />
                  {isLoading && loadingTip && (
                    <div className="loading-tip">{loadingTip}</div>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Settings panel */}
      <SettingsPanel
        isOpen={showSettings}
        onClose={() => setShowSettings(false)}
        settings={settings}
        onSave={handleSaveSettings}
      />
    </div>
  );
}

export default App;
