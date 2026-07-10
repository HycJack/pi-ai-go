import { useState, useCallback, useEffect, useRef } from 'react';
import Sidebar from './components/Sidebar';
import ChatArea from './components/ChatArea';
import SettingsPanel from './components/SettingsPanel';
import { Message, Conversation, Settings, DEFAULT_SETTINGS, getCurrentProvider } from './types';
import {
  StreamMessage, CancelStream,
  AgentMessage, GetSettings, SaveSettings,
  GetMemory, SetMemoryEntry, DeleteMemoryEntry, GetContextStats,
  GetConversations, SaveConversations, GetModels,
} from '../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';
import { RefreshOutlined } from './icons';

let streamIdCounter = 0;
function nextStreamId(): string {
  return `stream_${Date.now()}_${++streamIdCounter}`;
}

function App() {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeConversationId, setActiveConversationId] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [settings, setSettings] = useState<Settings>(DEFAULT_SETTINGS);
  const [settingsLoaded, setSettingsLoaded] = useState(false);
  const [speakingMessageId, setSpeakingMessageId] = useState<string | null>(null);
  const [contextStats, setContextStats] = useState('');
  const [memoryEntries, setMemoryEntries] = useState<{key: string; value: string; category?: string}[]>([]);
  const [showMemoryPanel, setShowMemoryPanel] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [toast, setToast] = useState<string | null>(null);
  const [models, setModels] = useState<{id: string; name: string; reasoning?: boolean; thinkingLevelMap?: Record<string, string>}[]>([]);

  const speechSynthesisRef = useRef<SpeechSynthesis | null>(null);
  const currentUtteranceRef = useRef<SpeechSynthesisUtterance | null>(null);

  const conversationsRef = useRef<Conversation[]>([]);
  useEffect(() => { conversationsRef.current = conversations; }, [conversations]);

  useEffect(() => {
    GetSettings().then((str) => {
      try {
        const s = JSON.parse(str) as Settings;
        // Go backend flattens embedded struct fields (TTSSettings, AgentSettings)
        // to top-level JSON keys. Use a wider type to access them safely.
        const raw = JSON.parse(str) as Record<string, any>;
        if (!s.providers || s.providers.length === 0) {
          setSettings({ ...DEFAULT_SETTINGS });
        } else {
          // Only take non-zero values from backend to preserve frontend defaults
          const merged: Settings = {
            ...DEFAULT_SETTINGS,
            providers: s.providers.map((p) => ({
              ...p,
              apiKeys: p.apiKeys ?? [],
            })),
            currentProviderIndex: s.currentProviderIndex,
            model: s.model || DEFAULT_SETTINGS.model,
            maxTokens: s.maxTokens || DEFAULT_SETTINGS.maxTokens,
            temperature: s.temperature || DEFAULT_SETTINGS.temperature,
            reasoning: s.reasoning || DEFAULT_SETTINGS.reasoning,
            ttsEnabled: s.ttsEnabled ?? DEFAULT_SETTINGS.ttsEnabled,
            ttsVoice: s.ttsVoice || DEFAULT_SETTINGS.ttsVoice,
            agentMode: s.agentMode ?? DEFAULT_SETTINGS.agentMode,
            agentSettings: {
              autoLearn: raw.autoLearn ?? s.agentSettings?.autoLearn ?? DEFAULT_SETTINGS.agentSettings.autoLearn,
              autoCompact: raw.autoCompact ?? s.agentSettings?.autoCompact ?? DEFAULT_SETTINGS.agentSettings.autoCompact,
              skillsDir: raw.skillsDir || s.agentSettings?.skillsDir || DEFAULT_SETTINGS.agentSettings.skillsDir,
            },
          };
          setSettings(merged);
        }
        setSettingsLoaded(true);
      } catch (e) { /* ignore */ }
    }).catch(() => {});
    GetConversations().then((str) => {
      try {
        const convs = JSON.parse(str) as Conversation[];
        if (convs.length > 0) {
          setConversations(convs);
          setActiveConversationId(convs[0].id);
        }
      } catch (e) { /* ignore */ }
    }).catch(() => {});
  }, []);

  useEffect(() => {
    const cp = getCurrentProvider(settings);
    if (!cp) return;
    const fetchModels = async () => {
      try {
        const list = await GetModels({
          provider: cp.type,
          baseUrl: cp.baseUrl,
          apiKey: cp.apiKey,
        });
        if (list && list.length > 0) {
          setModels(list);
          setSettings((prev) => prev.model ? prev : { ...prev, model: list[0].id });
        }
      } catch { /* ignore */ }
    };
    fetchModels();
  }, [settings.currentProviderIndex, settings.providers]);

  const saveConvsTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (saveConvsTimeoutRef.current) clearTimeout(saveConvsTimeoutRef.current);
    saveConvsTimeoutRef.current = setTimeout(() => {
      SaveConversations(JSON.stringify(conversations)).catch(() => {});
    }, 500);
    return () => {
      if (saveConvsTimeoutRef.current) clearTimeout(saveConvsTimeoutRef.current);
    };
  }, [conversations]);

  useEffect(() => {
    return () => {
      if (eventCleanupRef.current) {
        eventCleanupRef.current();
        eventCleanupRef.current = null;
      }
    };
  }, []);

  const currentStreamRef = useRef<{ streamId: string; convId: string; msgId: string } | null>(null);
  const sendingRef = useRef<string | null>(null);
  const eventCleanupRef = useRef<(() => void) | null>(null);

  const showToast = useCallback((msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 2000);
  }, []);

  const updateAssistantInConv = useCallback((convId: string, updater: (msg: Message) => Message) => {
    setConversations((prev) => prev.map((c) => {
      if (c.id !== convId) return c;
      const msgs = [...c.messages];
      for (let i = msgs.length - 1; i >= 0; i--) {
        if (msgs[i].role === 'assistant') {
          msgs[i] = updater(msgs[i]);
          break;
        }
      }
      return { ...c, messages: msgs };
    }));
  }, []);

  const registerStreamEvents = useCallback((streamId: string, convId: string, msgId: string) => {
    const handler = (eventName: string, handlerFn: (...args: any[]) => void) => {
      const wrapped = (...args: any[]) => {
        const cur = currentStreamRef.current;
        if (cur && cur.streamId === streamId && cur.convId === convId) {
          handlerFn(...args);
        }
      };
      EventsOn(eventName, wrapped);
      return () => EventsOff(eventName);
    };

    const cleanups: (() => void)[] = [];

    cleanups.push(handler('stream-thinking-delta', (delta: string) => {
      updateAssistantInConv(convId, (m) => ({ ...m, thinking: (m.thinking || '') + delta }));
    }));

    cleanups.push(handler('stream-tool-call-start', (data: string) => {
      try {
        const tc = JSON.parse(data);
        updateAssistantInConv(convId, (m) => ({
          ...m,
          toolCalls: [...(m.toolCalls || []), { id: tc.id, name: tc.name, arguments: '' }],
        }));
      } catch (e) { console.error(e); }
    }));

    cleanups.push(handler('stream-tool-call-delta', (delta: string) => {
      updateAssistantInConv(convId, (m) => {
        const tcs = [...(m.toolCalls || [])];
        if (tcs.length > 0) {
          tcs[tcs.length - 1] = { ...tcs[tcs.length - 1], arguments: tcs[tcs.length - 1].arguments + delta };
        }
        return { ...m, toolCalls: tcs };
      });
    }));

    cleanups.push(handler('stream-tool-call-end', (args: string) => {
      updateAssistantInConv(convId, (m) => {
        const tcs = [...(m.toolCalls || [])];
        if (tcs.length > 0) {
          tcs[tcs.length - 1] = { ...tcs[tcs.length - 1], arguments: args };
        }
        return { ...m, toolCalls: tcs };
      });
    }));

    cleanups.push(handler('stream-text-delta', (delta: string) => {
      updateAssistantInConv(convId, (m) => ({ ...m, content: m.content + delta }));
    }));

    cleanups.push(handler('stream-done', () => {
      updateAssistantInConv(convId, (m) => {
        if (!m.timestamp) {
          return { ...m, timestamp: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) };
        }
        return m;
      });
      setIsLoading(false);
      currentStreamRef.current = null;
      sendingRef.current = null;
      GetContextStats().then((s) => setContextStats(s)).catch(() => {});
    }));

    cleanups.push(handler('stream-error', (error: string) => {
      updateAssistantInConv(convId, (m) => {
        if (!m.content) {
          return { ...m, content: error, timestamp: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) };
        }
        return m;
      });
      setIsLoading(false);
      currentStreamRef.current = null;
      sendingRef.current = null;
    }));

    cleanups.push(handler('agent-text-delta', (delta: string) => {
      updateAssistantInConv(convId, (m) => ({ ...m, content: m.content + delta }));
    }));

    cleanups.push(handler('agent-thinking-delta', (delta: string) => {
      updateAssistantInConv(convId, (m) => ({ ...m, thinking: (m.thinking || '') + delta }));
    }));

    cleanups.push(handler('agent-tool-call-start', (data: string) => {
      try {
        const tc = JSON.parse(data);
        updateAssistantInConv(convId, (m) => ({
          ...m,
          toolCalls: [...(m.toolCalls || []), { id: tc.id, name: tc.name, arguments: '' }],
        }));
      } catch (e) { console.error(e); }
    }));

    cleanups.push(handler('agent-tool-call-delta', (delta: string) => {
      updateAssistantInConv(convId, (m) => {
        const tcs = [...(m.toolCalls || [])];
        if (tcs.length > 0) {
          tcs[tcs.length - 1] = { ...tcs[tcs.length - 1], arguments: tcs[tcs.length - 1].arguments + delta };
        }
        return { ...m, toolCalls: tcs };
      });
    }));

    cleanups.push(handler('agent-tool-call-end', (args: string) => {
      updateAssistantInConv(convId, (m) => {
        const tcs = [...(m.toolCalls || [])];
        if (tcs.length > 0) {
          tcs[tcs.length - 1] = { ...tcs[tcs.length - 1], arguments: args };
        }
        return { ...m, toolCalls: tcs };
      });
    }));

    cleanups.push(handler('agent-done', () => {
      updateAssistantInConv(convId, (m) => {
        if (!m.timestamp) {
          return { ...m, timestamp: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) };
        }
        return m;
      });
      setIsLoading(false);
      currentStreamRef.current = null;
      sendingRef.current = null;
      GetContextStats().then((s) => setContextStats(s)).catch(() => {});
      refreshMemory();
    }));

    cleanups.push(handler('agent-error', (error: string) => {
      updateAssistantInConv(convId, (m) => {
        if (!m.content) {
          return { ...m, content: `Error: ${error}`, timestamp: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) };
        }
        return m;
      });
      setIsLoading(false);
      currentStreamRef.current = null;
      sendingRef.current = null;
    }));

    return () => {
      cleanups.forEach((fn) => fn());
    };
  }, [updateAssistantInConv]);

  const refreshMemory = useCallback(() => {
    GetMemory().then((str) => {
      try {
        const entries = JSON.parse(str);
        setMemoryEntries(entries);
      } catch (e) { /* ignore */ }
    }).catch(() => {});
  }, []);

  const createNewConversation = useCallback(() => {
    const newConv: Conversation = {
      id: Date.now().toString(),
      title: 'New Chat',
      messages: [],
      timestamp: new Date().toLocaleDateString(),
    };
    setConversations((prev) => [newConv, ...prev]);
    setActiveConversationId(newConv.id);
    return newConv.id;
  }, []);

  const selectConversation = useCallback((id: string) => {
    setActiveConversationId(id);
  }, []);

  const deleteConversation = useCallback((id: string) => {
    setConversations((prev) => prev.filter((c) => c.id !== id));
    if (activeConversationId === id) {
      setActiveConversationId(null);
    }
  }, [activeConversationId]);

  const renameConversation = useCallback((id: string, title: string) => {
    setConversations((prev) => prev.map((c) => c.id === id ? { ...c, title } : c));
  }, []);

  const stopSpeaking = useCallback(() => {
    if (currentUtteranceRef.current) {
      speechSynthesisRef.current?.cancel();
      currentUtteranceRef.current = null;
    }
    setSpeakingMessageId(null);
  }, []);

  const speakText = useCallback((text: string, messageId: string) => {
    stopSpeaking();
    if ('speechSynthesis' in window) {
      speechSynthesisRef.current = window.speechSynthesis;
      const utterance = new SpeechSynthesisUtterance(text);
      utterance.lang = settings.ttsVoice || 'zh-CN';
      utterance.onend = () => setSpeakingMessageId(null);
      utterance.onerror = () => setSpeakingMessageId(null);
      currentUtteranceRef.current = utterance;
      setSpeakingMessageId(messageId);
      speechSynthesisRef.current.speak(utterance);
    }
  }, [settings.ttsVoice, stopSpeaking]);

  const generateResponse = useCallback(async (message: string, targetConvId: string, responseMessageId: string, directHistory?: {role: string; content: string}[]) => {
    if (sendingRef.current === targetConvId) return;
    sendingRef.current = targetConvId;
    setIsLoading(true);
    stopSpeaking();

    const newMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content: message,
      timestamp: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }),
    };

    const assistantPlaceholder: Message = {
      id: responseMessageId,
      role: 'assistant',
      content: '',
      timestamp: '',
      thinking: '',
      toolCalls: [],
    };

    setConversations((prev) =>
      prev.map((c) =>
        c.id === targetConvId
          ? {
              ...c,
              messages: [...c.messages, newMessage, assistantPlaceholder],
              title: c.messages.length === 0 ? message.slice(0, 20) + (message.length > 20 ? '...' : '') : c.title,
              timestamp: new Date().toLocaleDateString(),
            }
          : c
      )
    );

    if (eventCleanupRef.current) {
      eventCleanupRef.current();
      eventCleanupRef.current = null;
    }

    const streamId = nextStreamId();
    currentStreamRef.current = { streamId, convId: targetConvId, msgId: responseMessageId };
    const cleanup = registerStreamEvents(streamId, targetConvId, responseMessageId);
    eventCleanupRef.current = cleanup;

    const conv = conversationsRef.current.find((c) => c.id === targetConvId);
    const historyMessages = directHistory ?? (conv?.messages || []).map((m) => {
      const base: { role: string; content: string; tool_calls?: any[]; tool_call_id?: string; tool_name?: string } = {
        role: m.role,
        content: m.content,
      };
      if (m.role === 'assistant' && m.toolCalls && m.toolCalls.length > 0) {
        base.tool_calls = m.toolCalls;
      }
      return base;
    });

    const cp = getCurrentProvider(settings);
    const providerType = cp?.type || 'openai';
    const apiKey = cp?.apiKey || '';
    const baseUrl = cp?.baseUrl || '';

    try {
      if (settings.agentMode) {
        const req = JSON.stringify({
          message,
          messages: historyMessages,
          provider: providerType,
          apiKey,
          baseUrl,
          model: settings.model,
          maxTokens: settings.maxTokens,
          temperature: settings.temperature,
          reasoning: settings.reasoning,
        });
        await AgentMessage(req);
      } else {
        await StreamMessage({
          message,
          messages: historyMessages,
          provider: providerType,
          apiKey,
          baseUrl,
          model: settings.model,
          maxTokens: settings.maxTokens,
          temperature: settings.temperature,
          reasoning: settings.reasoning,
        });
      }
    } catch (error) {
      console.error('Error:', error);
      setConversations((prev) =>
        prev.map((c) =>
          c.id === targetConvId
            ? {
                ...c,
                messages: c.messages.map((m) =>
                  m.id === responseMessageId
                    ? { ...m, content: 'Sorry, an error occurred.', timestamp: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) }
                    : m
                ),
              }
            : c
        )
      );
      setIsLoading(false);
      currentStreamRef.current = null;
      sendingRef.current = null;
    }
  }, [settings, registerStreamEvents, stopSpeaking]);

  const handleSendMessage = useCallback(
    (message: string) => {
      let cid = activeConversationId;
      if (!cid) {
        const newConv: Conversation = {
          id: Date.now().toString(),
          title: message.slice(0, 20) + (message.length > 20 ? '...' : ''),
          messages: [],
          timestamp: new Date().toLocaleDateString(),
        };
        setConversations((prev) => [newConv, ...prev]);
        setActiveConversationId(newConv.id);
        const msgId = (Date.now() + 1).toString();
        generateResponse(message, newConv.id, msgId, []);
        return;
      }
      const msgId = (Date.now() + 1).toString();
      generateResponse(message, cid, msgId);
    },
    [activeConversationId, generateResponse]
  );

  const handleRegenerate = useCallback((message: string) => {
    if (isLoading) {
      CancelStream().catch(() => {});
      if (eventCleanupRef.current) {
        eventCleanupRef.current();
        eventCleanupRef.current = null;
      }
      setIsLoading(false);
      currentStreamRef.current = null;
    }
    const cid = activeConversationId;
    if (!cid) return;
    const msgId = (Date.now() + 1).toString();
    generateResponse(message, cid, msgId);
  }, [activeConversationId, isLoading, generateResponse]);

  const handleDeleteMessage = useCallback((messageId: string) => {
    const cid = activeConversationId;
    if (!cid) return;
    setConversations((prev) =>
      prev.map((c) =>
        c.id === cid
          ? { ...c, messages: c.messages.filter((m) => m.id !== messageId) }
          : c
      )
    );
  }, [activeConversationId]);

  const handleCancel = useCallback(() => {
    CancelStream().catch(() => {});
    if (eventCleanupRef.current) {
      eventCleanupRef.current();
      eventCleanupRef.current = null;
    }
    setIsLoading(false);
    currentStreamRef.current = null;
    sendingRef.current = null;
    showToast('已停止生成');
  }, [showToast]);

  const handleSaveSettings = useCallback((newSettings: Settings) => {
    setSettings(newSettings);
    setIsSettingsOpen(false);
    showToast('设置已保存');
  }, [showToast]);

  const activeConversation = conversations.find((c) => c.id === activeConversationId);

  // Get working dir from the current provider's base URL or a static field
  const workingDir = '';

  return (
    <div className={`app-shell ${sidebarCollapsed ? 'sidebar-collapsed' : ''}`}>
      <Sidebar
        conversations={conversations}
        activeConversation={activeConversationId}
        workingDir={workingDir}
        collapsed={sidebarCollapsed}
        onToggleCollapse={() => setSidebarCollapsed((v) => !v)}
        onSelectConversation={selectConversation}
        onCreateNewConversation={createNewConversation}
        onDeleteConversation={deleteConversation}
        onRenameConversation={renameConversation}
        onOpenSettings={() => setIsSettingsOpen(true)}
      />

      <div className="main-frame">
        {activeConversation ? (
          <>
            <div className="topbar">
              <div className="topbar-left">
                <div className="breadcrumb">
                  <span className="crumb-root">Conversations</span>
                  <span className="crumb-sep">/</span>
                  <span className="crumb-current">{activeConversation.title}</span>
                </div>
                {contextStats && (
                  <span className="bridge-status" title="Context stats">
                    <span className="status-dot green" />
                    {contextStats.split('\n')[0]}
                  </span>
                )}
              </div>
              <div className="topbar-right">
                {isLoading && (
                  <span className="status-spinner" />
                )}
              </div>
            </div>

            <ChatArea
              messages={activeConversation.messages}
              isLoading={isLoading}
              workingDir={workingDir}
              onSendMessage={handleSendMessage}
              onStop={handleCancel}
              onSpeak={speakText}
              onStopSpeak={stopSpeaking}
              speakingMessageId={speakingMessageId}
              models={models}
              currentModel={settings.model}
              currentThinkingLevel={settings.reasoning}
              onModelChange={(model) => setSettings((prev) => ({ ...prev, model }))}
              onThinkingLevelChange={(level) => setSettings((prev) => ({ ...prev, reasoning: level }))}
            />
          </>
        ) : (
          <ChatArea
            messages={[]}
            isLoading={isLoading}
            workingDir={workingDir}
            onSendMessage={handleSendMessage}
            onStop={handleCancel}
            onSpeak={speakText}
            onStopSpeak={stopSpeaking}
            speakingMessageId={speakingMessageId}
            models={models}
            currentModel={settings.model}
            currentThinkingLevel={settings.reasoning}
            onModelChange={(model) => setSettings((prev) => ({ ...prev, model }))}
            onThinkingLevelChange={(level) => setSettings((prev) => ({ ...prev, reasoning: level }))}
          />
        )}
      </div>

      <SettingsPanel
        isOpen={isSettingsOpen}
        onClose={() => setIsSettingsOpen(false)}
        currentSettings={settings}
        onSave={handleSaveSettings}
      />

      {toast && <div className="app-toast">{toast}</div>}
    </div>
  );
}

export default App;
