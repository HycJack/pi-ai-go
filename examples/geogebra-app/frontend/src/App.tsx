import { useState, useCallback, useEffect, useRef } from 'react';
import {
  Conversation, Message, Settings, GeogebraResult,
  DEFAULT_SETTINGS, getCurrentProvider,
} from './types';
import Sidebar from './components/Sidebar';
import ChatMessage from './components/ChatMessage';
import ChatInput from './components/ChatInput';
import GeogebraRunner from './components/GeogebraRunner';
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
  return `conv_${Date.now()}_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
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
  const [rightPanel, setRightPanel] = useState<{ html: string; ggbCode: string; svg: string; activeTab: 'html' | 'ggb' | 'svg' } | null>(null);
  const retryCountRef = useRef(0);
  const lastPromptRef = useRef<string>('');
  const settingsRef = useRef(settings);
  settingsRef.current = settings;

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
    // Guard: only process events for the current stream
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

        // Update conversation: finalize assistant message + set result
        setConversations((prev) => prev.map((c) => {
          if (c.id !== currentConvId) return c;
          const msgs = [...c.messages];
          const last = msgs[msgs.length - 1];
          if (last && last.role === 'assistant') {
            msgs[msgs.length - 1] = {
              ...last,
              content: result.text || last.content,
              html: result.html,
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

            // Add a user message showing lint errors
            const lintUserMsg: Message = {
              id: nextMsgId(),
              role: 'user',
              content: `🔍 GeoGebra 命令校验不通过，报错如下：\n\n\`\`\`\n${validationErrors}\n\`\`\`\n\n请修正以上问题后重新生成。`,
              timestamp: formatTimestamp(),
            };
            // Add a new empty assistant message for the regenerated result
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
                result: c.result, // Keep previous result to avoid UI flicker
                timestamp: formatTimestamp(),
              };
            }));

            // Update lastPromptRef with the regen prompt so it gets the full context
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
            return; // Don't end loading; regen stream handles it
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

  // ─── Submit ───

  const handleSubmit = useCallback(() => {
    const text = input.trim();
    if (!text) return;

    // Create or reuse conversation
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

    // Cleanup previous events
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

    // Build history messages for multi-turn context
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
    });

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
  }, [input, activeConvId, activeConv, settings, registerGeogebraEvents]);

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

  // ─── New conversation ───

  const handleNewConversation = useCallback(() => {
    setActiveConvId(null);
  }, []);

  // ─── Select conversation ───

  const handleSelectConversation = useCallback((id: string) => {
    setActiveConvId(id);
  }, []);

  // ─── Delete conversation ───

  const handleDeleteConversation = useCallback((id: string) => {
    const conv = conversations.find((c) => c.id === id);
    const title = conv?.title || '此对话';
    if (!confirm(`确定要删除"${title}"吗？此操作不可恢复。`)) return;
    setConversations((prev) => prev.filter((c) => c.id !== id));
    DeleteConversation(id).catch(() => {});
    if (activeConvId === id) {
      setActiveConvId(null);
    }
  }, [activeConvId, conversations]);

  // ─── Save settings ───

  const handleSaveSettings = useCallback((s: Settings) => {
    setSettings(s);
  }, []);

  // ─── Right Panel ───

  const handleExecuteGGB = useCallback((code: string) => {
    setRightPanel((prev) => ({
      html: prev?.html || '',
      ggbCode: code,
      svg: prev?.svg || '',
      activeTab: 'ggb',
    }));
  }, []);

  const handleOpenHTMLPreview = useCallback((html: string, ggbCode: string, svg: string) => {
    setRightPanel({ html, ggbCode, svg, activeTab: svg ? 'svg' : 'html' });
  }, []);

  const handleCloseRightPanel = useCallback(() => {
    setRightPanel(null);
  }, []);

  // Show empty state? (no conversations and not loading)
  const showEmpty = !isLoading && messages.length === 0;

  const cp = getCurrentProvider(settings);

  return (
    <div className="app-shell">
      {/* Sidebar */}
      <Sidebar
        conversations={conversations}
        activeConversationId={activeConvId}
        onSelectConversation={handleSelectConversation}
        onNewConversation={handleNewConversation}
        onDeleteConversation={handleDeleteConversation}
        onOpenSettings={() => setShowSettings(true)}
      />

      {/* Main area */}
      <main className="main-frame">
        {/* Top bar */}
        <div className="topbar">
          <div className="topbar-left">
            <span className="topbar-title">
              {activeConv ? activeConv.title : 'GeoGebra 指令生成'}
            </span>
            {cp && <span className="topbar-model-badge">{cp.name} · {settings.model}</span>}
          </div>
        </div>

        {/* Content */}
        <div className="main-content">
          {showEmpty && (
            <div className="empty-state">
              <div className="empty-icon">
                <Triangle size={36} />
              </div>
              <h2 className="empty-title">GeoGebra 指令生成器</h2>
              <p className="empty-sub">
                输入你的几何、代数或函数描述，AI 将自动生成 GeoGebra 命令和可嵌入的 HTML 课件。
              </p>
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
            <div className="stage">
              <div className="messages">
                {messages.map((msg) => (
                  <ChatMessage
                    key={msg.id}
                    role={msg.role}
                    content={msg.content}
                    timestamp={msg.timestamp}
                    html={msg.html}
                    result={activeConv?.result}
                    isLoading={msg.role === 'assistant' && isLoading && msg.id === messages[messages.length - 1]?.id}
                    onExecuteGGB={handleExecuteGGB}
                    onOpenHTMLPreview={handleOpenHTMLPreview}
                  />
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Input area */}
        <div className="input-area">
          <ChatInput
            value={input}
            onChange={handleInputChange}
            onSubmit={handleSubmit}
            onCancel={handleCancel}
            isLoading={isLoading}
          />
        </div>
      </main>

      {/* Settings panel */}
      <SettingsPanel
        isOpen={showSettings}
        onClose={() => setShowSettings(false)}
        settings={settings}
        onSave={handleSaveSettings}
      />

      {/* GeoGebra Runner (right panel) */}
      {rightPanel && (
        <GeogebraRunner
          html={rightPanel.html}
          ggbCode={rightPanel.ggbCode}
          svg={rightPanel.svg}
          activeTab={rightPanel.activeTab}
          onClose={handleCloseRightPanel}
        />
      )}
    </div>
  );
}

export default App;
