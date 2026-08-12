import React, { useState, useEffect, useCallback } from 'react';
import { API, AppSettings } from '../lib/api';

interface SettingsModalProps {
  settings: AppSettings;
  onSave: (settings: AppSettings) => void;
  onClose: () => void;
  addToast: (message: string, type?: string) => void;
}

export default function SettingsModal({ settings, onSave, onClose, addToast }: SettingsModalProps) {
  const [localSettings, setLocalSettings] = useState<AppSettings>({ ...settings });
  const [newToken, setNewToken] = useState('');
  const [addingAccount, setAddingAccount] = useState(false);

  const handleSave = () => {
    onSave(localSettings);
    onClose();
  };

  const handleAddAccount = async () => {
    if (!newToken.trim()) return;
    setAddingAccount(true);
    try {
      const username = await API.AddAccount(newToken.trim());
      const str = await API.GetSettings();
      const updated = JSON.parse(str);
      setLocalSettings(updated);
      onSave(updated);
      addToast(`Signed in as ${username}`, 'success');
      setNewToken('');
    } catch (e: any) {
      addToast('Failed to add account: ' + (e?.message || e), 'error');
    } finally {
      setAddingAccount(false);
    }
  };

  const handleRemoveAccount = async (index: number) => {
    try {
      await API.RemoveAccount(index);
      const str = await API.GetSettings();
      const updated = JSON.parse(str);
      setLocalSettings(updated);
      onSave(updated);
      addToast('Account removed', 'success');
    } catch { addToast('Failed to remove account', 'error'); }
  };

  const handleSwitchAccount = async (index: number) => {
    try {
      await API.SwitchAccount(index);
      const str = await API.GetSettings();
      const updated = JSON.parse(str);
      setLocalSettings(updated);
      onSave(updated);
    } catch { addToast('Failed to switch account', 'error'); }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Settings</h2>
          <button className="btn btn-ghost btn-sm btn-icon" onClick={onClose}>
            <svg viewBox="0 0 24 24" fill="currentColor" width="16" height="16">
              <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
            </svg>
          </button>
        </div>
        <div className="modal-body">
          <div className="settings-group">
            <h3 style={{ fontSize: 14, fontWeight: 600, marginBottom: 8, color: 'var(--text-primary)' }}>Accounts</h3>
            {localSettings.accounts.length === 0 && (
              <p style={{ color: 'var(--text-tertiary)', fontSize: 13, marginBottom: 8 }}>
                No accounts configured. Add a GitHub Personal Access Token to get started.
              </p>
            )}
            {localSettings.accounts.map((acc, i) => (
              <div key={i} style={{
                display: 'flex', alignItems: 'center', gap: 8, padding: '8px 12px',
                background: 'var(--bg-overlay)', borderRadius: 'var(--radius-sm)',
                marginBottom: 6, border: i === localSettings.activeAccount ? '1px solid var(--border-accent)' : '1px solid var(--border-muted)'
              }}>
                <div className="account-avatar" style={{ width: 24, height: 24 }}>
                  {acc.avatarUrl ? <img src={acc.avatarUrl} alt="" /> : null}
                </div>
                <span style={{ flex: 1, fontSize: 13, fontWeight: 500 }}>{acc.username}</span>
                {i === localSettings.activeAccount && (
                  <span style={{ fontSize: 10, color: 'var(--text-link)', fontWeight: 600, background: 'rgba(31,111,235,0.15)', padding: '1px 6px', borderRadius: 4 }}>ACTIVE</span>
                )}
                <button className="btn btn-ghost btn-sm" onClick={() => handleSwitchAccount(i)} disabled={i === localSettings.activeAccount}
                  style={{ fontSize: 11 }}>Switch</button>
                <button className="btn btn-danger btn-sm" onClick={() => handleRemoveAccount(i)}
                  style={{ fontSize: 11 }}>Remove</button>
              </div>
            ))}
            {addingAccount ? (
              <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
                <input className="input" value={newToken} onChange={e => setNewToken(e.target.value)}
                  placeholder="Paste GitHub Personal Access Token..." style={{ flex: 1 }} disabled />
                <div className="spinner" style={{ width: 20, height: 20 }} />
              </div>
            ) : (
              <div style={{ marginTop: 8 }}>
                <div style={{ display: 'flex', gap: 8 }}>
                  <input className="input" value={newToken} onChange={e => setNewToken(e.target.value)}
                    placeholder="Paste GitHub Personal Access Token..." style={{ flex: 1 }} />
                  <button className="btn btn-primary" onClick={handleAddAccount}>Add</button>
                </div>
                <p style={{ fontSize: 11, color: 'var(--text-tertiary)', marginTop: 6, lineHeight: 1.5 }}>
                  创建 token 时请勾选以下 scope： <code style={{ color: 'var(--text-link)' }}>repo</code>、
                  <code style={{ color: 'var(--text-link)' }}> notifications</code>、
                  <code style={{ color: 'var(--text-link)' }}> read:user</code>、
                  <code style={{ color: 'var(--text-link)' }}> workflow</code>
                  <br />
                  <a href="https://github.com/settings/tokens/new?scopes=repo,notifications,read:user,workflow&description=OhMyGitHub-Desktop"
                    onClick={(e) => { e.preventDefault(); API.OpenExternal('https://github.com/settings/tokens/new?scopes=repo,notifications,read:user,workflow&description=OhMyGitHub-Desktop'); }}
                    style={{ color: 'var(--text-link)', textDecoration: 'underline', cursor: 'pointer' }}>
                    点此直接创建带正确权限的 token ↗
                  </a>
                </p>
              </div>
            )}
          </div>

          <div className="settings-group">
            <label>Theme</label>
            <select className="select" value={localSettings.theme}
              onChange={e => setLocalSettings({ ...localSettings, theme: e.target.value })}>
              <option value="dark">Dark</option>
              <option value="light">Light</option>
              <option value="system">System</option>
            </select>
          </div>

          <div className="settings-group">
            <label>Font Size</label>
            <input className="input" type="number" min={12} max={20} value={localSettings.fontSize}
              onChange={e => setLocalSettings({ ...localSettings, fontSize: parseInt(e.target.value) || 14 })} />
            <div className="help-text">Adjust the base font size (12–20px)</div>
          </div>

          <div className="settings-group">
            <label>Code Font</label>
            <input className="input" value={localSettings.codeFont}
              onChange={e => setLocalSettings({ ...localSettings, codeFont: e.target.value })} />
            <div className="help-text">Font family for code blocks (e.g., JetBrains Mono, Fira Code)</div>
          </div>
        </div>
        <div className="modal-footer">
          <button className="btn" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={handleSave}>Save Changes</button>
        </div>
      </div>
    </div>
  );
}
