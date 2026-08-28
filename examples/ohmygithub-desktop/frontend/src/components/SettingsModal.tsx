import React, { useState, useEffect, useCallback } from 'react';
import { API, AppSettings } from '../lib/api';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from './ui/dialog';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Select } from './ui/select';
import { ScrollArea } from './ui/scroll-area';
import { Badge } from './ui/badge';
import { Trash2, ArrowRightLeft } from 'lucide-react';

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
    } catch {
      addToast('Failed to remove account', 'error');
    }
  };

  const handleSwitchAccount = async (index: number) => {
    try {
      await API.SwitchAccount(index);
      const str = await API.GetSettings();
      const updated = JSON.parse(str);
      setLocalSettings(updated);
      onSave(updated);
    } catch {
      addToast('Failed to switch account', 'error');
    }
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-[520px] max-h-[80vh]">
        <DialogHeader>
          <DialogTitle>Settings</DialogTitle>
        </DialogHeader>

        <ScrollArea className="max-h-[60vh] pr-4">
          <div className="space-y-6">
            {/* Accounts */}
            <div className="space-y-3">
              <h3 className="text-sm font-semibold">Accounts</h3>
              {localSettings.accounts.length === 0 && (
                <p className="text-xs text-muted-foreground">
                  No accounts configured. Add a GitHub Personal Access Token to get started.
                </p>
              )}
              {localSettings.accounts.map((acc, i) => (
                <div
                  key={i}
                  className={`flex items-center gap-2 rounded-md border p-3 ${
                    i === localSettings.activeAccount
                      ? 'border-primary bg-primary/5'
                      : 'border-border bg-muted/50'
                  }`}
                >
                  <div className="flex h-6 w-6 shrink-0 overflow-hidden rounded-full bg-border">
                    {acc.avatarUrl ? (
                      <img src={acc.avatarUrl} alt="" className="h-full w-full object-cover" />
                    ) : null}
                  </div>
                  <span className="flex-1 text-sm font-medium">{acc.username}</span>
                  {i === localSettings.activeAccount && (
                    <Badge variant="secondary" className="text-xs">
                      ACTIVE
                    </Badge>
                  )}
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleSwitchAccount(i)}
                    disabled={i === localSettings.activeAccount}
                    className="h-7 text-xs"
                  >
                    <ArrowRightLeft className="h-3 w-3" />
                    Switch
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleRemoveAccount(i)}
                    className="h-7 text-xs text-destructive hover:text-destructive"
                  >
                    <Trash2 className="h-3 w-3" />
                    Remove
                  </Button>
                </div>
              ))}

              {addingAccount ? (
                <div className="flex items-center gap-2">
                  <Input
                    value={newToken}
                    onChange={(e) => setNewToken(e.target.value)}
                    placeholder="Paste GitHub Personal Access Token..."
                    className="flex-1"
                    disabled
                  />
                  <div className="h-5 w-5 animate-spin rounded-full border-2 border-border border-t-primary" />
                </div>
              ) : (
                <div className="space-y-2">
                  <div className="flex gap-2">
                    <Input
                      value={newToken}
                      onChange={(e) => setNewToken(e.target.value)}
                      placeholder="Paste GitHub Personal Access Token..."
                      className="flex-1"
                    />
                    <Button onClick={handleAddAccount}>Add</Button>
                  </div>
                  <p className="text-xs leading-relaxed text-muted-foreground">
                    创建 token 时请勾选以下 scope：
                    <code className="text-primary">repo</code>、
                    <code className="text-primary">notifications</code>、
                    <code className="text-primary">read:user</code>、
                    <code className="text-primary">workflow</code>
                    <br />
                    <a
                      href="https://github.com/settings/tokens/new?scopes=repo,notifications,read:user,workflow&description=OhMyGitHub-Desktop"
                      onClick={(e) => {
                        e.preventDefault();
                        API.OpenExternal(
                          'https://github.com/settings/tokens/new?scopes=repo,notifications,read:user,workflow&description=OhMyGitHub-Desktop'
                        );
                      }}
                      className="text-primary underline underline-offset-2 hover:text-primary/80 cursor-pointer"
                    >
                      点此直接创建带正确权限的 token ↗
                    </a>
                  </p>
                </div>
              )}
            </div>

            {/* Theme */}
            <div className="space-y-2">
              <Label>Theme</Label>
              <Select
                value={localSettings.theme}
                onChange={(e) => setLocalSettings({ ...localSettings, theme: e.target.value })}
              >
                <option value="dark">Dark</option>
                <option value="light">Light</option>
                <option value="system">System</option>
              </Select>
            </div>

            {/* Font Size */}
            <div className="space-y-2">
              <Label>Font Size</Label>
              <Input
                type="number"
                min={12}
                max={20}
                value={localSettings.fontSize}
                onChange={(e) =>
                  setLocalSettings({ ...localSettings, fontSize: parseInt(e.target.value) || 14 })
                }
              />
              <p className="text-xs text-muted-foreground">
                Adjust the base font size (12–20px)
              </p>
            </div>

            {/* Code Font */}
            <div className="space-y-2">
              <Label>Code Font</Label>
              <Input
                value={localSettings.codeFont}
                onChange={(e) => setLocalSettings({ ...localSettings, codeFont: e.target.value })}
              />
              <p className="text-xs text-muted-foreground">
                Font family for code blocks (e.g., JetBrains Mono, Fira Code)
              </p>
            </div>
          </div>
        </ScrollArea>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={handleSave}>Save Changes</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
