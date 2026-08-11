import React from 'react';
import { API, AppSettings, GitHubAccount } from '../lib/api';

interface SidebarProps {
  activePage: string;
  onNavigate: (page: string) => void;
  settings: AppSettings;
  onOpenSettings: () => void;
  onUpdateSettings: (settings: AppSettings) => void;
  addToast: (message: string, type?: string) => void;
}

const navItems = [
  { id: 'overview', label: 'Overview', icon: 'compass' },
  { id: 'notifications', label: 'Notifications', icon: 'bell' },
  { id: 'pull-requests', label: 'Pull Requests', icon: 'git-pull-request' },
  { id: 'issues', label: 'Issues', icon: 'circle-dot' },
  { id: 'actions', label: 'Actions', icon: 'play-circle' },
  { id: 'repositories', label: 'Repositories', icon: 'book' },
];

const Icon = ({ name }: { name: string }) => {
  const paths: Record<string, string> = {
    compass: 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 5-5v3h4v4h-4v3zm7-3l-5 5v-3h-4v-4h4v-3l5 5z',
    bell: 'M12 22c1.1 0 2-.9 2-2h-4c0 1.1.89 2 2 2zm6-6v-5c0-3.07-1.64-5.64-4.5-6.32V4c0-.83-.67-1.5-1.5-1.5s-1.5.67-1.5 1.5v.68C7.63 5.36 6 7.92 6 11v5l-2 2v1h16v-1l-2-2z',
    'git-pull-request': 'M21 8c0-1.66-1.34-3-3-3s-3 1.34-3 3c0 1.3.84 2.4 2 2.82V15c0 .55-.45 1-1 1h-2.18c-.41-1.16-1.52-2-2.82-2s-2.4.84-2.82 2H8c-.55 0-1-.45-1-1v-4.18c1.16-.41 2-1.52 2-2.82C9 7.34 7.66 6 6 6S3 7.34 3 9c0 1.3.84 2.4 2 2.82V15c0 1.66 1.34 3 3 3h2.18c.41 1.16 1.52 2 2.82 2s2.4-.84 2.82-2H16c1.66 0 3-1.34 3-3v-4.18A2.99 2.99 0 0 0 21 8zm-15 1c-.55 0-1-.45-1-1s.45-1 1-1 1 .45 1 1-.45 1-1 1z',
    'circle-dot': 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 15c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5z',
    'play-circle': 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 14.5v-9l6 4.5-6 4.5z',
    book: 'M18 2H6c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 18H6V4h2v8l2.5-1.5L13 12V4h5v16z',
    bookmark: 'M17 3H7c-1.1 0-2 .9-2 2v16l7-3 7 3V5c0-1.1-.9-2-2-2z',
    add: 'M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z',
    close: 'M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z',
    gear: 'M19.14 12.94c.04-.3.06-.62.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58a.49.49 0 0 0 .12-.61l-1.92-3.32a.49.49 0 0 0-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94L14.4 2.8a.501.501 0 0 0-.48-.41h-3.84c-.24 0-.46.15-.49.38l-.38 2.5c-.6.24-1.13.57-1.62.94l-2.39-.96c-.22-.09-.47 0-.59.22L2.74 9.81c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.07.62-.07.94s.02.64.07.94l-2.03 1.58a.49.49 0 0 0-.12.61l1.92 3.32c.12.22.37.31.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.38 2.5c.03.24.25.41.49.41h3.84c.24 0 .46-.15.49-.38l.38-2.5c.6-.24 1.13-.57 1.62-.94l2.39.96c.22.09.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.99 0-3.6-1.61-3.6-3.6s1.61-3.6 3.6-3.6 3.6 1.61 3.6 3.6-1.61 3.6-3.6 3.6z',
  };
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" width="18" height="18" className="nav-icon">
      <path d={paths[name] || paths.compass} />
    </svg>
  );
};

const pageMapping: Record<string, string> = {
  overview: 'overview',
  notifications: 'notifications',
  'pull-requests': 'pull-requests',
  issues: 'issues',
  actions: 'actions',
  repositories: 'repositories',
};

export default function Sidebar({ activePage, onNavigate, settings, onOpenSettings, onUpdateSettings, addToast }: SidebarProps) {
  const [showBookmarks, setShowBookmarks] = React.useState(true);
  const [editingBookmark, setEditingBookmark] = React.useState(false);
  const [newBmTitle, setNewBmTitle] = React.useState('');
  const [newBmUrl, setNewBmUrl] = React.useState('');

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
    } catch { addToast('Failed to add bookmark', 'error'); }
  };

  const handleRemoveBookmark = async (id: string) => {
    try {
      await API.RemoveBookmark(id);
      const str = await API.GetSettings();
      onUpdateSettings(JSON.parse(str));
    } catch { addToast('Failed to remove bookmark', 'error'); }
  };

  const activeAccount: GitHubAccount | undefined = settings.accounts[settings.activeAccount];

  return (
    <div className="sidebar">
      <div className="sidebar-header">
        <div className="logo">OG</div>
        <h1>Oh My GitHub</h1>
      </div>
      <div style={{ padding: '8px', borderBottom: '1px solid var(--border-muted)' }}>
        {activeAccount ? (
          <div className="account-selector" onClick={onOpenSettings}>
            <div className="account-avatar">
              {activeAccount.avatarUrl ? <img src={activeAccount.avatarUrl} alt="" /> : null}
            </div>
            <span className="account-name">{activeAccount.username}</span>
            <Icon name="gear" />
          </div>
        ) : (
          <button className="btn btn-primary btn-sm" onClick={onOpenSettings} style={{ width: '100%', justifyContent: 'center' }}>
            Sign in with GitHub
          </button>
        )}
      </div>
      <div className="sidebar-nav">
        {navItems.map((item) => (
          <div key={item.id} className={`nav-item ${activePage === pageMapping[item.id] ? 'active' : ''}`}
            onClick={() => onNavigate(item.id)}>
            <Icon name={item.icon} />
            {item.label}
          </div>
        ))}
        <div style={{ marginTop: 24 }}>
          <div className="sidebar-section-title" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <span onClick={() => setShowBookmarks(!showBookmarks)} style={{ cursor: 'pointer', flex: 1 }}>Bookmarks</span>
            <span onClick={() => setEditingBookmark(!editingBookmark)} style={{ cursor: 'pointer', color: 'var(--text-link)' }}>
              <Icon name="add" />
            </span>
          </div>
          {editingBookmark && (
            <div style={{ padding: '4px 12px', marginBottom: 8 }}>
              <input className="input" placeholder="Title" value={newBmTitle}
                onChange={e => setNewBmTitle(e.target.value)} style={{ marginBottom: 4, fontSize: 12 }} />
              <input className="input" placeholder="URL or repo" value={newBmUrl}
                onChange={e => setNewBmUrl(e.target.value)} style={{ marginBottom: 4, fontSize: 12 }} />
              <button className="btn btn-success btn-sm" onClick={handleAddBookmark} style={{ width: '100%' }}>Add</button>
            </div>
          )}
          {showBookmarks && settings.bookmarks.map((bm) => (
            <div key={bm.id} className="bookmark-item">
              <Icon name="bookmark" />
              <span>{bm.title}</span>
              <span className="remove-btn" onClick={() => handleRemoveBookmark(bm.id)}>
                <Icon name="close" />
              </span>
            </div>
          ))}
          {showBookmarks && settings.bookmarks.length === 0 && (
            <div style={{ padding: '8px 12px', fontSize: 12, color: 'var(--text-tertiary)' }}>
              No bookmarks yet. Click + to add.
            </div>
          )}
        </div>
      </div>
      <div className="sidebar-footer">
        <button className="btn btn-ghost btn-sm" onClick={onOpenSettings} style={{ width: '100%', justifyContent: 'center' }}>
          <Icon name="gear" />
          Settings
        </button>
      </div>
    </div>
  );
}
