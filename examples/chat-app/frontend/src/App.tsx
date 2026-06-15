import { useState, useCallback, useEffect, useRef } from 'react';
import Sidebar from './components/Sidebar';
import ChatArea from './components/ChatArea';
import SettingsPanel from './components/SettingsPanel';
import { Message, Conversation } from './types';
import { StreamMessage, CancelStream } from '../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';

interface Settings {
  provider: 'openai' | 'anthropic';
  apiKey: string;
  baseUrl: string;
  model: string;
  customModel: string;
  ttsEnabled: boolean;
  ttsVoice: string;
}

function App() {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeConversationId, setActiveConversationId] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  
  const loadSettings = (): Settings => {
    try {
      const saved = localStorage.getItem('ai-assistant-settings');
      if (saved) {
        return {
          provider: 'openai',
          apiKey: '',
          baseUrl: 'https://api.openai.com/v1',
          model: '',
          customModel: '',
          ttsEnabled: false,
          ttsVoice: 'zh-CN',
          ...JSON.parse(saved)
        };
      }
    } catch (e) {
      console.error('Failed to load settings:', e);
    }
    return {
      provider: 'openai',
      apiKey: '',
      baseUrl: 'https://api.openai.com/v1',
      model: '',
      customModel: '',
      ttsEnabled: false,
      ttsVoice: 'zh-CN'
    };
  };
  
  const [settings, setSettings] = useState<Settings>(loadSettings);
  const [speakingMessageId, setSpeakingMessageId] = useState<string | null>(null);
  
  const speechSynthesisRef = useRef<SpeechSynthesis | null>(null);
  const currentUtteranceRef = useRef<SpeechSynthesisUtterance | null>(null);
  
  const speakText = useCallback((text: string, messageId: string) => {
    if (speechSynthesisRef.current) {
      speechSynthesisRef.current.cancel();
    }
    
    setSpeakingMessageId(messageId);
    
    const utterance = new SpeechSynthesisUtterance(text);
    utterance.lang = settings.ttsVoice;
    utterance.rate = 0.8;
    
    utterance.onend = () => {
      setSpeakingMessageId(null);
    };
    
    utterance.onerror = () => {
      setSpeakingMessageId(null);
    };
    
    speechSynthesisRef.current = window.speechSynthesis;
    currentUtteranceRef.current = utterance;
    
    speechSynthesisRef.current.speak(utterance);
  }, [settings.ttsVoice]);
  
  const stopSpeaking = useCallback(() => {
    if (speechSynthesisRef.current) {
      speechSynthesisRef.current.cancel();
    }
    setSpeakingMessageId(null);
  }, []);
  
  useEffect(() => {
    localStorage.setItem('ai-assistant-settings', JSON.stringify(settings));
  }, [settings]);

  const activeConversation = conversations.find((c) => c.id === activeConversationId);

  const formatTimestamp = (date: Date) => {
    return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
  };

  const createNewConversation = useCallback(() => {
    const newConversation: Conversation = {
      id: Date.now().toString(),
      title: 'New Chat',
      messages: [],
      timestamp: new Date().toLocaleDateString(),
    };
    setConversations((prev) => [newConversation, ...prev]);
    setActiveConversationId(newConversation.id);
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

  const generateResponse = useCallback(async (message: string) => {
    setIsLoading(true);
    stopSpeaking();

    const newMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content: message,
      timestamp: formatTimestamp(new Date()),
    };

    const responseMessageId = (Date.now() + 1).toString();
    
    setConversations((prev) =>
      prev.map((c) =>
        c.id === activeConversationId
          ? {
              ...c,
              messages: [...c.messages, newMessage, {
                id: responseMessageId,
                role: 'assistant',
                content: '',
                timestamp: '',
                thinking: '',
                toolCalls: [],
              }],
              title: c.messages.length === 0 ? message.slice(0, 20) + (message.length > 20 ? '...' : '') : c.title,
              timestamp: new Date().toLocaleDateString(),
            }
          : c
      )
    );

    try {
      await StreamMessage({
        message,
        provider: settings.provider,
        apiKey: settings.apiKey,
        baseUrl: settings.baseUrl,
        model: settings.model
      });
    } catch (error) {
      console.error('Error calling StreamMessage:', error);
      setConversations((prev) =>
        prev.map((c) =>
          c.id === activeConversationId
            ? {
                ...c,
                messages: c.messages.map((m) =>
                  m.id === responseMessageId
                    ? { ...m, content: 'Sorry, I encountered an error. Please try again later.', timestamp: formatTimestamp(new Date()) }
                    : m
                ),
                timestamp: new Date().toLocaleDateString(),
              }
            : c
        )
      );
      setIsLoading(false);
    }
  }, [activeConversationId, settings, stopSpeaking]);

  const handleStreamThinkingDelta = useCallback((delta: string) => {
    setConversations((prev) =>
      prev.map((c) => {
        if (c.id !== activeConversationId) return c;
        const messages = [...c.messages];
        const lastMsg = messages[messages.length - 1];
        if (lastMsg && lastMsg.role === 'assistant') {
          messages[messages.length - 1] = {
            ...lastMsg,
            thinking: (lastMsg.thinking || '') + delta,
          };
        }
        return { ...c, messages };
      })
    );
  }, [activeConversationId]);

  const handleStreamToolCallStart = useCallback((data: string) => {
    try {
      const toolCall = JSON.parse(data);
      setConversations((prev) =>
        prev.map((c) => {
          if (c.id !== activeConversationId) return c;
          const messages = [...c.messages];
          const lastMsg = messages[messages.length - 1];
          if (lastMsg && lastMsg.role === 'assistant') {
            messages[messages.length - 1] = {
              ...lastMsg,
              toolCalls: [...(lastMsg.toolCalls || []), {
                id: toolCall.id,
                name: toolCall.name,
                arguments: '',
              }],
            };
          }
          return { ...c, messages };
        })
      );
    } catch (e) {
      console.error('Error parsing tool call start:', e);
    }
  }, [activeConversationId]);

  const handleStreamToolCallDelta = useCallback((delta: string) => {
    setConversations((prev) =>
      prev.map((c) => {
        if (c.id !== activeConversationId) return c;
        const messages = [...c.messages];
        const lastMsg = messages[messages.length - 1];
        if (lastMsg && lastMsg.role === 'assistant' && lastMsg.toolCalls && lastMsg.toolCalls.length > 0) {
          const toolCalls = [...lastMsg.toolCalls];
          const lastToolCall = { ...toolCalls[toolCalls.length - 1] };
          lastToolCall.arguments += delta;
          toolCalls[toolCalls.length - 1] = lastToolCall;
          messages[messages.length - 1] = { ...lastMsg, toolCalls };
        }
        return { ...c, messages };
      })
    );
  }, [activeConversationId]);

  const handleStreamToolCallEnd = useCallback((args: string) => {
    setConversations((prev) =>
      prev.map((c) => {
        if (c.id !== activeConversationId) return c;
        const messages = [...c.messages];
        const lastMsg = messages[messages.length - 1];
        if (lastMsg && lastMsg.role === 'assistant' && lastMsg.toolCalls && lastMsg.toolCalls.length > 0) {
          const toolCalls = [...lastMsg.toolCalls];
          const lastToolCall = { ...toolCalls[toolCalls.length - 1] };
          lastToolCall.arguments = args;
          toolCalls[toolCalls.length - 1] = lastToolCall;
          messages[messages.length - 1] = { ...lastMsg, toolCalls };
        }
        return { ...c, messages };
      })
    );
  }, [activeConversationId]);

  const handleStreamTextDelta = useCallback((delta: string) => {
    setConversations((prev) =>
      prev.map((c) => {
        if (c.id !== activeConversationId) return c;
        const messages = [...c.messages];
        const lastMsg = messages[messages.length - 1];
        if (lastMsg && lastMsg.role === 'assistant') {
          messages[messages.length - 1] = {
            ...lastMsg,
            content: lastMsg.content + delta,
          };
        }
        return { ...c, messages };
      })
    );
  }, [activeConversationId]);

  const handleStreamDone = useCallback(() => {
    setConversations((prev) =>
      prev.map((c) => {
        if (c.id !== activeConversationId) return c;
        const messages = [...c.messages];
        const lastMsg = messages[messages.length - 1];
        if (lastMsg && lastMsg.role === 'assistant' && !lastMsg.timestamp) {
          messages[messages.length - 1] = { ...lastMsg, timestamp: formatTimestamp(new Date()) };
        }
        return { ...c, messages };
      })
    );
    setIsLoading(false);
  }, [activeConversationId]);

  const handleStreamError = useCallback((error: string) => {
    console.error('Stream error:', error);
    setConversations((prev) =>
      prev.map((c) => {
        if (c.id !== activeConversationId) return c;
        const messages = [...c.messages];
        const lastMsg = messages[messages.length - 1];
        if (lastMsg && lastMsg.role === 'assistant') {
          if (!lastMsg.content) {
            messages[messages.length - 1] = {
              ...lastMsg,
              content: error,
              timestamp: formatTimestamp(new Date()),
            };
          }
        }
        return { ...c, messages };
      })
    );
    setIsLoading(false);
  }, [activeConversationId]);

  useEffect(() => {
    EventsOn('stream-thinking-delta', handleStreamThinkingDelta);
    EventsOn('stream-tool-call-start', handleStreamToolCallStart);
    EventsOn('stream-tool-call-delta', handleStreamToolCallDelta);
    EventsOn('stream-tool-call-end', handleStreamToolCallEnd);
    EventsOn('stream-text-delta', handleStreamTextDelta);
    EventsOn('stream-done', handleStreamDone);
    EventsOn('stream-error', handleStreamError);

    return () => {
      EventsOff('stream-thinking-delta');
      EventsOff('stream-tool-call-start');
      EventsOff('stream-tool-call-delta');
      EventsOff('stream-tool-call-end');
      EventsOff('stream-text-delta');
      EventsOff('stream-done');
      EventsOff('stream-error');
    };
  }, [handleStreamThinkingDelta, handleStreamToolCallStart, handleStreamToolCallDelta, handleStreamToolCallEnd, handleStreamTextDelta, handleStreamDone, handleStreamError]);

  const handleSendMessage = useCallback(
    (message: string) => {
      if (!activeConversationId) {
        createNewConversation();
        setTimeout(() => {
          generateResponse(message);
        }, 0);
      } else {
        generateResponse(message);
      }
    },
    [activeConversationId, createNewConversation, generateResponse]
  );

  const handleSaveSettings = useCallback((newSettings: Settings) => {
    setSettings(newSettings);
    setIsSettingsOpen(false);
  }, []);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setActiveConversationId(null);
        setIsSettingsOpen(false);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  return (
    <div className="flex h-screen bg-slate-900 overflow-hidden">
      <Sidebar
        conversations={conversations}
        activeConversation={activeConversationId}
        onSelectConversation={selectConversation}
        onCreateNewConversation={createNewConversation}
        onDeleteConversation={deleteConversation}
        onOpenSettings={() => setIsSettingsOpen(true)}
      />

      <div className="flex-1 flex flex-col overflow-hidden">
        {activeConversation ? (
          <>
            <div className="h-14 border-b border-slate-700 flex items-center px-6 bg-slate-900 justify-between">
              <h2 className="text-lg font-medium text-white truncate">{activeConversation.title}</h2>
              {isLoading && (
                <button
                  onClick={async () => {
                    try {
                      await CancelStream();
                    } catch (err) {
                      console.error('Failed to cancel stream:', err);
                    }
                  }}
                  className="px-4 py-1.5 bg-red-600 hover:bg-red-700 text-white text-sm font-medium rounded-lg transition-colors"
                >
                  停止
                </button>
              )}
            </div>
            <ChatArea
              messages={activeConversation.messages}
              isLoading={isLoading}
              onSendMessage={handleSendMessage}
              onSpeak={speakText}
              onStopSpeak={stopSpeaking}
              speakingMessageId={speakingMessageId}
            />
          </>
        ) : (
          <div className="flex-1 flex items-center justify-center bg-slate-900">
            <div className="text-center">
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
    </div>
  );
}

export default App;
