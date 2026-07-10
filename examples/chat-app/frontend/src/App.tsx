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

// 生成唯一流 ID，用于区分不同请求的流式事件
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

  // 启动时加载设置和对话
  useEffect(() => {
    GetSettings().then((str) => {
      try {
        const s = JSON.parse(str) as Settings;
        // 兼容旧设置格式：若 s.providers 不存在则将旧字段迁移
        if (!s.providers || s.providers.length === 0) {
          setSettings({ ...DEFAULT_SETTINGS });
        } else {
          setSettings({ ...DEFAULT_SETTINGS, ...s });
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

  // 加载模型列表（在当前 provider 变化时）
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
          // 如果没有设置 model，自动选取第一个
          setSettings((prev) => prev.model ? prev : { ...prev, model: list[0].id });
        }
      } catch {
        // ignore
      }
    };
    fetchModels();
  }, [settings.currentProviderIndex, settings.providers]);

  // 保存记忆

  // 对话变化时自动保存
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

  // 组件卸载时清理事件监听器
  useEffect(() => {
    return () => {
      if (eventCleanupRef.current) {
        eventCleanupRef.current();
        eventCleanupRef.current = null;
      }
    };
  }, []);

  // 当前流的元数据：{ convId, msgId, role: 'stream' | 'agent' }
  const currentStreamRef = useRef<{ streamId: string; convId: string; msgId: string } | null>(null);
  const sendingRef = useRef<string | null>(null); // stores convId of active stream, null if none
  const eventCleanupRef = useRef<(() => void) | null>(null); // stores cleanup function for event listeners

  // ─── Toast ───
  const showToast = useCallback((msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 2000);
  }, []);

  // ─── 辅助：更新指定会话的最新助手消息 ───
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

  // ─── 注册流事件（带流 ID 隔离） ───
  const registerStreamEvents = useCallback((streamId: string, convId: string, msgId: string) => {
    const handler = (eventName: string, handlerFn: (...args: any[]) => void) => {
      const wrapped = (...args: any[]) => {
        // 只有当前流 ID 匹配时才处理
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

    // Agent 事件
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

  // ─── 加载 memory ───
  const refreshMemory = useCallback(() => {
    GetMemory().then((str) => {
      try {
        const entries = JSON.parse(str);
        setMemoryEntries(entries);
      } catch (e) { /* ignore */ }
    }).catch(() => {});
  }, []);

  // ─── 发送消息 ───
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
    // 同一个会话正在流式输出时不允许重复发送
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

    // 创建单个占位消息
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

    // 清理旧的事件监听，防止内存泄漏
    if (eventCleanupRef.current) {
      eventCleanupRef.current();
      eventCleanupRef.current = null;
    }

    // 注册流事件
    const streamId = nextStreamId();
    currentStreamRef.current = { streamId, convId: targetConvId, msgId: responseMessageId };
    const cleanup = registerStreamEvents(streamId, targetConvId, responseMessageId);
    eventCleanupRef.current = cleanup;

    // 获取历史消息（排除当前正在发送的这一条和占位消息）
    const conv = conversationsRef.current.find((c) => c.id === targetConvId);
    const historyMessages = directHistory ?? (conv?.messages || []).map((m) => {
      const base: { role: string; content: string; tool_calls?: any[]; tool_call_id?: string; tool_name?: string } = {
        role: m.role,
        content: m.content,
      };
      // Agent 模式下保留 toolCalls 信息
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

    // 不在 try-catch 后立即 cleanup，因为后端流式事件是异步发送的，
    // 事件处理器需要保持活动直到 stream-done/agent-done 触发。
    // 下次发送请求时 eventCleanupRef 会自动清理旧的监听器。
  }, [settings, registerStreamEvents, stopSpeaking]);

  const handleSendMessage = useCallback(
    (message: string) => {
      let cid = activeConversationId;
      if (!cid) {
        // 新建会话时直接创建，无需 setTimeout
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

  // ─── 重新生成 ───
  const handleRegenerate = useCallback((message: string) => {
    // 取消当前流
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

  // ─── 取消流 ───
  const handleCancel = useCallback(() => {
    CancelStream().catch(() => {});
    // 清理事件监听器，防止内存泄漏
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

  return (
    <div className="flex h-screen bg-slate-900 overflow-hidden">
      <Sidebar
        conversations={conversations}
        activeConversation={activeConversationId}
        collapsed={sidebarCollapsed}
        onToggleCollapse={() => setSidebarCollapsed((v) => !v)}
        onSelectConversation={selectConversation}
        onCreateNewConversation={createNewConversation}
        onDeleteConversation={deleteConversation}
        onRenameConversation={renameConversation}
        onOpenSettings={() => setIsSettingsOpen(true)}
      />

      <div className="flex-1 flex flex-col overflow-hidden">
        {activeConversation ? (
          <>
            <div className="h-14 border-b border-slate-700 flex items-center px-4 bg-slate-900 justify-between">
              <div className="flex items-center gap-3 min-w-0 flex-1">
                <h2 className="text-lg font-medium text-white truncate">{activeConversation.title}</h2>
                {contextStats && (
                  <span className="text-xs text-slate-500 hidden md:inline-block truncate max-w-[200px]">
                    {contextStats.split('\n')[0]}
                  </span>
                )}
              </div>
              <div className="flex items-center gap-2">
                {/* Agent 模式切换 */}
                <button
                  onClick={() => setSettings((prev) => {
                    const next = { ...prev, agentMode: !prev.agentMode };
                    SaveSettings(JSON.stringify(next)).catch(() => {});
                    return next;
                  })}
                  className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${
                    settings.agentMode
                      ? 'bg-purple-600 text-white'
                      : 'bg-slate-700 text-slate-400 hover:bg-slate-600'
                  }`}
                  title={settings.agentMode ? 'Agent mode (tools enabled)' : 'Chat mode'}
                >
                  {settings.agentMode ? 'Agent' : 'Chat'}
                </button>
                {/* 记忆按钮 */}
                <button
                  onClick={() => {
                    if (!showMemoryPanel) refreshMemory();
                    setShowMemoryPanel(!showMemoryPanel);
                  }}
                  className="px-3 py-1.5 text-xs font-medium rounded-lg bg-slate-700 text-slate-400 hover:bg-slate-600 transition-colors"
                >
                  Memory
                </button>
              </div>
            </div>
            <div className="flex flex-1 overflow-hidden">
              <ChatArea
                messages={activeConversation.messages}
                isLoading={isLoading}
                onSendMessage={handleSendMessage}
                onSpeak={speakText}
                onStopSpeak={stopSpeaking}
                speakingMessageId={speakingMessageId}
                onCancel={handleCancel}
                onRegenerate={handleRegenerate}
                onDeleteMessage={handleDeleteMessage}
                models={models}
                currentModel={settings.model}
                currentThinkingLevel={settings.reasoning}
                onModelChange={(m) => setSettings(prev => ({ ...prev, model: m }))}
                onThinkingLevelChange={(l) => setSettings(prev => ({ ...prev, reasoning: l }))}
              />
              {/* 记忆侧边栏 */}
              {showMemoryPanel && (
                <MemorySidebar
                  entries={memoryEntries}
                  onRefresh={refreshMemory}
                  onDelete={(key) => {
                    DeleteMemoryEntry(key).then(() => refreshMemory()).catch(() => {});
                  }}
                  onAdd={(key, value) => {
                    SetMemoryEntry(key, value, 'manual').then(() => refreshMemory()).catch(() => {});
                  }}
                  onClose={() => setShowMemoryPanel(false)}
                />
              )}
            </div>
          </>
        ) : (
          <div className="flex-1 flex items-center justify-center bg-slate-900">
            <div className="text-center">
              <h1 className="text-2xl font-bold text-white mb-2">PI AI Chat</h1>
              <p className="text-slate-400">Select a conversation or start a new one</p>
            </div>
          </div>
        )}
      </div>

      <SettingsPanel
        isOpen={isSettingsOpen}
        onClose={() => setIsSettingsOpen(false)}
        currentSettings={settings}
        onSave={handleSaveSettings}
      />

      {/* Toast 提示 */}
      {toast && (
        <div className="fixed bottom-24 left-1/2 -translate-x-1/2 z-50 px-4 py-2 bg-slate-700 border border-slate-600 rounded-lg shadow-lg">
          <p className="text-sm text-white">{toast}</p>
        </div>
      )}
    </div>
  );
}

// ─── MemorySidebar ───
function MemorySidebar({ entries, onRefresh, onDelete, onAdd, onClose }: {
  entries: {key: string; value: string; category?: string}[];
  onRefresh: () => void;
  onDelete: (key: string) => void;
  onAdd: (key: string, value: string) => void;
  onClose: () => void;
}) {
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');

  return (
    <div className="w-80 border-l border-slate-700 bg-slate-850 flex flex-col overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3 border-b border-slate-700">
        <h3 className="text-sm font-medium text-white">Memory</h3>
        <div className="flex items-center gap-2">
          <button onClick={onRefresh} className="text-xs text-slate-400 hover:text-white transition-colors">Refresh</button>
          <button onClick={onClose} className="text-xs text-slate-400 hover:text-white transition-colors">Close</button>
        </div>
      </div>
      <div className="flex-1 overflow-y-auto p-3 space-y-1">
        {entries.length === 0 && (
          <p className="text-xs text-slate-500 text-center py-4">No memory entries yet</p>
        )}
        {entries.map((entry, i) => (
          <div key={i} className="group flex items-start justify-between p-2 rounded hover:bg-slate-700/50 transition-colors">
            <div className="min-w-0 flex-1">
              <div className="text-xs font-medium text-slate-300">{entry.key}</div>
              <div className="text-xs text-slate-500 truncate">{entry.value}</div>
            </div>
            <button
              onClick={() => onDelete(entry.key)}
              className="opacity-0 group-hover:opacity-100 text-xs text-red-400 hover:text-red-300 transition-all"
            >
              del
            </button>
          </div>
        ))}
      </div>
      <div className="p-3 border-t border-slate-700 space-y-2">
        <input
          value={newKey}
          onChange={(e) => setNewKey(e.target.value)}
          placeholder="Key (e.g. user.name)"
          className="w-full px-3 py-1.5 text-xs bg-slate-700 border border-slate-600 rounded text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
        />
        <input
          value={newValue}
          onChange={(e) => setNewValue(e.target.value)}
          placeholder="Value"
          className="w-full px-3 py-1.5 text-xs bg-slate-700 border border-slate-600 rounded text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
        />
        <button
          onClick={() => {
            if (newKey && newValue) {
              onAdd(newKey, newValue);
              setNewKey('');
              setNewValue('');
            }
          }}
          className="w-full py-1.5 text-xs bg-blue-600 text-white rounded hover:bg-blue-500 transition-colors"
        >
          Add Entry
        </button>
      </div>
    </div>
  );
}

export default App;
