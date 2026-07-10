import { useState, useEffect, useCallback } from 'react';
import {
  CloseOutlined,
  EyeInvisibleOutlined,
  EyeOutlined,
  RefreshOutlined,
  SaveOutlined,
  Brain,
  Settings2,
  CodeOutlined,
  WrenchOutlined,
  PlusOutlined,
  DeleteOutlined,
  ChevronDownOutlined,
  ChevronUpOutlined,
  FolderOpenOutlined,
} from '../icons';
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

type TabId = 'general' | 'system';

const tabs: { id: TabId; label: string; icon: React.ReactNode }[] = [
  { id: 'general', label: 'General', icon: <Settings2 size={14} /> },
  { id: 'system', label: 'System', icon: <WrenchOutlined size={14} /> },
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

  const currentProvider = settings.providers[settings.currentProviderIndex] || settings.providers[0];

  useEffect(() => {
    if (!isOpen) return;
    setSettings({ ...currentSettings });
    setSaved(false);
  }, [currentSettings, isOpen]);

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
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-card settings-modal-card" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2 className="modal-title">Settings</h2>
          <button className="icon-btn" onClick={onClose} aria-label="Close">
            <CloseOutlined size={16} />
          </button>
        </div>

        <div className="settings-tabs">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              className={`settings-tab ${activeTab === tab.id ? 'active' : ''}`}
              onClick={() => setActiveTab(tab.id)}
            >
              {tab.icon}
              <span>{tab.label}</span>
            </button>
          ))}
        </div>

        <div className="modal-body">
          {activeTab === 'general' && (
            <>
              <section className="settings-section">
                <div className="settings-section-header">
                  <label className="settings-label">Providers</label>
                  <button
                    onClick={() => setShowNewForm(!showNewForm)}
                    className="btn-sm"
                  >
                    <PlusOutlined size={12} />
                    Add Provider
                  </button>
                </div>

                {showNewForm && (
                  <div className="add-provider-form">
                    <div className="provider-field">
                      <label className="provider-field-label">Type</label>
                      <div className="provider-type-grid">
                        {PROVIDER_TYPES.map((pt) => (
                          <button
                            key={pt.type}
                            className={`provider-type-btn ${newProviderType === pt.type ? 'active' : ''}`}
                            onClick={() => { setNewProviderType(pt.type); setNewProviderName(''); }}
                          >
                            {pt.name}
                          </button>
                        ))}
                      </div>
                    </div>
                    <div className="provider-field">
                      <label className="provider-field-label">Display Name (optional)</label>
                      <input
                        type="text"
                        value={newProviderName}
                        onChange={(e) => setNewProviderName(e.target.value)}
                        placeholder={getProviderTypeName(newProviderType)}
                        className="text-input"
                      />
                    </div>
                    <div className="provider-field">
                      <label className="provider-field-label">API Key</label>
                      <div className="input-with-adornment">
                        <input
                          type={showNewKey ? 'text' : 'password'}
                          value={newProviderKey}
                          onChange={(e) => setNewProviderKey(e.target.value)}
                          placeholder="sk-..."
                          className="text-input"
                        />
                        <button
                          type="button"
                          className="input-adornment"
                          onClick={() => setShowNewKey(v => !v)}
                        >
                          {showNewKey ? <EyeInvisibleOutlined size={14} /> : <EyeOutlined size={14} />}
                        </button>
                      </div>
                    </div>
                    <div style={{ display: 'flex', gap: 8, marginTop: 12 }}>
                      <button
                        onClick={() => setShowNewForm(false)}
                        className="btn-sm"
                      >
                        Cancel
                      </button>
                      <button
                        onClick={addProvider}
                        className="btn-sm btn-primary-sm"
                      >
                        Add
                      </button>
                    </div>
                  </div>
                )}

                {settings.providers.length === 0 ? (
                  <div className="settings-empty">No providers configured. Click "Add Provider" to get started.</div>
                ) : (
                  <div className="provider-list">
                    {settings.providers.map((p, idx) => (
                      <div
                        key={idx}
                        className={`provider-card ${idx === settings.currentProviderIndex ? 'active' : ''}`}
                      >
                        <div
                          className="provider-card-header"
                          onClick={() => selectProvider(idx)}
                        >
                          <div className="provider-card-info">
                            <span className="provider-card-name">{p.name}</span>
                            <span className="provider-card-type">{getProviderTypeName(p.type)}</span>
                            {idx === settings.currentProviderIndex && (
                              <span className="provider-card-active-badge">Active</span>
                            )}
                          </div>
                          <div className="provider-card-actions">
                            <button
                              onClick={(e) => { e.stopPropagation(); removeProvider(idx); }}
                              className="icon-btn"
                              title="Remove provider"
                              style={{ width: 24, height: 24 }}
                            >
                              <DeleteOutlined size={14} />
                            </button>
                            {expandedIdx === idx ? <ChevronUpOutlined size={14} /> : <ChevronDownOutlined size={14} />}
                          </div>
                        </div>

                        {expandedIdx === idx && (
                          <div className="provider-card-body">
                            <div className="provider-field">
                              <label className="provider-field-label">Display Name</label>
                              <input
                                type="text"
                                value={p.name}
                                onChange={(e) => updateProvider(idx, (pv) => ({ ...pv, name: e.target.value }))}
                                className="text-input"
                              />
                            </div>
                            <div className="provider-field">
                              <label className="provider-field-label">API Key</label>
                              <div className="input-with-adornment">
                                <input
                                  type={secretKeys[idx] ? 'text' : 'password'}
                                  value={p.apiKey}
                                  onChange={(e) => updateProvider(idx, (pv) => ({ ...pv, apiKey: e.target.value }))}
                                  placeholder="sk-..."
                                  className="text-input"
                                />
                                <button
                                  type="button"
                                  className="input-adornment"
                                  onClick={() => setSecretKeys(prev => ({ ...prev, [idx]: !prev[idx] }))}
                                >
                                  {secretKeys[idx] ? <EyeInvisibleOutlined size={14} /> : <EyeOutlined size={14} />}
                                </button>
                              </div>
                            </div>
                            <div className="provider-field">
                              <label className="provider-field-label">Base URL</label>
                              <input
                                type="text"
                                value={p.baseUrl}
                                onChange={(e) => updateProvider(idx, (pv) => ({ ...pv, baseUrl: e.target.value }))}
                                className="text-input"
                              />
                            </div>
                            <div className="provider-field">
                              <label className="provider-field-label">
                                API Keys Pool
                                <span style={{ color: 'var(--n-stone)', fontWeight: 400, marginLeft: 4, fontSize: 11 }}>
                                  (add one by one)
                                </span>
                              </label>
                              <div className="api-key-rows">
                                {(p.apiKeys || []).map((key, kidx) => (
                                  <div key={kidx} className="input-with-adornment" style={{ marginBottom: 4 }}>
                                    <input
                                      type="password"
                                      value={key}
                                      onChange={(e) => updateProvider(idx, (pv) => ({
                                        ...pv,
                                        apiKeys: (pv.apiKeys || []).map((k, i) => i === kidx ? e.target.value : k),
                                      }))}
                                      className="text-input"
                                      placeholder="sk-..."
                                    />
                                    <button
                                      type="button"
                                      className="input-adornment"
                                      onClick={() => updateProvider(idx, (pv) => ({
                                        ...pv,
                                        apiKeys: (pv.apiKeys || []).filter((_, i) => i !== kidx),
                                      }))}
                                      title="Remove key"
                                    >
                                      <DeleteOutlined size={14} />
                                    </button>
                                  </div>
                                ))}
                                <button
                                  type="button"
                                  className="btn-sm"
                                  onClick={() => updateProvider(idx, (pv) => ({
                                    ...pv,
                                    apiKeys: [...(pv.apiKeys || []), ''],
                                  }))}
                                >
                                  <PlusOutlined size={12} />
                                  Add Key
                                </button>
                              </div>
                              <p className="settings-hint" style={{ marginTop: 4 }}>
                                {(p.apiKeys || []).length > 0 ? `${p.apiKeys.length} keys configured` : 'Leave empty to use single API key above'}
                              </p>
                            </div>
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </section>

              <section className="settings-section">
                <label className="settings-label">Model</label>
                <div className="input-with-adornment">
                  <select
                    value={settings.model}
                    onChange={(e) => setSettings((prev) => ({ ...prev, model: e.target.value }))}
                    disabled={isLoading}
                    className="text-input select-input"
                  >
                    <option value="" disabled>Select a model</option>
                    {models.map((m) => (
                      <option key={m.id} value={m.id}>{m.name || m.id}</option>
                    ))}
                  </select>
                  <button
                    type="button"
                    className="input-adornment"
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
                    title="Refresh models"
                  >
                    <RefreshOutlined size={16} style={{ animation: isLoading ? 'status-spin 900ms linear infinite' : undefined }} />
                  </button>
                </div>
              </section>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
                <section className="settings-section" style={{ marginBottom: 0 }}>
                  <label className="settings-label">
                    Max Tokens <span style={{ color: 'var(--n-stone)', fontWeight: 400 }}>({settings.maxTokens})</span>
                  </label>
                  <input
                    type="range"
                    min="256"
                    max="65536"
                    step="256"
                    value={settings.maxTokens}
                    onChange={(e) => setSettings((prev) => ({ ...prev, maxTokens: parseInt(e.target.value) }))}
                    style={{ width: '100%', accentColor: 'var(--n-primary)' }}
                  />
                </section>
                <section className="settings-section" style={{ marginBottom: 0 }}>
                  <label className="settings-label">
                    Temperature <span style={{ color: 'var(--n-stone)', fontWeight: 400 }}>({settings.temperature.toFixed(1)})</span>
                  </label>
                  <input
                    type="range"
                    min="0"
                    max="2"
                    step="0.1"
                    value={settings.temperature}
                    onChange={(e) => setSettings((prev) => ({ ...prev, temperature: parseFloat(e.target.value) }))}
                    style={{ width: '100%', accentColor: 'var(--n-primary)' }}
                  />
                </section>
              </div>

              <section className="settings-section">
                <label className="settings-label">Text-to-Speech</label>
                <div className="switch-row" style={{ marginBottom: 8 }}>
                  <span>Enable TTS</span>
                  <button
                    className={`switch ${settings.ttsEnabled ? 'on' : ''}`}
                    onClick={() => setSettings((prev) => ({ ...prev, ttsEnabled: !prev.ttsEnabled }))}
                    aria-pressed={settings.ttsEnabled}
                  >
                    <span className="switch-knob" />
                  </button>
                </div>
                {settings.ttsEnabled && (
                  <select
                    value={settings.ttsVoice}
                    onChange={(e) => setSettings((prev) => ({ ...prev, ttsVoice: e.target.value }))}
                    className="text-input select-input"
                  >
                    <option value="zh-CN">中文（普通话）</option>
                    <option value="en-US">English (US)</option>
                    <option value="en-GB">English (UK)</option>
                    <option value="ja-JP">日本語</option>
                    <option value="ko-KR">한국어</option>
                  </select>
                )}
              </section>

              <section className="settings-section">
                <label className="settings-label">Agent Mode</label>
                <div className="switch-row">
                  <span>Enable agent mode</span>
                  <button
                    className={`switch ${settings.agentMode ? 'on' : ''}`}
                    onClick={() => setSettings((prev) => ({ ...prev, agentMode: !prev.agentMode }))}
                    aria-pressed={settings.agentMode}
                  >
                    <span className="switch-knob" />
                  </button>
                </div>
              </section>

              {settings.agentMode && (
                <>
                  <section className="settings-section">
                    <label className="settings-label">Auto-learn</label>
                    <div className="switch-row">
                      <span>Automatically learn from conversations</span>
                      <button
                        className={`switch ${settings.agentSettings.autoLearn ? 'on' : ''}`}
                        onClick={() => setSettings((prev) => ({
                          ...prev,
                          agentSettings: { ...prev.agentSettings, autoLearn: !prev.agentSettings.autoLearn },
                        }))}
                        aria-pressed={settings.agentSettings.autoLearn}
                      >
                        <span className="switch-knob" />
                      </button>
                    </div>
                  </section>
                  <section className="settings-section">
                    <label className="settings-label">Auto-compact</label>
                    <div className="switch-row">
                      <span>Automatically compact context</span>
                      <button
                        className={`switch ${settings.agentSettings.autoCompact ? 'on' : ''}`}
                        onClick={() => setSettings((prev) => ({
                          ...prev,
                          agentSettings: { ...prev.agentSettings, autoCompact: !prev.agentSettings.autoCompact },
                        }))}
                        aria-pressed={settings.agentSettings.autoCompact}
                      >
                        <span className="switch-knob" />
                      </button>
                    </div>
                  </section>
                </>
              )}
            </>
          )}

          {activeTab === 'system' && (
            <>
              <section className="settings-section">
                <label className="settings-label">About</label>
                <p className="settings-hint">
                  Pi-AI Chat v0.1.0 — Multi-provider AI chat interface built with Go + React.
                </p>
              </section>
            </>
          )}
        </div>

        <div className="modal-footer">
          <button className="btn-primary" onClick={handleSave}>
            <SaveOutlined size={14} />
            {saved ? 'Saved!' : 'Save'}
          </button>
        </div>
      </div>
    </div>
  );
}
