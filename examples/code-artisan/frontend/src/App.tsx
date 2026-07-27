import { useState, useCallback, useEffect, useRef } from 'react';
import Sidebar from './components/Sidebar';
import ChatArea from './components/ChatArea';
import SettingsPanel from './components/SettingsPanel';
import CodePreview from './components/CodePreview';
import { Settings, Conversation, ConversationSummary, Message, DEFAULT_SETTINGS, getCurrentProvider } from './types';
import {
  GetSettings, SaveSettings,
  StreamGenerateCode,
  RunPythonScriptBackground, CancelStream,
  GetConversations, GetConversation, SaveConversation, DeleteConversation,
  GetPythonStatus, RebuildPythonRuntime,
} from '../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';
import { HistoryOutlined, SettingOutlined } from './icons';

function App() {
  const [settings, setSettings] = useState<Settings>(DEFAULT_SETTINGS);
  const [settingsLoaded, setSettingsLoaded] = useState(false);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [prompt, setPrompt] = useState('');
  const [generatedCode, setGeneratedCode] = useState('');
  const [isStreaming, setIsStreaming] = useState(false);
  const [streamingCode, setStreamingCode] = useState('');
  const [conversations, setConversations] = useState<ConversationSummary[]>([]);
  const [selectedConvId, setSelectedConvId] = useState<string | null>(null);
  const [pythonStatus, setPythonStatus] = useState('');
  const [pythonStatusLoading, setPythonStatusLoading] = useState(false);
  const [runtimeReady, setRuntimeReady] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [codePanelOpen, setCodePanelOpen] = useState(false);

  // Events cleanup
  useEffect(() => {
    return () => {
      const events = [
        'codegen-delta', 'codegen-done', 'codegen-error',
        'run-started', 'run-finished', 'run-error',
      ];
      events.forEach((evt) => EventsOff(evt));
    };
  }, []);

  // Load settings
  useEffect(() => {
    GetSettings().then((str) => {
      try {
        const s = JSON.parse(str) as Settings;
        if (s.providers && s.providers.length > 0) {
          setSettings({ ...DEFAULT_SETTINGS, ...s });
        }
        setSettingsLoaded(true);
      } catch (e) {
        setSettingsLoaded(true);
      }
    }).catch(() => setSettingsLoaded(true));
  }, []);

  // Auto-save settings
  const settingsSaveTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (!settingsLoaded) return;
    if (settingsSaveTimeoutRef.current) clearTimeout(settingsSaveTimeoutRef.current);
    settingsSaveTimeoutRef.current = setTimeout(() => {
      SaveSettings(JSON.stringify(settings)).catch(() => {});
    }, 300);
    return () => {
      if (settingsSaveTimeoutRef.current) clearTimeout(settingsSaveTimeoutRef.current);
    };
  }, [settings, settingsLoaded]);

  // Load conversations list
  useEffect(() => {
    if (!settingsLoaded) return;
    GetConversations().then((str) => {
      try {
        const convs = JSON.parse(str) as ConversationSummary[];
        setConversations(convs);
      } catch (e) { /* ignore */ }
    }).catch(() => {});
  }, [settingsLoaded]);

  // messages ref to track current conversation messages
  const messagesRef = useRef<Message[]>([]);

  // After generation, update messages ref + refresh sidebar
  // (persistence is handled by Go backend)
  const postGeneration = useCallback((promptText: string, code: string) => {
    messagesRef.current = [
      ...messagesRef.current,
      { role: 'user' as const, content: promptText },
      { role: 'assistant' as const, content: code },
    ];
    // Refresh sidebar
    GetConversations().then((str) => {
      try {
        setConversations(JSON.parse(str));
      } catch {}
    }).catch(() => {});
  }, []);

  const handleGenerate = useCallback(() => {
    if (!prompt.trim() || isLoading || isStreaming) return;

    const cp = getCurrentProvider(settings);
    if (!cp) return;

    const req: Record<string, unknown> = {
      prompt: prompt.trim(),
      provider: cp.type,
      apiKey: cp.apiKey,
      baseUrl: cp.baseUrl,
      model: settings.model,
      maxTokens: settings.maxTokens,
      temperature: settings.temperature,
      currentCode: generatedCode || '',
      convId: selectedConvId,
      messages: messagesRef.current,
    };

    setIsStreaming(true);
    setStreamingCode('');

    const onDelta = (delta: string) => {
      setStreamingCode((prev) => prev + delta);
    };

    const onDone = (code: string) => {
      EventsOff('codegen-delta');
      EventsOff('codegen-done');
      EventsOff('codegen-error');

      if (code) {
        setGeneratedCode(code);
        setStreamingCode('');
        setIsStreaming(false);
        // Backend saves conversation; update messages ref + sidebar
        postGeneration(prompt.trim(), code);
        // selectedConvId is already set from req, or backend returns it
      } else {
        setIsStreaming(false);
      }
    };

    const onError = (error: string) => {
      EventsOff('codegen-delta');
      EventsOff('codegen-done');
      EventsOff('codegen-error');
      setStreamingCode((prev) => prev + '\n\n[错误] ' + error);
      setIsStreaming(false);
    };

    EventsOn('codegen-delta', onDelta);
    EventsOn('codegen-done', onDone);
    EventsOn('codegen-error', onError);

    StreamGenerateCode(JSON.stringify(req)).catch((err) => {
      EventsOff('codegen-delta');
      EventsOff('codegen-done');
      EventsOff('codegen-error');
      setStreamingCode('生成失败: ' + String(err));
      setIsStreaming(false);
    });
  }, [prompt, generatedCode, isLoading, isStreaming, settings, selectedConvId, messagesRef, postGeneration]);

  const handleRun = useCallback(() => {
    const codeToRun = streamingCode || generatedCode;
    if (!codeToRun.trim()) return;

    setIsLoading(true);

    EventsOn('run-started', () => {
      EventsOff('run-started');
    });

    EventsOn('run-error', (err: string) => {
      EventsOff('run-error');
      alert('运行出错:\n' + err);
    });

    EventsOn('run-finished', () => {
      EventsOff('run-finished');
      setIsLoading(false);
    });

    RunPythonScriptBackground(JSON.stringify({ code: codeToRun })).catch((err) => {
      setIsLoading(false);
      alert('启动失败: ' + String(err));
    });
  }, [streamingCode, generatedCode]);

  const handleCancel = useCallback(() => {
    CancelStream();
    setIsStreaming(false);
    setIsLoading(false);
  }, []);

  const handleSelectConv = useCallback((summary: ConversationSummary) => {
    GetConversation(summary.id).then((jsonStr) => {
      try {
        const conv = JSON.parse(jsonStr) as Conversation;
        setPrompt(conv.prompt);
        setGeneratedCode(conv.code);
        setStreamingCode('');
        setSelectedConvId(conv.id);
        messagesRef.current = conv.messages || [];
      } catch {}
    }).catch(() => {});
  }, []);

  const handleDeleteConv = useCallback((id: string) => {
    DeleteConversation(id).then(() => {
      setConversations((prev) => prev.filter((c) => c.id !== id));
      if (selectedConvId === id) {
        setSelectedConvId(null);
        setPrompt('');
        setGeneratedCode('');
        setStreamingCode('');
      }
    }).catch(() => {});
  }, [selectedConvId]);

  const handleSaveSettings = useCallback((newSettings: Settings) => {
    setSettings(newSettings);
    setIsSettingsOpen(false);
  }, []);

  const handleCheckPython = useCallback(() => {
    setPythonStatusLoading(true);
    GetPythonStatus().then((result) => {
      setPythonStatus(result);
      setPythonStatusLoading(false);
      try {
        const s = JSON.parse(result);
        setRuntimeReady(s.ready);
      } catch {}
    }).catch(() => {
      setPythonStatus('检查失败');
      setPythonStatusLoading(false);
    });
  }, []);

  const handleRebuildRuntime = useCallback(() => {
    setPythonStatusLoading(true);
    RebuildPythonRuntime().then((result) => {
      setPythonStatus(result);
      setPythonStatusLoading(false);
      try {
        const s = JSON.parse(result);
        setRuntimeReady(s.ready);
      } catch {}
    }).catch(() => {
      setPythonStatus('重建失败');
      setPythonStatusLoading(false);
    });
  }, []);

  const handleClearCode = useCallback(() => {
    setGeneratedCode('');
    setStreamingCode('');
    setPrompt('');
    setSelectedConvId(null);
    messagesRef.current = [];
  }, []);

  return (
    <div className="app-shell">
      {/* Left sidebar with animated width */}
      <div className={`sidebar-container ${sidebarCollapsed ? 'sidebar-closed' : 'sidebar-open'}`}>
        <Sidebar
          conversations={conversations}
          selectedConvId={selectedConvId}
          pythonStatus={pythonStatus}
          pythonStatusLoading={pythonStatusLoading}
          runtimeReady={runtimeReady}
          onSelectConv={handleSelectConv}
          onDeleteConv={handleDeleteConv}
          onOpenSettings={() => setIsSettingsOpen(true)}
          onCheckPython={handleCheckPython}
          onRebuildRuntime={handleRebuildRuntime}
          onClearCode={handleClearCode}
        />
      </div>

      <main className="main-frame">
        <div className="topbar">
          <div className="topbar-left">
            {/* Sidebar toggle button */}
            <button
              onClick={() => setSidebarCollapsed((v) => !v)}
              title={sidebarCollapsed ? '展开侧边栏' : '收起侧边栏'}
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
            <span className="brand-title">Code Artisan</span>
            <span className="brand-subtitle">AI 绘图代码生成器</span>
          </div>
          <div className="topbar-right">
            <button
              className="topbar-btn"
              onClick={() => setIsSettingsOpen(true)}
              title="设置"
            >
              <SettingOutlined size={16} />
            </button>
          </div>
        </div>

        <div className="workspace">
          <ChatArea
            prompt={prompt}
            onPromptChange={setPrompt}
            generatedCode={streamingCode || generatedCode}
            isStreaming={isStreaming}
            isLoading={isLoading}
            onGenerate={handleGenerate}
            onRun={handleRun}
            onCancel={handleCancel}
            onClear={handleClearCode}
            onToggleCodePanel={() => setCodePanelOpen((v) => !v)}
            isCodePanelOpen={codePanelOpen}
          />

          {/* Right panel: code preview with animated width */}
          <div className={`code-panel-container ${codePanelOpen ? 'code-panel-open' : 'code-panel-closed'}`}>
            <CodePreview
              code={streamingCode || generatedCode}
              isStreaming={isStreaming}
            />
          </div>
        </div>
      </main>

      {isSettingsOpen && (
        <SettingsPanel
          settings={settings}
          onSave={handleSaveSettings}
          onClose={() => setIsSettingsOpen(false)}
        />
      )}
    </div>
  );
}

export default App;
