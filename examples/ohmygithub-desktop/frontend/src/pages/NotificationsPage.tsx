import React, { useState, useEffect, useCallback } from 'react';
import { API, Notification, formatRelativeTime } from '../lib/api';

interface NotificationsPageProps {
  onSelect: (item: any) => void;
  addToast: (message: string, type?: string) => void;
}

export default function NotificationsPage({ onSelect, addToast }: NotificationsPageProps) {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<'all' | 'unread'>('unread');

  const loadNotifications = useCallback(async () => {
    setLoading(true);
    try {
      const str = await API.GetNotifications();
      if (!str) {
        setNotifications([]);
        return;
      }
      const data = JSON.parse(str);
      setNotifications(Array.isArray(data) ? data : []);
    } catch (e: any) {
      // Wails 返回的 error 可能不是标准 Error，提取多种可能的字段
      const msg = e?.message || e?.error || (typeof e === 'string' ? e : 'unknown error');
      console.error('GetNotifications failed:', e);
      // 针对常见错误给出可操作提示
      if (msg.includes('403') || msg.includes('Resource not accessible')) {
        addToast('Token 缺少 notifications 权限。请到 Settings 删除账号后重新添加，创建 token 时勾选 "notifications" scope', 'error');
      } else if (msg.includes('401') || msg.toLowerCase().includes('bad credentials')) {
        addToast('Token 无效或已过期，请到 Settings 重新添加 token', 'error');
      } else if (msg.includes('no GitHub account')) {
        addToast('请先在 Settings 添加 GitHub token', 'error');
      } else {
        addToast('Failed to load notifications: ' + msg, 'error');
      }
      setNotifications([]);
    } finally {
      setLoading(false);
    }
  }, [addToast]);

  useEffect(() => {
    loadNotifications();
    const interval = setInterval(loadNotifications, 60000);
    return () => clearInterval(interval);
  }, [loadNotifications]);

  const handleMarkAllRead = async () => {
    try {
      await API.MarkAllNotificationsRead();
      addToast('All notifications marked as read', 'success');
      loadNotifications();
    } catch { addToast('Failed to mark all as read', 'error'); }
  };

  const handleMarkRead = async (id: string) => {
    try {
      await API.MarkNotificationRead(id);
      setNotifications(prev => prev.map(n => n.id === id ? { ...n, read: true } : n));
    } catch { /* ignore */ }
  };

  const handleClick = (notif: Notification) => {
    onSelect({ ...notif, _type: 'notification' });
    if (!notif.read) handleMarkRead(notif.id);
  };

  const filtered = filter === 'unread' ? notifications.filter(n => !n.read) : notifications;

  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'issue': return '—';
      case 'pr': case 'pull_request': return '⧩';
      case 'release': return '⌂';
      case 'discussion': return '💬';
      default: return '•';
    }
  };

  return (
    <div className="fade-in">
      <div className="filter-bar">
        <span className="filter-label">Filter:</span>
        <button className={`btn btn-sm ${filter === 'unread' ? 'btn-primary' : 'btn-ghost'}`} onClick={() => setFilter('unread')}>Unread ({notifications.filter(n => !n.read).length})</button>
        <button className={`btn btn-sm ${filter === 'all' ? 'btn-primary' : 'btn-ghost'}`} onClick={() => setFilter('all')}>All</button>
        <div style={{ flex: 1 }} />
        <button className="btn btn-sm" onClick={handleMarkAllRead}>Mark all as read</button>
        <button className="btn btn-ghost btn-sm" onClick={loadNotifications}>Refresh</button>
      </div>

      {loading ? (
        <div className="loading-spinner"><div className="spinner" /></div>
      ) : filtered.length === 0 ? (
        <div className="empty-state">
          <svg viewBox="0 0 24 24" fill="currentColor" width="48" height="48" className="icon">
            <path d="M12 22c1.1 0 2-.9 2-2h-4c0 1.1.89 2 2 2zm6-6v-5c0-3.07-1.64-5.64-4.5-6.32V4c0-.83-.67-1.5-1.5-1.5s-1.5.67-1.5 1.5v.68C7.63 5.36 6 7.92 6 11v5l-2 2v1h16v-1l-2-2z"/>
          </svg>
          <h3>All caught up!</h3>
          <p>No notifications to show.</p>
        </div>
      ) : (
        <div style={{ border: '1px solid var(--border-muted)', borderRadius: 'var(--radius-md)', overflow: 'hidden' }}>
          {filtered.map((notif, i) => (
            <div key={notif.id} className={`list-item ${i === 0 ? '' : ''}`} onClick={() => handleClick(notif)}
              style={{ background: notif.read ? undefined : 'rgba(31,111,235,0.05)', borderLeft: notif.read ? undefined : '3px solid var(--text-link)' }}>
              <div className="list-item-avatar" style={{ width: 24, height: 24, fontSize: 12, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                {getTypeIcon(notif.type)}
              </div>
              <div className="list-item-content">
                <div className="list-item-title">{notif.title}</div>
                <div className="list-item-meta">
                  <span className="repo-name">{notif.repo}</span>
                  <span>{notif.type}</span>
                  <span>{formatRelativeTime(notif.updatedAt)}</span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
