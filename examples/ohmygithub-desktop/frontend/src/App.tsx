import React, { useState, useCallback, useEffect } from 'react';
import { API, AppSettings, GitHubAccount, Notification, PullRequest, Issue, Repo, WorkflowRun, Bookmark, formatRelativeTime } from './lib/api';
import Sidebar from './components/Sidebar';
import NotificationsPage from './pages/NotificationsPage';
import PullRequestsPage from './pages/PullRequestsPage';
import IssuesPage from './pages/IssuesPage';
import ActionsPage from './pages/ActionsPage';
import OverviewPage from './pages/OverviewPage';
import RepositoriesPage from './pages/RepositoriesPage';
import StarredReposPage from './pages/StarredReposPage';
import RepoDetailPage from './pages/RepoDetailPage';
import SettingsModal from './components/SettingsModal';
import PreviewPanel from './components/PreviewPanel';
import Toast from './components/Toast';

type Page = 'overview' | 'notifications' | 'pull-requests' | 'issues' | 'actions' | 'repositories' | 'starred' | 'repo-detail';
type NavHandler = (page: string) => void;

function App() {
  const [activePage, setActivePage] = useState<string>('overview');
  const [settings, setSettings] = useState<AppSettings | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [activeRepo, setActiveRepo] = useState<string>('');
  const [toasts, setToasts] = useState<Array<{id: string; message: string; type: string}>>([]);
  const [selectedItem, setSelectedItem] = useState<any>(null);
  const [previewOpen, setPreviewOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  // Load settings on mount
  useEffect(() => {
    loadSettings();
  }, []);

  // Apply theme / font size / code font to document root
  useEffect(() => {
    if (!settings) return;
    const root = document.documentElement;
    root.setAttribute('data-theme', settings.theme === 'light' ? 'light' : 'dark');
    if (settings.fontSize && settings.fontSize > 0) {
      root.style.setProperty('--app-font-size', `${settings.fontSize}px`);
      root.style.fontSize = `${settings.fontSize}px`;
    }
    if (settings.codeFont) {
      root.style.setProperty('--font-mono', `'${settings.codeFont}', monospace`);
    }
  }, [settings]);

  const loadSettings = async () => {
    try {
      const str = await API.GetSettings();
      const s = JSON.parse(str) as AppSettings;
      // 防御性处理：确保数组字段非 null，避免后续 .map() 崩溃
      if (!Array.isArray(s.accounts)) s.accounts = [];
      if (!Array.isArray(s.bookmarks)) s.bookmarks = [];
      if (!Array.isArray(s.starGroups)) s.starGroups = [];
      if (typeof s.fontSize !== 'number' || s.fontSize <= 0) s.fontSize = 14;
      if (!s.theme) s.theme = 'dark';
      setSettings(s);
    } catch (e) {
      console.error('Failed to load settings:', e);
      // 加载失败时使用默认设置，避免一直卡在 loading
      setSettings({
        accounts: [],
        activeAccount: 0,
        theme: 'dark',
        fontSize: 14,
        codeFont: 'JetBrains Mono, Fira Code, monospace',
        bookmarks: [],
        starGroups: [],
        windowWidth: 0,
        windowHeight: 0,
      });
    }
  };

  const addToast = useCallback((message: string, type: string = 'info') => {
    const id = `toast-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    setToasts(prev => [...prev, { id, message, type }]);
    setTimeout(() => {
      setToasts(prev => prev.filter(t => t.id !== id));
    }, 3000);
  }, []);

  const handleSelectItem = useCallback((item: any) => {
    setSelectedItem(item);
    setPreviewOpen(true);
  }, []);

  const handleClosePreview = useCallback(() => {
    setPreviewOpen(false);
    setSelectedItem(null);
  }, []);

  const handleOpenExternal = useCallback(async (url: string) => {
    try {
      await API.OpenExternal(url);
    } catch {
      addToast('Failed to open URL', 'error');
    }
  }, [addToast]);

  const handleUpdateSettings = useCallback(async (newSettings: AppSettings) => {
    try {
      await API.UpdateSettings(JSON.stringify(newSettings));
      setSettings(newSettings);
      addToast('Settings saved', 'success');
    } catch (e) {
      addToast('Failed to save settings', 'error');
    }
  }, [addToast]);

  if (!settings) {
    return (
      <div className="loading-spinner" style={{ height: '100vh' }}>
        <div className="spinner" />
      </div>
    );
  }

  const activeAccount: GitHubAccount | undefined = settings.accounts[settings.activeAccount];

  const currentUser = activeAccount?.username || 'Not signed in';
  const currentAvatar = activeAccount?.avatarUrl || '';

  const pageTitle: Record<string, string> = {
    overview: 'Overview',
    notifications: 'Notifications',
    'pull-requests': 'Pull Requests',
    issues: 'Issues',
    actions: 'Actions',
    repositories: 'Repositories',
    starred: 'Starred',
    'repo-detail': 'Repository',
  };

  return (
    <div className="app-layout">
      <Sidebar
        activePage={activePage}
        onNavigate={(p: string) => setActivePage(p)}
        settings={settings}
        onOpenSettings={() => setShowSettings(true)}
        onUpdateSettings={handleUpdateSettings}
        addToast={addToast}
      />

      <div className="main-area">
        <div className="main-header">
          {activePage !== 'overview' && (
            <button
              className="btn btn-ghost btn-sm"
              onClick={() => { setActivePage('overview'); setActiveRepo(''); }}
              title="返回 Overview"
              style={{ marginRight: 8, padding: '4px 10px' }}
            >
              ← 返回
            </button>
          )}
          <h2>{pageTitle[activePage]}</h2>
          {activeRepo && <span className="subtitle">{activeRepo}</span>}
          <div style={{ marginLeft: 'auto', display: 'flex', gap: 8, alignItems: 'center' }}>
            <div className="search-box">
              <svg className="search-icon" viewBox="0 0 16 16" fill="currentColor">
                <path d="M10.68 11.74a6 6 0 0 1-7.922-8.982 6 6 0 0 1 8.982 7.922l3.04 3.04a.749.749 0 0 1-.326 1.275.749.749 0 0 1-.734-.215l-3.04-3.04Zm-4.68-.74a4.5 4.5 0 1 0 0-9 4.5 4.5 0 0 0 0 9Z"/>
              </svg>
              <input
                className="input"
                placeholder="Search repositories..."
                value={searchQuery}
                onChange={e => setSearchQuery(e.target.value)}
                style={{ width: 240 }}
              />
            </div>
          </div>
        </div>

        <div className="main-content">
          {activePage === 'overview' && (
            <OverviewPage settings={settings} onNavigate={setActivePage} addToast={addToast} />
          )}
          {activePage === 'notifications' && (
            <NotificationsPage onSelect={handleSelectItem} addToast={addToast} />
          )}
          {activePage === 'pull-requests' && (
            <PullRequestsPage onSelect={handleSelectItem} addToast={addToast} activeRepo={activeRepo} />
          )}
          {activePage === 'issues' && (
            <IssuesPage onSelect={handleSelectItem} addToast={addToast} activeRepo={activeRepo} />
          )}
          {activePage === 'actions' && (
            <ActionsPage addToast={addToast} initialRepo={activeRepo} />
          )}
          {activePage === 'repositories' && (
            <RepositoriesPage
              searchQuery={searchQuery}
              onSearchChange={setSearchQuery}
              onSelect={(repo: Repo) => { setActiveRepo(repo.fullName); setActivePage('repo-detail'); }}
              addToast={addToast}
            />
          )}
          {activePage === 'starred' && settings && (
            <StarredReposPage
              addToast={addToast}
              starGroups={settings.starGroups || []}
              onGroupsChange={loadSettings}
              onSelectRepo={(repo) => { setActiveRepo(repo.fullName); setActivePage('repo-detail'); }}
            />
          )}
          {activePage === 'repo-detail' && activeRepo && (
            <RepoDetailPage
              repoFullName={activeRepo}
              addToast={addToast}
              onNavigate={setActivePage}
              onOpenExternal={handleOpenExternal}
              onBack={() => { setActivePage('repositories'); }}
            />
          )}
        </div>
      </div>

      <PreviewPanel
        open={previewOpen}
        item={selectedItem}
        onClose={handleClosePreview}
      />

      {showSettings && (
        <SettingsModal
          settings={settings}
          onSave={handleUpdateSettings}
          onClose={() => setShowSettings(false)}
          addToast={addToast}
        />
      )}

      <Toast toasts={toasts} />
    </div>
  );
}

export default App;
