import { useState, useEffect } from 'react';
import type { Settings, ProviderConfig } from '../types';
import { PROVIDER_TYPES } from '../types';
import { GetModels } from '../../wailsjs/go/main/App';

interface SettingsPanelProps {
  isOpen: boolean;
  onClose: () => void;
  settings: Settings;
  onSave: (settings: Settings) => void;
}

export default function SettingsPanel({ isOpen, onClose, settings, onSave }: SettingsPanelProps) {
  const [local, setLocal] = useState<Settings>({ ...settings });
  const [models, setModels] = useState<{ id: string; name: string }[]>([]);

  useEffect(() => {
    if (isOpen) setLocal({ ...settings });
  }, [isOpen, settings]);

  useEffect(() => {
    if (!isOpen) return;
    const cp = local.providers[local.currentProviderIndex];
    if (!cp) return;
    GetModels({ provider: cp.type, baseUrl: cp.baseUrl, apiKey: cp.apiKey })
      .then((list) => {
        if (list && list.length > 0) setModels(list);
      })
      .catch(() => {});
  }, [isOpen, local.currentProviderIndex, local.providers]);

  if (!isOpen) return null;

  const cp = local.providers[local.currentProviderIndex] || local.providers[0];

  const updateProvider = (patch: Partial<ProviderConfig>) => {
    const providers = [...local.providers];
    if (providers.length === 0) return;
    providers[local.currentProviderIndex] = { ...providers[local.currentProviderIndex], ...patch };
    setLocal({ ...local, providers });
  };

  const handleSave = () => {
    onSave(local);
    onClose();
  };

  return (
    <div className="settings-overlay" onClick={onClose}>
      <div className="settings-panel" onClick={(e) => e.stopPropagation()}>
        <h2 className="settings-title">设置</h2>
        <div className="settings-group">
          <div className="settings-field">
            <label>提供商类型</label>
            <select
              value={cp?.type || ''}
              onChange={(e) => {
                const t = PROVIDER_TYPES.find((p) => p.type === e.target.value);
                if (t) updateProvider({ name: t.name, type: t.type, baseUrl: t.baseUrl });
              }}
            >
              {PROVIDER_TYPES.map((p) => (
                <option key={p.type} value={p.type}>{p.name}</option>
              ))}
            </select>
          </div>
          <div className="settings-field">
            <label>API Key</label>
            <input
              type="password"
              value={cp?.apiKey || ''}
              onChange={(e) => updateProvider({ apiKey: e.target.value })}
              placeholder="输入 API Key"
            />
          </div>
          <div className="settings-field">
            <label>Base URL</label>
            <input
              value={cp?.baseUrl || ''}
              onChange={(e) => updateProvider({ baseUrl: e.target.value })}
              placeholder="https://api.openai.com/v1"
            />
          </div>
          <div className="settings-field">
            <label>模型</label>
            <select
              value={local.model}
              onChange={(e) => setLocal({ ...local, model: e.target.value })}
            >
              {models.map((m) => (
                <option key={m.id} value={m.id}>{m.name}</option>
              ))}
              {models.length === 0 && (
                <option value={local.model}>{local.model || 'gpt-4o-mini'}</option>
              )}
            </select>
          </div>
          <div className="settings-field">
            <label>Temperature ({local.temperature})</label>
            <input
              type="range"
              min="0"
              max="2"
              step="0.1"
              value={local.temperature}
              onChange={(e) => setLocal({ ...local, temperature: parseFloat(e.target.value) })}
            />
          </div>
          <div className="settings-field">
            <label>Max Tokens</label>
            <input
              type="number"
              value={local.maxTokens}
              onChange={(e) => setLocal({ ...local, maxTokens: parseInt(e.target.value) || 4096 })}
            />
          </div>
        </div>
        <div className="settings-actions">
          <button className="btn-cancel" onClick={onClose}>取消</button>
          <button className="btn-save" onClick={handleSave}>保存</button>
        </div>
      </div>
    </div>
  );
}
