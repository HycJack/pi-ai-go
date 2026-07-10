import { useState, useEffect, useCallback } from 'react';
import { X, Save, Brain, Settings2, Wrench, Eye, EyeOff, Plus, Trash2, ChevronDown, ChevronUp, RefreshCw } from 'lucide-react';
import { GetModels } from '../../wailsjs/go/main/App';
import type { Settings, ProviderConfig } from '../types';
import { PROVIDER_TYPES, getProviderTypeName } from '../types';

interface SettingsPanelProps {
  isOpen: boolean;
  onClose: () => void;
  currentSettings: Settings;
  onSave: (settings: Settings) => void;
}

interface ModelInfo {
  id: string;
  name: string;
  reasoning?: boolean;
  thinkingLevelMap?: Record<string, string>;
}

type TabId = 'general' | 'agent' | 'system';

const tabs: { id: TabId; label: string; icon: React.ReactNode }[] = [
  { id: 'general', label: 'General', icon: <Settings2 size={14} /> },
  { id: 'agent', label: 'Agent', icon: <Brain size={14} /> },
  { id: 'system', label: 'System', icon: <Wrench size={14} /> },
];

export default function SettingsPanel({ isOpen, onClose, currentSettings, onSave }: SettingsPanelProps) {
  const [activeTab, setActiveTab] = useState<TabId>('general');
  const [settings, setSettings] = useState<Settings>(currentSettings);
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [saved, setSaved] = useState(false);
  const [showNewForm, setShowNewForm] = useState(false);
  const [newProviderType, setNewProviderType] = useState('openai');
  const [newProviderName, setNewProviderName] = useState('');
  const [newProviderKey, setNewProviderKey] = useState('');
  const [showNewKey, setShowNewKey] = useState(false);
  const [expandedIdx, setExpandedIdx] = useState<number | null>(0);
  const [secretKeys, setSecretKeys] = useState<Record<number, boolean>>({});

  // 获取当前 provider
  const currentProvider = settings.providers[settings.currentProviderIndex] || settings.providers[0];

  useEffect(() => {
    if (!isOpen) return;
    setSettings({ ...currentSettings });
    setSaved(false);
  }, [currentSettings, isOpen]);

  // 当前 provider 变化时拉取模型
  useEffect(() => {
    if (!isOpen) return;
    const cp = settings.providers[settings.currentProviderIndex];
    if (!cp) return;
    setIsLoading(true);
    GetModels({
      provider: cp.type,
      baseUrl: cp.baseUrl,
      apiKey: cp.apiKey,
    }).then((list) => {
      setModels(list || []);
    }).catch(() => {}).finally(() => setIsLoading(false));
  }, [isOpen, settings.currentProviderIndex, settings.providers]);

  // Auto-save on panel close
  useEffect(() => {
    if (isOpen) return;
    if (JSON.stringify(settings) === JSON.stringify(currentSettings)) return;
    onSave(settings);
  }, [isOpen]);

  const addProvider = useCallback(() => {
    const name = newProviderName.trim() || getProviderTypeName(newProviderType);
    const newP: ProviderConfig = {
      name,
      type: newProviderType,
      apiKey: newProviderKey.trim(),
      apiKeys: [],
      baseUrl: PROVIDER_TYPES.find(p => p.type === newProviderType)?.baseUrl || `https://api.${newProviderType}.com/v1`,
    };
    setSettings((prev) => ({
      ...prev,
      providers: [...prev.providers, newP],
      currentProviderIndex: prev.providers.length,
    }));
    setShowNewForm(false);
    setNewProviderName('');
    setNewProviderKey('');
    setShowNewKey(false);
  }, [newProviderType, newProviderName, newProviderKey]);

  const removeProvider = useCallback((idx: number) => {
    setSettings((prev) => {
      const providers = prev.providers.filter((_, i) => i !== idx);
      const currentIdx = Math.min(prev.currentProviderIndex, providers.length - 1);
      return { ...prev, providers, currentProviderIndex: Math.max(currentIdx, 0) };
    });
  }, []);

  const selectProvider = useCallback((idx: number) => {
    setSettings((prev) => ({ ...prev, currentProviderIndex: idx }));
    setExpandedIdx(idx);
  }, []);

  const updateProvider = useCallback((idx: number, updater: (p: ProviderConfig) => ProviderConfig) => {
    setSettings((prev) => {
      const providers = [...prev.providers];
      providers[idx] = updater(providers[idx]);
      return { ...prev, providers };
    });
  }, []);

  const handleSave = () => {
    onSave(settings);
    setSaved(true);
    setTimeout(() => setSaved(false), 1600);
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-slate-800 rounded-xl shadow-2xl w-full max-w-2xl mx-4 overflow-hidden max-h-[90vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-700">
          <h2 className="text-lg font-semibold text-white">Settings</h2>
          <button onClick={onClose} className="p-2 hover:bg-slate-700 rounded-lg transition-colors">
            <X className="w-5 h-5 text-slate-400" />
          </button>
        </div>

        {/* Tab bar */}
        <div className="flex gap-1 px-6 pt-3 pb-0 border-b border-slate-700 bg-slate-800/50">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              className={`flex items-center gap-1.5 px-3 py-2 text-sm rounded-t-lg transition-colors ${
                activeTab === tab.id
                  ? 'bg-slate-700 text-white border border-b-0 border-slate-600'
                  : 'text-slate-400 hover:text-slate-300 hover:bg-slate-700/50'
              }`}
              onClick={() => setActiveTab(tab.id)}
            >
              {tab.icon}
              <span>{tab.label}</span>
            </button>
          ))}
        </div>

        {/* Body */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {/* ═══ General ═══ */}
          {activeTab === 'general' && (
            <>
              {/* Providers list */}
              <section>
                <div className="flex items-center justify-between mb-3">
                  <label className="block text-sm font-medium text-slate-300">Providers</label>
                  <button
                    onClick={() => setShowNewForm(!showNewForm)}
                    className="flex items-center gap-1 px-2 py-1 text-xs bg-blue-600 text-white rounded hover:bg-blue-500 transition-colors"
                  >
                    <Plus size={12} />
                    Add Provider
                  </button>
                </div>

                {/* Add new provider form */}
                {showNewForm && (
                  <div className="mb-4 p-4 bg-slate-700/50 rounded-lg border border-slate-600 space-y-3">
                    <div>
                      <label className="block text-xs text-slate-400 mb-1">Type</label>
                      <div className="flex flex-wrap gap-1.5">
                        {PROVIDER_TYPES.map((pt) => (
                          <button
                            key={pt.type}
                            className={`px-2.5 py-1 text-xs rounded border transition-colors ${
                              newProviderType === pt.type
                                ? 'bg-blue-600 border-blue-500 text-white'
                                : 'bg-slate-700 border-slate-600 text-slate-300 hover:bg-slate-600'
                            }`}
                            onClick={() => { setNewProviderType(pt.type); setNewProviderName(''); }}
                          >
                            {pt.name}
                          </button>
                        ))}
                      </div>
                    </div>
                    <div>
                      <label className="block text-xs text-slate-400 mb-1">Display Name (optional)</label>
                      <input
                        type="text"
                        value={newProviderName}
                        onChange={(e) => setNewProviderName(e.target.value)}
                        placeholder={getProviderTypeName(newProviderType)}
                        className="w-full px-3 py-1.5 text-sm bg-slate-700 border border-slate-600 rounded text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-slate-400 mb-1">API Key</label>
                      <div className="relative">
                        <input
                          type={showNewKey ? 'text' : 'password'}
                          value={newProviderKey}
                          onChange={(e) => setNewProviderKey(e.target.value)}
                          placeholder="sk-..."
                          className="w-full px-3 py-1.5 text-sm bg-slate-700 border border-slate-600 rounded text-white placeholder-slate-400 focus:outline-none focus:border-blue-500 pr-8"
                        />
                        <button
                          type="button"
                          className="absolute right-1.5 top-1/2 -translate-y-1/2 p-0.5 text-slate-400 hover:text-blue-400"
                          onClick={() => setShowNewKey(v => !v)}
                        >
                          {showNewKey ? <EyeOff size={14} /> : <Eye size={14} />}
                        </button>
                      </div>
                    </div>
                    <div className="flex gap-2">
                      <button
                        onClick={() => setShowNewForm(false)}
                        className="px-3 py-1.5 text-xs bg-slate-600 text-slate-300 rounded hover:bg-slate-500 transition-colors"
                      >
                        Cancel
                      </button>
                      <button
                        onClick={addProvider}
                        className="px-3 py-1.5 text-xs bg-blue-600 text-white rounded hover:bg-blue-500 transition-colors"
                      >
                        Add
                      </button>
                    </div>
                  </div>
                )}

                {/* Provider cards */}
                {settings.providers.length === 0 ? (
                  <p className="text-sm text-slate-500">No providers configured. Click "Add Provider" to get started.</p>
                ) : (
                  <div className="space-y-2">
                    {settings.providers.map((p, idx) => (
                      <div
                        key={idx}
                        className={`rounded-lg border transition-colors ${
                          idx === settings.currentProviderIndex
                            ? 'border-blue-500 bg-slate-700/80'
                            : 'border-slate-600 bg-slate-700/40'
                        }`}
                      >
                        {/* Card header */}
                        <div
                          className="flex items-center justify-between px-4 py-3 cursor-pointer"
                          onClick={() => selectProvider(idx)}
                        >
                          <div className="flex items-center gap-2 min-w-0">
                            <span className="text-sm font-medium text-white truncate">{p.name}</span>
                            <span className="text-xs text-slate-400">{getProviderTypeName(p.type)}</span>
                            {idx === settings.currentProviderIndex && (
                              <span className="text-xs text-blue-400 font-medium">Active</span>
                            )}
                          </div>
                          <div className="flex items-center gap-1">
                            <button
                              onClick={(e) => { e.stopPropagation(); removeProvider(idx); }}
                              className="p-1 text-slate-400 hover:text-red-400 transition-colors"
                              title="Remove provider"
                            >
                              <Trash2 size={14} />
                            </button>
                            {expandedIdx === idx ? <ChevronUp size={14} className="text-slate-400" /> : <ChevronDown size={14} className="text-slate-400" />}
                          </div>
                        </div>

                        {/* Expanded config */}
                        {expandedIdx === idx && (
                          <div className="px-4 pb-4 space-y-3 border-t border-slate-600 pt-3">
                            <div>
                              <label className="block text-xs text-slate-400 mb-1">Display Name</label>
                              <input
                                type="text"
                                value={p.name}
                                onChange={(e) => updateProvider(idx, (pv) => ({ ...pv, name: e.target.value }))}
                                className="w-full px-3 py-1.5 text-sm bg-slate-700 border border-slate-600 rounded text-white focus:outline-none focus:border-blue-500"
                              />
                            </div>
                            <div>
                              <label className="block text-xs text-slate-400 mb-1">API Key</label>
                              <div className="relative">
                                <input
                                  type={secretKeys[idx] ? 'text' : 'password'}
                                  value={p.apiKey}
                                  onChange={(e) => updateProvider(idx, (pv) => ({ ...pv, apiKey: e.target.value }))}
                                  placeholder="sk-..."
                                  className="w-full px-3 py-1.5 text-sm bg-slate-700 border border-slate-600 rounded text-white placeholder-slate-400 focus:outline-none focus:border-blue-500 pr-8"
                                />
                                <button
                                  type="button"
                                  className="absolute right-1.5 top-1/2 -translate-y-1/2 p-0.5 text-slate-400 hover:text-blue-400"
                                  onClick={() => setSecretKeys(prev => ({ ...prev, [idx]: !prev[idx] }))}
                                >
                                  {secretKeys[idx] ? <EyeOff size={14} /> : <Eye size={14} />}
                                </button>
                              </div>
                            </div>
                            <div>
                              <label className="block text-xs text-slate-400 mb-1">Base URL</label>
                              <input
                                type="text"
                                value={p.baseUrl}
                                onChange={(e) => updateProvider(idx, (pv) => ({ ...pv, baseUrl: e.target.value }))}
                                className="w-full px-3 py-1.5 text-sm bg-slate-700 border border-slate-600 rounded text-white focus:outline-none focus:border-blue-500"
                              />
                            </div>
                            <div>
                              <label className="block text-xs text-slate-400 mb-1">
                                API Keys Pool
                                <span className="text-xs text-slate-500 ml-1">(one per line)</span>
                              </label>
                              <textarea
                                value={p.apiKeys.join('\n')}
                                onChange={(e) => updateProvider(idx, (pv) => ({ ...pv, apiKeys: e.target.value.split('\n').map(s => s.trim()).filter(Boolean) }))}
                                rows={2}
                                className="w-full px-3 py-1.5 text-sm bg-slate-700 border border-slate-600 rounded text-white focus:outline-none focus:border-blue-500 font-mono"
                                placeholder="sk-xxx1&#10;sk-xxx2"
                              />
                              <p className="text-xs text-slate-500 mt-0.5">
                                {p.apiKeys.length > 0 ? `${p.apiKeys.length} keys configured` : 'Leave empty to use single API key above'}
                              </p>
                            </div>
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </section>

              {/* Model selector */}
              <section>
                <label className="block text-sm font-medium text-slate-300 mb-2">Model</label>
                <div className="flex gap-2">
                  <select
                    value={settings.model}
                    onChange={(e) => setSettings((prev) => ({ ...prev, model: e.target.value }))}
                    disabled={isLoading}
                    className="flex-1 px-4 py-2.5 bg-slate-700 border border-slate-600 rounded-lg text-white appearance-none focus:outline-none focus:border-blue-500 cursor-pointer"
                  >
                    <option value="" disabled>Select a model</option>
                    {models.map((m) => (
                      <option key={m.id} value={m.id}>{m.name || m.id}</option>
                    ))}
                  </select>
                  <button
                    onClick={() => {
                      const cp = settings.providers[settings.currentProviderIndex];
                      if (!cp) return;
                      setIsLoading(true);
                      GetModels({ provider: cp.type, baseUrl: cp.baseUrl, apiKey: cp.apiKey })
                        .then((list) => setModels(list || []))
                        .catch(() => {})
                        .finally(() => setIsLoading(false));
                    }}
                    disabled={isLoading}
                    className="px-3 py-2.5 bg-slate-700 border border-slate-600 rounded-lg hover:bg-slate-600 transition-colors disabled:opacity-50"
                    title="Refresh models"
                  >
                    <RefreshCw className={`w-4 h-4 text-slate-400 ${isLoading ? 'animate-spin' : ''}`} />
                  </button>
                </div>
              </section>

              {/* Max Tokens + Temperature */}
              <div className="grid grid-cols-2 gap-4">
                <section>
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    Max Tokens <span className="text-xs text-slate-500">({settings.maxTokens})</span>
                  </label>
                  <input
                    type="range"
                    min="256"
                    max="65536"
                    step="256"
                    value={settings.maxTokens}
                    onChange={(e) => setSettings((prev) => ({ ...prev, maxTokens: parseInt(e.target.value) }))}
                    className="w-full accent-blue-500"
                  />
                </section>
                <section>
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    Temperature <span className="text-xs text-slate-500">({settings.temperature.toFixed(1)})</span>
                  </label>
                  <input
                    type="range"
                    min="0"
                    max="2"
                    step="0.1"
                    value={settings.temperature}
                    onChange={(e) => setSettings((prev) => ({ ...prev, temperature: parseFloat(e.target.value) }))}
                    className="w-full accent-blue-500"
                  />
                </section>
              </div>

              {/* TTS */}
              <section>
                <label className="block text-sm font-medium text-slate-300 mb-2">Text-to-Speech</label>
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm text-slate-400">Enable TTS</span>
                  <button
                    onClick={() => setSettings((prev) => ({ ...prev, ttsEnabled: !prev.ttsEnabled }))}
                    className={`relative w-12 h-6 rounded-full transition-colors ${settings.ttsEnabled ? 'bg-blue-600' : 'bg-slate-600'}`}
                  >
                    <span className={`absolute top-1 w-4 h-4 rounded-full bg-white transition-transform ${settings.ttsEnabled ? 'translate-x-7' : 'translate-x-1'}`} />
                  </button>
                </div>
                {settings.ttsEnabled && (
                  <select
                    value={settings.ttsVoice}
                    onChange={(e) => setSettings((prev) => ({ ...prev, ttsVoice: e.target.value }))}
                    className="w-full px-4 py-2.5 bg-slate-700 border border-slate-600 rounded-lg text-white focus:outline-none focus:border-blue-500 cursor-pointer"
                  >
                    <option value="zh-CN">Chinese (Mandarin)</option>
                    <option value="en-US">English (US)</option>
                    <option value="en-GB">English (UK)</option>
                    <option value="ja-JP">Japanese</option>
                    <option value="ko-KR">Korean</option>
                  </select>
                )}
              </section>
            </>
          )}

          {/* ═══ Agent ═══ */}
          {activeTab === 'agent' && (
            <>
              <section>
                <label className="block text-sm font-medium text-slate-300 mb-3">Agent Behavior</label>

                <div className="space-y-3">
                  {/* Auto-compact context */}
                  <div className="flex items-center justify-between">
                    <div>
                      <span className="text-sm text-slate-300">Auto-compact context</span>
                      <p className="text-xs text-slate-500">Automatically summarize old messages when context window is near limit</p>
                    </div>
                    <button
                      onClick={() => setSettings((prev) => ({
                        ...prev,
                        agentSettings: { ...prev.agentSettings, autoCompact: !prev.agentSettings.autoCompact },
                      }))}
                      className={`relative w-12 h-6 rounded-full transition-colors ${settings.agentSettings.autoCompact ? 'bg-blue-600' : 'bg-slate-600'}`}
                    >
                      <span className={`absolute top-1 w-4 h-4 rounded-full bg-white transition-transform ${settings.agentSettings.autoCompact ? 'translate-x-7' : 'translate-x-1'}`} />
                    </button>
                  </div>

                  {/* Auto-learn memory */}
                  <div className="flex items-center justify-between">
                    <div>
                      <span className="text-sm text-slate-300">Auto-learn memory</span>
                      <p className="text-xs text-slate-500">Learn facts (name, preferences, etc.) from conversations</p>
                    </div>
                    <button
                      onClick={() => setSettings((prev) => ({
                        ...prev,
                        agentSettings: { ...prev.agentSettings, autoLearn: !prev.agentSettings.autoLearn },
                      }))}
                      className={`relative w-12 h-6 rounded-full transition-colors ${settings.agentSettings.autoLearn ? 'bg-blue-600' : 'bg-slate-600'}`}
                    >
                      <span className={`absolute top-1 w-4 h-4 rounded-full bg-white transition-transform ${settings.agentSettings.autoLearn ? 'translate-x-7' : 'translate-x-1'}`} />
                    </button>
                  </div>
                </div>
              </section>

              <section>
                <label className="block text-sm font-medium text-slate-300 mb-2">Thinking Depth</label>
                <select
                  value={settings.reasoning}
                  onChange={(e) => setSettings((prev) => ({ ...prev, reasoning: e.target.value }))}
                  className="w-full px-4 py-2.5 bg-slate-700 border border-slate-600 rounded-lg text-white focus:outline-none focus:border-blue-500 cursor-pointer capitalize"
                >
                  <option value="minimal">Minimal</option>
                  <option value="low">Low</option>
                  <option value="medium">Medium</option>
                  <option value="high">High</option>
                  <option value="xhigh">X-High</option>
                </select>
              </section>

              <section>
                <label className="block text-sm font-medium text-slate-300 mb-2">Skills Directory</label>
                <input
                  type="text"
                  value={settings.agentSettings.skillsDir}
                  onChange={(e) => setSettings((prev) => ({
                    ...prev,
                    agentSettings: { ...prev.agentSettings, skillsDir: e.target.value },
                  }))}
                  placeholder="Leave empty for default (~/.pi-chat-app/skills)"
                  className="w-full px-4 py-2.5 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
                />
                <p className="text-xs text-slate-500 mt-1">
                  Loads SKILL.md files recursively from this directory
                </p>
              </section>
            </>
          )}

          {/* ═══ System ═══ */}
          {activeTab === 'system' && (
            <>
              <section>
                <label className="block text-sm font-medium text-slate-300 mb-2">Context Statistics</label>
                <p className="text-xs text-slate-500">
                  The Auto-compact toggle in the Agent tab controls automatic context summarization.
                  Token usage stats appear here when available.
                </p>
              </section>
            </>
          )}
        </div>

        {/* Footer */}
        <div className="flex gap-3 px-6 py-4 border-t border-slate-700 bg-slate-800/50">
          <button
            onClick={onClose}
            className="flex-1 py-2.5 px-4 bg-slate-700 text-slate-300 rounded-lg font-medium hover:bg-slate-600 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            className={`flex-1 py-2.5 px-4 rounded-lg font-medium transition-colors flex items-center justify-center gap-2 ${
              saved ? 'bg-green-600 text-white' : 'bg-blue-600 text-white hover:bg-blue-500'
            }`}
          >
            <Save className="w-4 h-4" />
            {saved ? 'Saved!' : 'Save'}
          </button>
        </div>
      </div>
    </div>
  );
}
