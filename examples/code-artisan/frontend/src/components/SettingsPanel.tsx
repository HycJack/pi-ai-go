import { useState } from 'react';
import { CloseOutlined } from '../icons';
import type { Settings, ProviderSetting } from '../types';

interface SettingsPanelProps {
  settings: Settings;
  onSave: (settings: Settings) => void;
  onClose: () => void;
}

const PROVIDER_TYPES = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'openai-compatible', label: 'OpenAI 兼容' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'google', label: 'Google' },
  { value: 'deepseek', label: 'DeepSeek' },
];

export default function SettingsPanel({ settings, onSave, onClose }: SettingsPanelProps) {
  const [local, setLocal] = useState<Settings>({ ...settings, providers: settings.providers.map((p) => ({ ...p })) });

  const updateProvider = (idx: number, field: keyof ProviderSetting, value: string) => {
    const providers = [...local.providers];
    providers[idx] = { ...providers[idx], [field]: value };
    setLocal({ ...local, providers });
  };

  const addProvider = () => {
    setLocal({
      ...local,
      providers: [...local.providers, { name: '新服务商', type: 'openai', apiKey: '', baseUrl: 'https://api.openai.com/v1' }],
    });
  };

  const removeProvider = (idx: number) => {
    const providers = local.providers.filter((_, i) => i !== idx);
    const newIndex = local.currentProviderIndex >= providers.length ? Math.max(0, providers.length - 1) : local.currentProviderIndex;
    setLocal({ ...local, providers, currentProviderIndex: newIndex });
  };

  return (
    <div className="settings-overlay" onClick={onClose}>
      <div className="settings-panel" onClick={(e) => e.stopPropagation()}>
        <div className="settings-header">
          <h2>设置</h2>
          <button className="settings-close" onClick={onClose}>
            <CloseOutlined size={18} />
          </button>
        </div>

        <div className="settings-body">
          <section className="settings-section">
            <h3>AI 服务商</h3>
            {local.providers.map((provider, idx) => (
              <div key={idx} className="provider-card">
                <div className="provider-card-header">
                  <span>服务商 {idx + 1}</span>
                  {local.providers.length > 1 && (
                    <button className="provider-remove" onClick={() => removeProvider(idx)}>删除</button>
                  )}
                </div>
                <div className="form-group">
                  <label>名称</label>
                  <input
                    type="text"
                    value={provider.name}
                    onChange={(e) => updateProvider(idx, 'name', e.target.value)}
                    placeholder="OpenAI"
                  />
                </div>
                <div className="form-group">
                  <label>类型</label>
                  <select
                    value={provider.type}
                    onChange={(e) => updateProvider(idx, 'type', e.target.value)}
                  >
                    {PROVIDER_TYPES.map((t) => (
                      <option key={t.value} value={t.value}>{t.label}</option>
                    ))}
                  </select>
                </div>
                <div className="form-group">
                  <label>API Key</label>
                  <input
                    type="password"
                    value={provider.apiKey}
                    onChange={(e) => updateProvider(idx, 'apiKey', e.target.value)}
                    placeholder="sk-..."
                  />
                </div>
                <div className="form-group">
                  <label>Base URL</label>
                  <input
                    type="text"
                    value={provider.baseUrl}
                    onChange={(e) => updateProvider(idx, 'baseUrl', e.target.value)}
                    placeholder="https://api.openai.com/v1"
                  />
                </div>
                {local.currentProviderIndex !== idx && (
                  <button
                    className="set-active-btn"
                    onClick={() => setLocal({ ...local, currentProviderIndex: idx })}
                  >
                    设为当前
                  </button>
                )}
                {local.currentProviderIndex === idx && (
                  <span className="active-badge">当前使用的服务商</span>
                )}
              </div>
            ))}
            <button className="add-provider-btn" onClick={addProvider}>
              + 添加服务商
            </button>
          </section>

          <section className="settings-section">
            <h3>模型设置</h3>
            <div className="form-group">
              <label>模型名称</label>
              <input
                type="text"
                value={local.model}
                onChange={(e) => setLocal({ ...local, model: e.target.value })}
                placeholder="gpt-4o-mini"
              />
            </div>
            <div className="form-row">
              <div className="form-group">
                <label>最大 Token</label>
                <input
                  type="number"
                  value={local.maxTokens}
                  onChange={(e) => setLocal({ ...local, maxTokens: parseInt(e.target.value) || 4096 })}
                  min={1}
                  max={131072}
                />
              </div>
              <div className="form-group">
                <label>温度</label>
                <input
                  type="number"
                  value={local.temperature}
                  onChange={(e) => setLocal({ ...local, temperature: parseFloat(e.target.value) || 1.0 })}
                  min={0}
                  max={2}
                  step={0.1}
                />
              </div>
            </div>
          </section>
        </div>

        <div className="settings-footer">
          <button className="settings-cancel" onClick={onClose}>取消</button>
          <button className="settings-save" onClick={() => onSave(local)}>保存</button>
        </div>
      </div>
    </div>
  );
}
