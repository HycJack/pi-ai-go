import React from 'react';
import { API, AppSettings, GitHubAccount, Notification } from '../lib/api';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { ScrollArea } from './ui/scroll-area';
import {
  Compass,
  Bell,
  GitPullRequest,
  CircleDot,
  PlayCircle,
  BookMarked,
  Star,
  Bookmark,
  Plus,
  X,
  Settings,
  Search,
} from 'lucide-react';

interface SidebarProps {
  activePage: string;
  onNavigate: (page: string) => void;
  settings: AppSettings;
  onOpenSettings: () => void;
  onUpdateSettings: (settings: AppSettings) => void;
  addToast: (message: string, type?: string) => void;
}

const navItems = [
  { id: 'overview', label: 'Overview', icon: Compass },
  { id: 'notifications', label: 'Notifications', icon: Bell },
  { id: 'pull-requests', label: 'Pull Requests', icon: GitPullRequest },
  { id: 'issues', label: 'Issues', icon: CircleDot },
  { id: 'actions', label: 'Actions', icon: PlayCircle },
  { id: 'repositories', label: 'Repositories', icon: BookMarked },
  { id: 'starred', label: 'Starred', icon: Star },
  { id: 'search', label: 'Search', icon: Search },
];

const pageMapping: Record<string, string> = {
  overview: 'overview',
  notifications: 'notifications',
  'pull-requests': 'pull-requests',
  issues: 'issues',
  actions: 'actions',
  repositories: 'repositories',
  starred: 'starred',
};

export default function Sidebar({
  activePage,
  onNavigate,
  settings,
  onOpenSettings,
  onUpdateSettings,
  addToast,
}: SidebarProps) {
  const [showBookmarks, setShowBookmarks] = React.useState(true);
  const [editingBookmark, setEditingBookmark] = React.useState(false);
  const [newBmTitle, setNewBmTitle] = React.useState('');
  const [newBmUrl, setNewBmUrl] = React.useState('');
  const [unreadCount, setUnreadCount] = React.useState(0);

  React.useEffect(() => {
    const active = settings.accounts[settings.activeAccount];
    if (!active?.token) return;
    let cancelled = false;
    const fetchUnread = async () => {
      try {
        const str = await API.GetNotifications();
        if (cancelled) return;
        const list = JSON.parse(str) as Notification[];
        setUnreadCount(list.filter(n => !n.read).length);
      } catch {
        /* ignore */
      }
    };
    fetchUnread();
    const t = setInterval(fetchUnread, 60000);
    return () => {
      cancelled = true;
      clearInterval(t);
    };
  }, [settings.activeAccount, settings.accounts]);

  const handleAddBookmark = async () => {
    if (!newBmTitle.trim() || !newBmUrl.trim()) return;
    try {
      await API.AddBookmark(newBmTitle.trim(), newBmUrl.trim(), 'bookmark');
      const str = await API.GetSettings();
      onUpdateSettings(JSON.parse(str));
      addToast('Bookmark added', 'success');
      setNewBmTitle('');
      setNewBmUrl('');
      setEditingBookmark(false);
    } catch {
      addToast('Failed to add bookmark', 'error');
    }
  };

  const handleRemoveBookmark = async (id: string) => {
    try {
      await API.RemoveBookmark(id);
      const str = await API.GetSettings();
      onUpdateSettings(JSON.parse(str));
    } catch {
      addToast('Failed to remove bookmark', 'error');
    }
  };

  const handleBookmarkClick = async (bm: { id: string; title: string; url: string }) => {
    const url = bm.url.trim();
    if (!url) return;
    const repoMatch = url.match(/^([A-Za-z0-9._-]+)\/([A-Za-z0-9._-]+)$/);
    const fullUrl = repoMatch
      ? `https://github.com/${repoMatch[0]}`
      : url.startsWith('http')
        ? url
        : `https://${url}`;
    try {
      await API.OpenExternal(fullUrl);
    } catch {
      addToast('Failed to open bookmark URL', 'error');
    }
  };

  const activeAccount: GitHubAccount | undefined = settings.accounts[settings.activeAccount];

  return (
    <div className="flex h-full w-[260px] min-w-[260px] flex-col border-r border-border bg-sidebar text-sidebar-foreground select-none">
      {/* Header */}
      <div className="flex h-12 items-center gap-2 border-b border-border px-4 shrink-0">
        <div className="flex h-6 w-6 items-center justify-center rounded-full bg-gradient-to-br from-success to-success/80 text-xs font-bold text-white">
          OG
        </div>
        <h1 className="text-sm font-semibold">Oh My GitHub</h1>
      </div>

      {/* Account */}
      <div className="border-b border-border p-2 shrink-0">
        {activeAccount ? (
          <button
            onClick={onOpenSettings}
            className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors hover:bg-sidebar-accent cursor-pointer"
          >
            <div className="flex h-5 w-5 shrink-0 overflow-hidden rounded-full bg-border">
              {activeAccount.avatarUrl ? (
                <img src={activeAccount.avatarUrl} alt="" className="h-full w-full object-cover" />
              ) : null}
            </div>
            <span className="flex-1 truncate font-medium text-sidebar-foreground">
              {activeAccount.username}
            </span>
            <Settings className="h-4 w-4 text-muted-foreground" />
          </button>
        ) : (
          <Button onClick={onOpenSettings} className="w-full justify-center" size="sm">
            Sign in with GitHub
          </Button>
        )}
      </div>

      {/* Navigation */}
      <ScrollArea className="flex-1">
        <nav className="p-2">
          {navItems.map((item) => {
            const Icon = item.icon;
            const isActive =
              activePage === pageMapping[item.id] ||
              activePage.startsWith(item.id + '-');
            return (
              <button
                key={item.id}
                onClick={() => onNavigate(item.id)}
                className={`flex w-full items-center gap-2.5 rounded-md px-3 py-1.5 text-sm transition-colors cursor-pointer ${
                  isActive
                    ? 'bg-sidebar-accent text-sidebar-accent-foreground font-medium'
                    : 'text-muted-foreground hover:bg-sidebar-accent/50 hover:text-sidebar-foreground'
                }`}
              >
                <Icon className={`h-[18px] w-[18px] shrink-0 ${isActive ? 'text-primary' : 'opacity-80'}`} />
                <span className="flex-1 text-left">{item.label}</span>
                {item.id === 'notifications' && unreadCount > 0 && (
                  <span className="flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-primary px-1.5 text-xs font-semibold text-primary-foreground">
                    {unreadCount > 99 ? '99+' : unreadCount}
                  </span>
                )}
              </button>
            );
          })}

          {/* Bookmarks */}
          <div className="mt-6">
            <div className="flex items-center justify-between px-3 py-1">
              <button
                onClick={() => setShowBookmarks(!showBookmarks)}
                className="cursor-pointer text-xs font-semibold uppercase tracking-wider text-muted-foreground hover:text-foreground"
              >
                Bookmarks
              </button>
              <button
                onClick={() => setEditingBookmark(!editingBookmark)}
                className="cursor-pointer text-primary hover:text-primary/80"
              >
                <Plus className="h-4 w-4" />
              </button>
            </div>

            {editingBookmark && (
              <div className="mb-2 px-3">
                <Input
                  placeholder="Title"
                  value={newBmTitle}
                  onChange={(e) => setNewBmTitle(e.target.value)}
                  className="mb-1 h-7 text-xs"
                />
                <Input
                  placeholder="URL or repo"
                  value={newBmUrl}
                  onChange={(e) => setNewBmUrl(e.target.value)}
                  className="mb-1 h-7 text-xs"
                />
                <Button onClick={handleAddBookmark} className="w-full bg-success hover:bg-success/90 text-success-foreground" size="sm">
                  Add
                </Button>
              </div>
            )}

            {showBookmarks &&
              settings.bookmarks.map((bm) => (
                <div
                  key={bm.id}
                  onClick={() => handleBookmarkClick(bm)}
                  title={bm.url}
                  className="group flex items-center gap-2.5 rounded-md px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-sidebar-accent/50 hover:text-foreground cursor-pointer"
                >
                  <Bookmark className="h-4 w-4 shrink-0" />
                  <span className="flex-1 truncate">{bm.title}</span>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      handleRemoveBookmark(bm.id);
                    }}
                    className="opacity-0 group-hover:opacity-60 hover:!opacity-100 transition-opacity cursor-pointer"
                  >
                    <X className="h-[18px] w-[18px] text-destructive" />
                  </button>
                </div>
              ))}

            {showBookmarks && settings.bookmarks.length === 0 && (
              <div className="px-3 py-2 text-xs text-muted-foreground">
                No bookmarks yet. Click + to add.
              </div>
            )}
          </div>
        </nav>
      </ScrollArea>

      {/* Footer */}
      <div className="border-t border-border p-2 shrink-0">
        <Button
          onClick={onOpenSettings}
          variant="ghost"
          className="w-full justify-center"
          size="sm"
        >
          <Settings className="h-4 w-4" />
          Settings
        </Button>
      </div>
    </div>
  );
}
