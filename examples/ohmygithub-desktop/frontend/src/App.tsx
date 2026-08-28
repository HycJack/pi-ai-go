import React, { useState, useCallback, useEffect, useRef } from 'react';
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
import { Button } from './components/ui/button';
import { Sun, Moon, ArrowLeft } from 'lucide-react';

type Page = 'overview' | 'notifications' | 'pull-requests' | 'issues' | 'actions' | 'repositories' | 'starred' | 'repo-detail';
type NavHandler = (page: string) => void;

function App() {
  const [activePage, setActivePage] = useState<string>('overview');
  const [pageHistory, setPageHistory] = useState<string[]>([]);
  const [settings, setSettings] = useState<AppSettings | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [activeRepo, setActiveRepo] = useState<string>('');
  const [toasts, setToasts] = useState<Array<{ id: string; message: string; type: string }>>([]);
  const [selectedItem, setSelectedItem] = useState<any>(null);
  const [previewOpen, setPreviewOpen] = useState(false);

  // Persisted filter state (survives page navigation)
  const [starredFilters, setStarredFilters] = useState({
    keyword: '',
    language: '',
    sort: 'starred' as 'starred' | 'name' | 'stars' | 'updated',
    groupID: '',
  });
  const [repoFilters, setRepoFilters] = useState({
    keyword: '',
    language: '',
    sort: 'updated' as 'updated' | 'created' | 'full_name',
  });

  // Navigate to a new page, pushing current page to history
  const navigateTo = useCallback((page: string, repo?: string) => {
    setPageHistory(prev => [...prev, activePage]);
    setActivePage(page);
    if (repo !== undefined) setActiveRepo(repo);
  }, [activePage]);

  // Go back to previous page in history
  const goBack = useCallback(() => {
    setPageHistory(prev => {
      if (prev.length === 0) return prev;
      const newHistory = [...prev];
      const previousPage = newHistory.pop()!;
      setActivePage(previousPage);
      // Clear repo if going back to a non-repo-detail page
      if (previousPage !== 'repo-detail') {
        setActiveRepo('');
      }
      return newHistory;
    });
  }, []);

  // Navigate from sidebar (no history push for sidebar clicks)
  const navigateFromSidebar = useCallback((page: string) => {
    setActivePage(page);
    setActiveRepo('');
    setPageHistory([]); // Reset history when using sidebar
    if (page === 'starred') {
      setStarredFilters({ keyword: '', language: '', sort: 'starred', groupID: '' });
    } else if (page === 'repositories') {
      setRepoFilters({ keyword: '', language: '', sort: 'updated' });
    }
  }, []);

  useEffect(() => {
    loadSettings();
  }, []);

  useEffect(() => {
    if (!settings) return;
    const root = document.documentElement;
    if (settings.theme === 'light') {
      root.classList.add('light');
      root.classList.remove('dark');
    } else {
      root.classList.add('dark');
      root.classList.remove('light');
    }
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
      if (!Array.isArray(s.accounts)) s.accounts = [];
      if (!Array.isArray(s.bookmarks)) s.bookmarks = [];
      if (!Array.isArray(s.starGroups)) s.starGroups = [];
      if (typeof s.fontSize !== 'number' || s.fontSize <= 0) s.fontSize = 14;
      if (!s.theme) s.theme = 'dark';
      setSettings(s);
    } catch (e) {
      console.error('Failed to load settings:', e);
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
      <div className="flex h-screen items-center justify-center">
        <div className="h-6 w-6 animate-spin rounded-full border-[3px] border-border border-t-primary" />
      </div>
    );
  }

  const activeAccount: GitHubAccount | undefined = settings.accounts[settings.activeAccount];
  const isDark = settings.theme !== 'light';
  const canGoBack = pageHistory.length > 0;

  const handleToggleTheme = async () => {
    const newTheme = isDark ? 'light' : 'dark';
    const newSettings = { ...settings, theme: newTheme };
    await handleUpdateSettings(newSettings);
  };

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
    <div className="flex h-screen overflow-hidden bg-background text-foreground">
      <Sidebar
        activePage={activePage}
        onNavigate={navigateFromSidebar}
        settings={settings}
        onOpenSettings={() => setShowSettings(true)}
        onUpdateSettings={handleUpdateSettings}
        addToast={addToast}
      />

      <div className="flex flex-1 flex-col min-w-0">
        {/* Header */}
        <div className="flex h-12 items-center gap-3 border-b border-border bg-background px-4 shrink-0">
          {canGoBack && (
            <Button
              variant="ghost"
              size="sm"
              onClick={goBack}
              title="返回上一页"
            >
              <ArrowLeft className="h-4 w-4" />
              返回
            </Button>
          )}
          <h2 className="text-base font-semibold">{pageTitle[activePage]}</h2>
          {activeRepo && (
            <span className="text-xs text-muted-foreground">{activeRepo}</span>
          )}
          <div className="ml-auto flex items-center gap-2">
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              onClick={handleToggleTheme}
              title={isDark ? '切换到浅色主题' : '切换到深色主题'}
              aria-label="Toggle theme"
            >
              {isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            </Button>
          </div>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-4">
          {activePage === 'overview' && (
            <OverviewPage settings={settings} onNavigate={navigateTo} addToast={addToast} />
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
              filters={repoFilters}
              onFiltersChange={setRepoFilters}
              onSelect={(repo: Repo) => navigateTo('repo-detail', repo.fullName)}
              addToast={addToast}
            />
          )}
          {activePage === 'starred' && settings && (
            <StarredReposPage
              addToast={addToast}
              starGroups={settings.starGroups || []}
              onGroupsChange={loadSettings}
              filters={starredFilters}
              onFiltersChange={setStarredFilters}
              onSelectRepo={(repo) => navigateTo('repo-detail', repo.fullName)}
            />
          )}
          {activePage === 'repo-detail' && activeRepo && (
            <RepoDetailPage
              repoFullName={activeRepo}
              addToast={addToast}
              onNavigate={navigateTo}
              onOpenExternal={handleOpenExternal}
            />
          )}
        </div>
      </div>

      <PreviewPanel open={previewOpen} item={selectedItem} onClose={handleClosePreview} />

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
