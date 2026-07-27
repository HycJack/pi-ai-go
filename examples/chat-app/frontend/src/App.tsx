import { useState, useCallback, useEffect, useRef, useMemo } from 'react';
import Sidebar from './components/Sidebar';
import ChatArea from './components/ChatArea';
import SettingsPanel from './components/SettingsPanel';
import FileTabBar, { type FileTab } from './components/FileTabBar';
import FilePreview from './components/FilePreview';
import { OnboardingApp } from './components/generic-onboarding';
import type { OnboardingResult } from './components/generic-onboarding';
import { Message, Conversation, Settings, DEFAULT_SETTINGS, getCurrentProvider, PROVIDER_TYPES, getProviderTypeName, ImageAttachment } from './types';
import { log } from './utils/logger';
import {
  StreamMessage, CancelStream,
  AgentMessage, GetSettings, SaveSettings,
  GetMemory, SetMemoryEntry, DeleteMemoryEntry, GetContextStats,
  GetConversations, SaveConversation, DeleteConversation, GetModels,
} from '../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';
import { RefreshOutlined } from './icons';
import { useT, setLocale, loadLocaleFromStorage, saveLocaleToStorage } from './i18n';

let streamIdCounter = 0;
function nextStreamId(): string {
  return `stream_${Date.now()}_${++streamIdCounter}`;
}
let msgIdCounter = 0;
function nextMsgId(): string {
  return `msg_${Date.now()}_${++msgIdCounter}`;
}
function nextConvId(): string {
  return `conv_${Date.now()}_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

function serializeSettings(settings: Settings): string {
  const { agentSettings, ...rest } = settings;
  return JSON.stringify({ ...rest, ...agentSettings });
}

function App() {
  const t = useT();
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
  const [fileTabs, setFileTabs] = useState<FileTab[]>([]);
  const [activeFileTabId, setActiveFileTabId] = useState<string | null>(null);
  const [rightPanelOpen, setRightPanelOpen] = useState(false);
  const [toast, setToast] = useState<string | null>(null);
  const [models, setModels] = useState<{id: string; name: string; reasoning?: boolean; thinkingLevelMap?: Record<string, string>}[]>([]);
  const [showOnboarding, setShowOnboarding] = useState(false);

  const speechSynthesisRef = useRef<SpeechSynthesis | null>(null);
  const currentUtteranceRef = useRef<SpeechSynthesisUtterance | null>(null);

  const conversationsRef = useRef<Conversation[]>([]);
  useEffect(() => { conversationsRef.current = conversations; }, [conversations]);

  useEffect(() => {
    // 初始化语言（从 localStorage 恢复）
    loadLocaleFromStorage();
    GetSettings().then((str) => {
      try {
        const s = JSON.parse(str) as Settings;
        // Go backend flattens embedded struct fields (TTSSettings, AgentSettings)
        // to top-level JSON keys. Use a wider type to access them safely.
        const raw = JSON.parse(str) as Record<string, any>;
        // 应用保存的语言偏好
        const locale = (s.locale || raw.locale || 'zh') as 'zh' | 'en';
        setLocale(locale);
        if (!s.providers || s.providers.length === 0) {
          setSettings({ ...DEFAULT_SETTINGS });
          setShowOnboarding(true);
        } else {
          // Only take non-zero values from backend to preserve frontend defaults
          const merged: Settings = {
            ...DEFAULT_SETTINGS,
            providers: s.providers.map((p) => ({
              ...p,
              apiKeys: p.apiKeys ?? [],
            })),
            currentProviderIndex: s.currentProviderIndex ?? DEFAULT_SETTINGS.currentProviderIndex,
            model: s.model || DEFAULT_SETTINGS.model,
            maxTokens: s.maxTokens ?? DEFAULT_SETTINGS.maxTokens,
            temperature: s.temperature ?? DEFAULT_SETTINGS.temperature,
            reasoning: s.reasoning || DEFAULT_SETTINGS.reasoning,
            workingDir: s.workingDir || DEFAULT_SETTINGS.workingDir,
            ttsEnabled: s.ttsEnabled ?? DEFAULT_SETTINGS.ttsEnabled,
            ttsVoice: s.ttsVoice || DEFAULT_SETTINGS.ttsVoice,
            agentMode: s.agentMode ?? DEFAULT_SETTINGS.agentMode,
            locale: s.locale || raw.locale || DEFAULT_SETTINGS.locale,
            agentSettings: {
              autoLearn: raw.autoLearn ?? s.agentSettings?.autoLearn ?? DEFAULT_SETTINGS.agentSettings.autoLearn,
              autoCompact: raw.autoCompact ?? s.agentSettings?.autoCompact ?? DEFAULT_SETTINGS.agentSettings.autoCompact,
              skillsDir: raw.skillsDir ?? s.agentSettings?.skillsDir ?? DEFAULT_SETTINGS.agentSettings.skillsDir,
            },
          };
          setSettings(merged);
          // Trigger onboarding if the current provider has no API key
          // (unless it's a local-only provider like Ollama)
          const cp = getCurrentProvider(merged);
          // Trigger onboarding only if this is a fresh install:
          // - no API key or API key pool configured
          // - AND no existing conversations (i.e. not a returning user)
          const isLocal = cp?.type === 'ollama';
          const hasKey = !!(cp?.apiKey || (cp?.apiKeys && cp.apiKeys.length > 0));
          if (cp && !isLocal && !hasKey && !merged.locale) {
            setShowOnboarding(true);
          }
        }
        setSettingsLoaded(true);
      } catch (e) { /* ignore */ }
    }).catch(() => {});
    GetConversations().then((str) => {
      try {
        const convs = JSON.parse(str) as Conversation[];
        if (convs.length > 0) {
          setConversations(convs);
          // 恢复上次活跃的对话 ID（如果还存在），否则选第一个
          const savedActiveId = (() => {
            try { return localStorage.getItem('pi-ai:activeConversationId'); } catch { return null; }
          })();
          const targetId = savedActiveId && convs.some((c) => c.id === savedActiveId)
            ? savedActiveId
            : convs[0].id;
          setActiveConversationId(targetId);
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
          // Auto-select first model only if user hasn't chosen one yet
          setSettings((prev) => prev.model ? prev : { ...prev, model: list[0].id });
        }
      } catch { /* ignore */ }
    };
    fetchModels();
  }, [settings.currentProviderIndex, settings.providers]);

  const settingsSaveTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (!settingsLoaded) return;
    if (settingsSaveTimeoutRef.current) clearTimeout(settingsSaveTimeoutRef.current);
    settingsSaveTimeoutRef.current = setTimeout(() => {
      SaveSettings(serializeSettings(settings)).catch(() => {});
    }, 300);
    return () => {
      if (settingsSaveTimeoutRef.current) clearTimeout(settingsSaveTimeoutRef.current);
    };
  }, [settings, settingsLoaded]);

  const initialLoadRef = useRef(true);
  const saveConvsTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    // Skip save on initial load to avoid rewriting conversation files
    // with potentially different JSON formatting on every startup.
    if (initialLoadRef.current) {
      initialLoadRef.current = false;
      return;
    }
    if (saveConvsTimeoutRef.current) clearTimeout(saveConvsTimeoutRef.current);
    saveConvsTimeoutRef.current = setTimeout(() => {
      // Save each conversation to its own file
      conversations.forEach((c) => {
        SaveConversation(c.id, JSON.stringify(c)).catch(() => {});
      });
    }, 500);
    return () => {
      if (saveConvsTimeoutRef.current) clearTimeout(saveConvsTimeoutRef.current);
    };
  }, [conversations]);

  // 持久化当前活跃对话 ID，以便下次启动时恢复
  useEffect(() => {
    try {
      if (activeConversationId) {
        localStorage.setItem('pi-ai:activeConversationId', activeConversationId);
      } else {
        localStorage.removeItem('pi-ai:activeConversationId');
      }
    } catch { /* ignore */ }
  }, [activeConversationId]);

  useEffect(() => {
    const onError = (event: ErrorEvent) => console.error('[Runtime error]', event.message, event.error?.stack);
    const onUnhandledRejection = (event: PromiseRejectionEvent) => console.error('[Unhandled rejection]', String(event.reason), event.reason?.stack);
    window.addEventListener('error', onError);
    window.addEventListener('unhandledrejection', onUnhandledRejection);
    return () => {
      window.removeEventListener('error', onError);
      window.removeEventListener('unhandledrejection', onUnhandledRejection);
    };
  }, []);

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
    // 全局 currentStreamRef 的 convId 必须匹配，防止事件错配到已切换的对话
    const cur = currentStreamRef.current;
    if (!cur || cur.convId !== convId) return;
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
        if (!m.content) {
          log.warn('[frontend] stream-done but content is empty', { convId, msgId: m.id });
          return {
            ...m,
            content: '(模型未返回任何内容，请检查模型设置或重试)',
            timestamp: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }),
          };
        }
        log.info('[frontend] stream-done, content length=' + m.content.length);
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
      log.error('[frontend] stream-error', { error, convId });
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
      updateAssistantInConv(convId, (m) => {
        const steps = [...(m.steps || [])];
        const last = steps[steps.length - 1];

        // Check if there is a running tool call — if so, this text delta
        // is an intermediate utterance during tool execution, not the final
        // assistant reply. Append to steps only, not to content.
        const hasRunningToolCall = steps.some(
          (s) => s.type === 'tool_call' && s.status === 'running',
        );

        if (last && last.type === 'text') {
          steps[steps.length - 1] = { ...last, content: last.content + delta };
        } else {
          steps.push({ type: 'text', content: delta });
        }

        if (hasRunningToolCall) {
          // Intermediate text — do not append to content.
          return { ...m, steps };
        }
        return { ...m, content: m.content + delta, steps };
      });
    }));

    cleanups.push(handler('agent-thinking-delta', (delta: string) => {
      updateAssistantInConv(convId, (m) => {
        const steps = [...(m.steps || [])];
        const last = steps[steps.length - 1];
        if (last && last.type === 'thinking') {
          steps[steps.length - 1] = { ...last, content: last.content + delta };
        } else {
          steps.push({ type: 'thinking', content: delta });
        }
        return { ...m, thinking: (m.thinking || '') + delta, steps };
      });
    }));

    cleanups.push(handler('agent-tool-call-start', (data: string) => {
      try {
        const tc = JSON.parse(data);
        updateAssistantInConv(convId, (m) => {
          const steps = [...(m.steps || [])];
          steps.push({ type: 'tool_call', content: '', toolName: tc.name, toolCallId: tc.id, status: 'running' });
          return {
            ...m,
            toolCalls: [...(m.toolCalls || []), { id: tc.id, name: tc.name, arguments: '' }],
            steps,
          };
        });
      } catch (e) { console.error(e); }
    }));

    cleanups.push(handler('agent-tool-call-delta', (delta: string) => {
      updateAssistantInConv(convId, (m) => {
        const tcs = [...(m.toolCalls || [])];
        if (tcs.length > 0) {
          tcs[tcs.length - 1] = { ...tcs[tcs.length - 1], arguments: tcs[tcs.length - 1].arguments + delta };
        }
        // Also update the last tool_call step.
        const steps = [...(m.steps || [])];
        for (let i = steps.length - 1; i >= 0; i--) {
          if (steps[i].type === 'tool_call' && steps[i].status === 'running') {
            steps[i] = { ...steps[i], content: steps[i].content + delta };
            break;
          }
        }
        return { ...m, toolCalls: tcs, steps };
      });
    }));

    cleanups.push(handler('agent-tool-call-end', (args: string) => {
      updateAssistantInConv(convId, (m) => {
        const tcs = [...(m.toolCalls || [])];
        if (tcs.length > 0) {
          tcs[tcs.length - 1] = { ...tcs[tcs.length - 1], arguments: args };
        }
        const steps = [...(m.steps || [])];
        for (let i = steps.length - 1; i >= 0; i--) {
          if (steps[i].type === 'tool_call' && steps[i].status === 'running') {
            steps[i] = { ...steps[i], content: args };
            break;
          }
        }
        return { ...m, toolCalls: tcs, steps };
      });
    }));

    cleanups.push(handler('agent-tool-result', (result: string) => {
      updateAssistantInConv(convId, (m) => {
        const steps = [...(m.steps || [])];
        // Mark the last running tool_call as done, and push a tool_result step.
        for (let i = steps.length - 1; i >= 0; i--) {
          if (steps[i].type === 'tool_call' && steps[i].status === 'running') {
            steps[i] = { ...steps[i], status: 'done' };
            break;
          }
        }
        steps.push({ type: 'tool_result', content: result });
        return { ...m, steps };
      });
    }));

    cleanups.push(handler('agent-tool-exec-end', (data: string) => {
      updateAssistantInConv(convId, (m) => {
        const steps = [...(m.steps || [])];
        for (let i = steps.length - 1; i >= 0; i--) {
          if (steps[i].type === 'tool_call' && steps[i].status === 'running') {
            try {
              const parsed = JSON.parse(data);
              steps[i] = { ...steps[i], status: parsed.success === false ? 'error' : 'done' };
            } catch {
              steps[i] = { ...steps[i], status: 'done' };
            }
            break;
          }
        }
        return { ...m, steps };
      });
    }));

    cleanups.push(handler('agent-done', () => {
      updateAssistantInConv(convId, (m) => {
        if (!m.content) {
          log.warn('[frontend] agent-done but content is empty', { convId, msgId: m.id });
          return {
            ...m,
            content: '(模型未返回任何内容，请检查模型设置或重试)',
            timestamp: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }),
          };
        }
        log.info('[frontend] agent-done, content length=' + m.content.length);
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
      log.error('[frontend] agent-error', { error, convId });
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
      id: nextConvId(),
      title: 'New Chat',
      messages: [],
      timestamp: new Date().toLocaleDateString(),
    };
    setConversations((prev) => [newConv, ...prev]);
    setActiveConversationId(newConv.id);
    return newConv.id;
  }, []);

  const selectConversation = useCallback((id: string) => {
    // 切换对话时，取消当前正在进行的流，防止事件错配
    if (eventCleanupRef.current) {
      eventCleanupRef.current();
      eventCleanupRef.current = null;
    }
    CancelStream().catch(() => {});
    setIsLoading(false);
    currentStreamRef.current = null;
    sendingRef.current = null;
    setActiveConversationId(id);
  }, []);

  const deleteConversation = useCallback((id: string) => {
    DeleteConversation(id).catch(() => {});
    setConversations((prev) => {
      const next = prev.filter((c) => c.id !== id);
      if (activeConversationId === id) {
        const nextId = next.length > 0 ? next[0].id : null;
        queueMicrotask(() => setActiveConversationId(nextId));
      }
      return next;
    });
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

  const generateResponse = useCallback(async (message: string, targetConvId: string, responseMessageId: string, historyMessages: {role: string; content: string; tool_calls?: any[]; tool_call_id?: string; tool_name?: string; images?: ImageAttachment[]}[], images?: ImageAttachment[]) => {
    if (sendingRef.current === targetConvId) return;
    sendingRef.current = targetConvId;
    setIsLoading(true);
    stopSpeaking();

    const newMessage: Message = {
      id: nextMsgId(),
      role: 'user',
      content: message,
      timestamp: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }),
      images,
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

    // 通过 currentStreamRef 锁定本次流所属的对话，防止事件错配
    const streamId = nextStreamId();
    currentStreamRef.current = { streamId, convId: targetConvId, msgId: responseMessageId };

    if (eventCleanupRef.current) {
      eventCleanupRef.current();
      eventCleanupRef.current = null;
    }
    const cleanup = registerStreamEvents(streamId, targetConvId, responseMessageId);
    eventCleanupRef.current = cleanup;

    const cp = getCurrentProvider(settings);
    const providerType = cp?.type || 'openai';
    const apiKey = cp?.apiKey || '';
    const baseUrl = cp?.baseUrl || '';

    try {
      if (settings.agentMode) {
        log.info('[frontend] sending AgentMessage', { provider: providerType, model: settings.model, history: historyMessages.length, images: images?.length || 0 });
        const req = JSON.stringify({
          message,
          messages: historyMessages,
          conversationId: activeConversationId,
          provider: providerType,
          apiKey,
          baseUrl,
          model: settings.model,
          maxTokens: settings.maxTokens,
          temperature: settings.temperature,
          reasoning: settings.reasoning,
          images: images && images.length > 0 ? images : undefined,
        });
        await AgentMessage(req);
      } else {
        log.info('[frontend] sending StreamMessage', { provider: providerType, model: settings.model, history: historyMessages.length, images: images?.length || 0 });
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
          images: images && images.length > 0 ? images : undefined,
        });
      }
    } catch (error) {
      log.error('[frontend] send error', error);
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
    (message: string, _model?: string, _thinkingLevel?: string, images?: ImageAttachment[]) => {
      let cid = activeConversationId;
      if (!cid) {
        const newConv: Conversation = {
          id: nextConvId(),
          title: message.slice(0, 20) + (message.length > 20 ? '...' : ''),
          messages: [],
          timestamp: new Date().toLocaleDateString(),
        };
        // 使用函数式更新保证和后续 setConversations 在同一批处理内顺序应用
        setConversations((prev) => [newConv, ...prev]);
        setActiveConversationId(newConv.id);
        const msgId = nextMsgId();
        // 传入目标对话的历史（空），不依赖 conversationsRef 的同步状态
        generateResponse(message, newConv.id, msgId, [], images);
        return;
      }
      // 从 conversationsRef 中实时读取当前对话的消息作为历史
      const conv = conversationsRef.current.find((c) => c.id === cid);
      const historyMessages = (conv?.messages || []).map((m) => {
        const base: { role: string; content: string; tool_calls?: any[]; tool_call_id?: string; tool_name?: string; images?: ImageAttachment[] } = {
          role: m.role,
          content: m.content,
        };
        if (m.role === 'assistant' && m.toolCalls && m.toolCalls.length > 0) {
          base.tool_calls = m.toolCalls;
        }
        if (m.role === 'user' && m.images && m.images.length > 0) {
          base.images = m.images;
        }
        return base;
      });
      const msgId = nextMsgId();
      generateResponse(message, cid, msgId, historyMessages, images);
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
      sendingRef.current = null;
    }
    const cid = activeConversationId;
    if (!cid) return;
    const conv = conversationsRef.current.find((c) => c.id === cid);
    if (!conv || conv.messages.length === 0) return;
    // 取最后一条 assistant 消息之前的所有消息作为历史（包括最后一条 user 消息）
    const indices = conv.messages
      .map((m, i) => (m.role === 'assistant' ? i : -1))
      .filter((i) => i >= 0);
    const lastAssistantIdx = indices.length > 0 ? indices[indices.length - 1] : -1;
    // 如果没找到 assistant 消息，使用所有消息；否则用最后一条 assistant 之前的消息
    const trimmed = lastAssistantIdx >= 0 ? conv.messages.slice(0, lastAssistantIdx) : conv.messages;
    const historyMessages = trimmed.map((m) => {
      const base: { role: string; content: string; tool_calls?: any[]; tool_call_id?: string; tool_name?: string; images?: ImageAttachment[] } = {
        role: m.role,
        content: m.content,
      };
      if (m.role === 'assistant' && m.toolCalls && m.toolCalls.length > 0) {
        base.tool_calls = m.toolCalls;
      }
      if (m.role === 'user' && m.images && m.images.length > 0) {
        base.images = m.images;
      }
      return base;
    });
    const msgId = nextMsgId();
    generateResponse(message, cid, msgId, historyMessages);
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

  const handleOpenFile = useCallback((filePath: string, fileName: string) => {
    const tabId = filePath;
    setFileTabs((prev) => {
      const existing = prev.find((t) => t.id === tabId);
      if (existing) return prev;
      return [...prev, { id: tabId, label: fileName, filePath }];
    });
    setActiveFileTabId(tabId);
    setRightPanelOpen(true);
  }, []);

  const handleCloseFileTab = useCallback((tabId: string) => {
    setFileTabs((prev) => {
      const remaining = prev.filter((t) => t.id !== tabId);
      if (remaining.length === 0) {
        setRightPanelOpen(false);
      }
      return remaining;
    });
    setActiveFileTabId((cur) => {
      if (cur !== tabId) return cur;
      // Switch to the last remaining tab or null
      const remaining = fileTabs.filter((t) => t.id !== tabId);
      return remaining.length > 0 ? remaining[remaining.length - 1].id : null;
    });
  }, [fileTabs]);

  const handleCancel = useCallback(() => {
    CancelStream().catch(() => {});
    if (eventCleanupRef.current) {
      eventCleanupRef.current();
      eventCleanupRef.current = null;
    }
    setIsLoading(false);
    currentStreamRef.current = null;
    sendingRef.current = null;
    showToast(t('msg.canceled'));
  }, [showToast, t]);

  const handleSaveSettings = useCallback((newSettings: Settings) => {
    setSettings(newSettings);
    SaveSettings(serializeSettings(newSettings)).catch(() => {});
    setIsSettingsOpen(false);
    // 同步语言偏好
    const locale = (newSettings.locale || 'zh') as 'zh' | 'en';
    setLocale(locale);
    saveLocaleToStorage(locale);
    showToast(t('settings.saved'));
  }, [showToast, t]);

  // ── Onboarding completion ──
  // Convert the OnboardingResult into the chat-app's Settings shape and persist it.
  const handleOnboardingComplete = useCallback(async (result: OnboardingResult) => {
    const m = result.model;
    // Determine a human-friendly display name for the provider
    const preset = PROVIDER_TYPES.find((p) => p.type === m.providerName);
    const providerName = preset?.name || getProviderTypeName(m.providerName) || m.providerName;
    // 从 onboarding 结果中获取语言偏好
    const locale = (result.locale === 'en' ? 'en' : 'zh') as 'zh' | 'en';
    setLocale(locale);
    saveLocaleToStorage(locale);
    const ws = result.workspace;
    const workingDir = ws && !ws.useDefault && ws.path ? ws.path : '';
    const newSettings: Settings = {
      ...DEFAULT_SETTINGS,
      locale,
      workingDir,
      providers: [
        {
          name: providerName,
          type: m.providerName,
          apiKey: m.apiKey,
          apiKeys: m.apiKey ? [m.apiKey] : [],
          baseUrl: m.providerUrl,
        },
      ],
      currentProviderIndex: 0,
      model: m.chatModel || DEFAULT_SETTINGS.model,
    };
    setSettings(newSettings);
    try {
      await SaveSettings(serializeSettings(newSettings));
    } catch (e) {
      console.error('Failed to save onboarding settings', e);
    }
  }, []);

  const handleOnboardingFinish = useCallback(() => {
    setShowOnboarding(false);
    showToast(t('app.onboardingDone'));
  }, [showToast, t]);

  const activeConversation = conversations.find((c) => c.id === activeConversationId);

  // Get working dir
  const workingDir = settings?.workingDir || '';

  // Sort conversations by timestamp descending (newest first).
  const sortedConversations = useMemo(() => {
    return [...conversations].sort((a, b) => {
      const ta = new Date(a.timestamp).getTime();
      const tb = new Date(b.timestamp).getTime();
      return tb - ta; // descending
    });
  }, [conversations]);

  return (
    <div className="app-shell">
      {showOnboarding && (
        <OnboardingApp
          onComplete={handleOnboardingComplete}
          onFinish={handleOnboardingFinish}
        />
      )}

      {/* Left sidebar with animated width */}
      <div className={`sidebar-container ${sidebarCollapsed ? 'sidebar-closed' : 'sidebar-open'}`}>
        <Sidebar
          conversations={sortedConversations}
          activeConversation={activeConversationId}
          workingDir={workingDir}
          onSelectConversation={selectConversation}
          onCreateNewConversation={createNewConversation}
          onDeleteConversation={deleteConversation}
          onRenameConversation={renameConversation}
          onOpenSettings={() => setIsSettingsOpen(true)}
          onOpenFile={handleOpenFile}
        />
      </div>

      <div className="main-frame">
        {activeConversation ? (
          <>
            <div className="topbar">
              <div className="topbar-left">
                {/* Sidebar toggle button — like pi-web */}
                <button
                  onClick={() => setSidebarCollapsed((v) => !v)}
                  title={sidebarCollapsed ? t('app.expandSidebar') : t('app.collapseSidebar')}
                  aria-label={sidebarCollapsed ? t('app.expandSidebar') : t('app.collapseSidebar')}
                  className="sidebar-toggle-btn"
                >
                  {sidebarCollapsed ? (
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
                      <line x1="3" y1="6" x2="21" y2="6" />
                      <line x1="3" y1="12" x2="21" y2="12" />
                      <line x1="3" y1="18" x2="21" y2="18" />
                    </svg>
                  ) : (
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <rect x="3" y="3" width="18" height="18" rx="2" />
                      <line x1="9" y1="3" x2="9" y2="21" />
                    </svg>
                  )}
                </button>
                <div className="breadcrumb">
                  <span className="crumb-root">{t('breadcrumb.conversations')}</span>
                  <span className="crumb-sep">/</span>
                  <span className="crumb-current">{activeConversation.title}</span>
                </div>
                {contextStats && (
                  <span className="bridge-status" title={t('breadcrumb.contextStats')}>
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
              key={activeConversation.id}
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
            key="empty-conversation"
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

      {/* Right panel: file preview with tabs — like pi-web */}
      <div
        className={`right-panel-container ${rightPanelOpen ? 'right-panel-open' : 'right-panel-closed'}`}
      >
        {fileTabs.length > 0 && (
          <FileTabBar
            tabs={fileTabs}
            activeTabId={activeFileTabId}
            onSelectTab={setActiveFileTabId}
            onCloseTab={handleCloseFileTab}
          />
        )}
        <div className="right-panel-body">
          {activeFileTabId ? (
            <FilePreview filePath={activeFileTabId} />
          ) : (
            <div className="file-preview-empty">{t('fileExplorer.noFileSelected')}</div>
          )}
        </div>
      </div>

      {/* File panel toggle — fixed at top-right, like pi-web */}
      <button
        onClick={() => setRightPanelOpen((v) => !v)}
        title={rightPanelOpen ? t('app.hideFilePanel') : t('app.showFilePanel')}
        aria-label={rightPanelOpen ? t('app.hideFilePanel') : t('app.showFilePanel')}
        className="right-panel-toggle-btn"
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <rect x="3" y="3" width="18" height="18" rx="2" />
          <line x1="15" y1="3" x2="15" y2="21" />
        </svg>
      </button>

      {isSettingsOpen && (
        <SettingsPanel
          isOpen={isSettingsOpen}
          currentSettings={settings}
          onSave={handleSaveSettings}
          onClose={() => setIsSettingsOpen(false)}
        />
      )}

      {toast && (
        <div className="app-toast">{toast}</div>
      )}

      {showMemoryPanel && (
        <div className="memory-panel">
          <div className="memory-panel-header">
            <h3>{t('memory.title')}</h3>
            <button onClick={() => setShowMemoryPanel(false)}>{t('memory.close')}</button>
          </div>
          <div className="memory-panel-body">
            {memoryEntries.length === 0 && (
              <div className="memory-empty">{t('memory.empty')}</div>
            )}
            {memoryEntries.map((entry) => (
              <div key={entry.key} className="memory-entry">
                <div className="memory-entry-key">{entry.key}</div>
                <div className="memory-entry-value">{entry.value}</div>
                <div className="memory-entry-meta">
                  {entry.category && <span className="memory-category">{entry.category}</span>}
                  <button
                    className="memory-delete-btn"
                    onClick={() => DeleteMemoryEntry(entry.key).then(refreshMemory)}
                  >
                    {t('memory.delete')}
                  </button>
                </div>
              </div>
            ))}
            <div className="memory-add-form">
              <input
                type="text"
                placeholder={t('memory.key')}
                id="memory-new-key"
              />
              <textarea placeholder={t('memory.value')} id="memory-new-value" />
              <input type="text" placeholder={t('memory.category')} id="memory-new-category" />
              <button
                onClick={() => {
                  const keyEl = document.getElementById('memory-new-key') as HTMLInputElement;
                  const valueEl = document.getElementById('memory-new-value') as HTMLTextAreaElement;
                  const catEl = document.getElementById('memory-new-category') as HTMLInputElement;
                  const key = keyEl.value.trim();
                  const value = valueEl.value.trim();
                  const category = catEl.value.trim();
                  if (key && value) {
                    SetMemoryEntry(key, value, category).then(refreshMemory);
                    keyEl.value = '';
                    valueEl.value = '';
                    catEl.value = '';
                  } else {
                    showToast(t('memory.required'));
                  }
                }}
              >
                {t('memory.addBtn')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default App;
