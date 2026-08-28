import React, { useState, useEffect, useCallback } from 'react';
import { API, Notification, formatRelativeTime } from '../lib/api';
import { Button } from '../components/ui/button';
import { ScrollArea } from '../components/ui/scroll-area';
import { Bell, RefreshCw, CheckCheck } from 'lucide-react';
import { cn } from '@/lib/utils';

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
      const msg = e?.message || e?.error || (typeof e === 'string' ? e : 'unknown error');
      console.error('GetNotifications failed:', e);
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
    } catch {
      addToast('Failed to mark all as read', 'error');
    }
  };

  const handleMarkRead = async (id: string) => {
    try {
      await API.MarkNotificationRead(id);
      setNotifications((prev) => prev.map((n) => (n.id === id ? { ...n, read: true } : n)));
    } catch {
      /* ignore */
    }
  };

  const handleClick = (notif: Notification) => {
    onSelect({ ...notif, _type: 'notification' });
    if (!notif.read) handleMarkRead(notif.id);
  };

  const filtered = filter === 'unread' ? notifications.filter((n) => !n.read) : notifications;

  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'issue':
        return '—';
      case 'pr':
      case 'pull_request':
        return '⧩';
      case 'release':
        return '⌂';
      case 'discussion':
        return '💬';
      default:
        return '•';
    }
  };

  return (
    <div className="animate-fade-in">
      {/* Filter Bar */}
      <div className="mb-4 flex flex-wrap items-center gap-2 rounded-md border border-border bg-background p-2">
        <span className="text-xs font-medium text-muted-foreground">Filter:</span>
        <Button
          variant={filter === 'unread' ? 'default' : 'ghost'}
          size="sm"
          onClick={() => setFilter('unread')}
        >
          Unread ({notifications.filter((n) => !n.read).length})
        </Button>
        <Button
          variant={filter === 'all' ? 'default' : 'ghost'}
          size="sm"
          onClick={() => setFilter('all')}
        >
          All
        </Button>
        <div className="flex-1" />
        <Button variant="outline" size="sm" onClick={handleMarkAllRead}>
          <CheckCheck className="h-3.5 w-3.5" />
          Mark all as read
        </Button>
        <Button variant="ghost" size="sm" onClick={loadNotifications}>
          <RefreshCw className="h-3.5 w-3.5" />
          Refresh
        </Button>
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-border border-t-primary" />
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
          <Bell className="mb-3 h-12 w-12 opacity-40" />
          <h3 className="mb-1 text-base font-semibold text-secondary-foreground">All caught up!</h3>
          <p className="text-sm">No notifications to show.</p>
        </div>
      ) : (
        <div className="overflow-hidden rounded-md border border-border">
          {filtered.map((notif) => (
            <div
              key={notif.id}
              onClick={() => handleClick(notif)}
              className={cn(
                'flex cursor-pointer gap-3 border-b border-border px-4 py-3 transition-colors hover:bg-muted/50 last:border-b-0',
                !notif.read && 'bg-primary/5 border-l-[3px] border-l-primary'
              )}
            >
              <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-muted text-xs">
                {getTypeIcon(notif.type)}
              </div>
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium truncate">{notif.title}</div>
                <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                  <span className="font-medium text-primary">{notif.repo}</span>
                  <span>·</span>
                  <span>{notif.type}</span>
                  <span>·</span>
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
