import React, { useState, useEffect, useCallback } from 'react';
import { API, AppSettings, formatRelativeTime, SyncStateEntry } from '../lib/api';
import { Button } from '../components/ui/button';
import {
  Bell,
  GitPullRequest,
  CircleDot,
  BookMarked,
  Compass,
  RefreshCw,
  Loader2,
} from 'lucide-react';
import { cn } from '@/lib/utils';

interface OverviewPageProps {
  settings: AppSettings;
  onNavigate: (page: string) => void;
  addToast: (message: string, type?: string) => void;
}

export default function OverviewPage({ settings, onNavigate, addToast }: OverviewPageProps) {
  const [stats, setStats] = useState({ notifications: 0, openPRs: 0, openIssues: 0, repos: 0 });
  const [recentActivity, setRecentActivity] = useState<any[]>([]);
  const [syncStates, setSyncStates] = useState<SyncStateEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [now, setNow] = useState(Date.now());

  useEffect(() => {
    loadSyncState();
    loadStats();
    const timer = setInterval(() => {
      setNow(Date.now());
      loadSyncState();
    }, 30000);
    return () => clearInterval(timer);
  }, []);

  const loadSyncState = useCallback(async () => {
    try {
      const str = await API.GetSyncState();
      const entries: SyncStateEntry[] = JSON.parse(str);
      setSyncStates(entries);
    } catch {
      // silent
    }
  }, []);

  const loadStats = async () => {
    setLoading(true);
    try {
      const [notifStr, prStr, issueStr, repoStr] = await Promise.all([
        API.GetNotifications().catch(() => '[]'),
        API.GetPullRequests('open', 'updated', '').catch(() => '[]'),
        API.GetIssues('open', 'updated', '').catch(() => '[]'),
        API.GetMyRepos('updated').catch(() => '{"data":[]}'),
      ]);
      const notifs = JSON.parse(notifStr);
      const prs = JSON.parse(prStr);
      const issues = JSON.parse(issueStr);
      const repoResp = JSON.parse(repoStr);
      const repos = Array.isArray(repoResp) ? repoResp : repoResp.data || [];

      setStats({
        notifications: notifs.length,
        openPRs: prs.length,
        openIssues: issues.length,
        repos: repos.length,
      });

      const activity = [
        ...prs.slice(0, 5).map((p: any) => ({ ...p, _type: 'pr', _time: p.updatedAt })),
        ...issues.slice(0, 5).map((i: any) => ({ ...i, _type: 'issue', _time: i.updatedAt })),
        ...notifs.slice(0, 3).map((n: any) => ({ ...n, _type: 'notification', _time: n.updatedAt })),
      ]
        .sort((a, b) => new Date(b._time).getTime() - new Date(a._time).getTime())
        .slice(0, 8);

      setRecentActivity(activity);
    } catch (e) {
      console.error('Failed to load stats', e);
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-6 w-6 animate-spin text-primary" />
      </div>
    );
  }

  if (settings.accounts.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center text-muted-foreground">
        <Compass className="mb-4 h-16 w-16 opacity-40" />
        <h3 className="mb-2 text-xl font-semibold text-secondary-foreground">
          Welcome to Oh My GitHub
        </h3>
        <p className="mb-4 max-w-md text-sm">
          Get started by adding your GitHub account in Settings. You'll need a Personal Access
          Token with repo, notifications, and workflow scopes.
        </p>
        <Button onClick={() => onNavigate('notifications')} className="mt-2">
          Browse without an account
        </Button>
      </div>
    );
  }

  const cards = [
    {
      title: 'Notifications',
      value: stats.notifications,
      label: 'Unread',
      page: 'notifications',
      icon: Bell,
    },
    {
      title: 'Pull Requests',
      value: stats.openPRs,
      label: 'Open',
      page: 'pull-requests',
      icon: GitPullRequest,
    },
    {
      title: 'Issues',
      value: stats.openIssues,
      label: 'Open',
      page: 'issues',
      icon: CircleDot,
    },
    {
      title: 'Repositories',
      value: stats.repos,
      label: 'Total',
      page: 'repositories',
      icon: BookMarked,
    },
  ];

  return (
    <div className="animate-fade-in space-y-6">
      {/* Stats Grid */}
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {cards.map((card) => {
          const Icon = card.icon;
          return (
            <div
              key={card.page}
              className="rounded-md border border-border bg-card p-4"
            >
              <div className="mb-3 flex items-center justify-between">
                <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  {card.title}
                </span>
                <Icon className="h-4 w-4 text-muted-foreground" />
              </div>
              <div className="mb-3 flex items-baseline gap-2">
                <span className="text-2xl font-bold">{card.value}</span>
                <span className="text-xs text-muted-foreground">{card.label}</span>
              </div>
              <Button
                variant="ghost"
                size="sm"
                className="w-full justify-start"
                onClick={() => onNavigate(card.page)}
              >
                View all →
              </Button>
            </div>
          );
        })}
      </div>

      {/* Sync Status */}
      {syncStates.length > 0 && (
        <div className="rounded-md border border-border bg-card p-4">
          <div className="mb-3 flex items-center justify-between">
            <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Sync Status
            </span>
            <Button variant="ghost" size="sm" onClick={loadSyncState}>
              <RefreshCw className="h-3.5 w-3.5" />
              Refresh
            </Button>
          </div>
          <div className="flex flex-wrap gap-4">
            {syncStates.map((s) => {
              const isSyncing = s.syncing;
              const lastSyncDate = s.lastSync > 0 ? new Date(s.lastSync * 1000) : null;
              const lastFullDate = s.lastFullSync > 0 ? new Date(s.lastFullSync * 1000) : null;
              const nextIn = s.nextSyncIn;
              const kindLabel = s.kind === 'mine' ? 'My Repos' : s.kind === 'starred' ? 'Starred' : s.kind;

              return (
                <div
                  key={s.kind}
                  className="min-w-[200px] flex-1 rounded-md border border-border bg-muted/50 p-3"
                >
                  <div className="mb-2 flex items-center gap-2">
                    <span className="text-sm font-semibold">{kindLabel}</span>
                    <span className="text-xs text-muted-foreground">· {s.totalCount} repos</span>
                    {isSyncing && (
                      <span className="flex items-center gap-1 text-xs text-primary">
                        <Loader2 className="h-2.5 w-2.5 animate-spin" />
                        Syncing
                      </span>
                    )}
                  </div>
                  <div className="mb-2 h-1 overflow-hidden rounded-full bg-border">
                    <div
                      className={cn(
                        'h-full rounded-full transition-all duration-500',
                        isSyncing
                          ? 'bg-primary w-3/5'
                          : s.needsSync || s.needsFull
                            ? 'bg-warning w-full'
                            : 'bg-success w-full'
                      )}
                    />
                  </div>
                  <div className="space-y-0.5 text-xs text-muted-foreground">
                    {isSyncing ? (
                      <span>正在同步...</span>
                    ) : s.needsSync || s.needsFull ? (
                      <span className="text-warning">
                        {s.needsFull ? '需要全量校正' : '需要增量同步'}
                      </span>
                    ) : lastSyncDate ? (
                      <span>最后同步: {formatRelativeTime(lastSyncDate.toISOString())}</span>
                    ) : (
                      <span>尚未同步</span>
                    )}
                    {nextIn > 0 && !isSyncing && !s.needsSync && !s.needsFull && (
                      <div className="text-muted-foreground">
                        下次同步: {Math.floor(nextIn / 60)}m {nextIn % 60}s 后
                      </div>
                    )}
                    {lastFullDate && (
                      <div className="text-muted-foreground">
                        全量校正: {formatRelativeTime(lastFullDate.toISOString())}
                      </div>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Recent Activity */}
      <div className="rounded-md border border-border bg-card p-4">
        <div className="mb-3 flex items-center justify-between">
          <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Recent Activity
          </span>
          <Button variant="ghost" size="sm" onClick={loadStats}>
            <RefreshCw className="h-3.5 w-3.5" />
            Refresh
          </Button>
        </div>
        {recentActivity.length === 0 ? (
          <div className="py-8 text-center text-sm text-muted-foreground">
            No recent activity found
          </div>
        ) : (
          <div className="space-y-0">
            {recentActivity.map((item, i) => {
              const Icon =
                item._type === 'pr'
                  ? GitPullRequest
                  : item._type === 'issue'
                    ? CircleDot
                    : Bell;
              return (
                <div
                  key={i}
                  className="flex items-center gap-3 border-b border-border py-2.5 last:border-b-0"
                >
                  <Icon
                    className={cn(
                      'h-4 w-4 shrink-0',
                      item._type === 'pr'
                        ? item.draft
                          ? 'text-muted-foreground'
                          : 'text-success'
                        : item.state === 'open'
                          ? 'text-success'
                          : 'text-destructive'
                    )}
                  />
                  <div className="flex-1 min-w-0">
                    <div className="truncate text-sm">{item.title}</div>
                    <div className="text-xs text-muted-foreground">
                      {item.repo} · {formatRelativeTime(item._time)}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
