import React, { useState, useEffect, useCallback } from 'react';
import { API, AppSettings, formatRelativeTime, SyncStateEntry } from '../lib/api';

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

  // 每 30 秒刷新同步状态 + 倒计时
  useEffect(() => {
    loadSyncState();
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
      // 静默失败
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
      const repos = Array.isArray(repoResp) ? repoResp : (repoResp.data || []);

      setStats({
        notifications: notifs.length,
        openPRs: prs.length,
        openIssues: issues.length,
        repos: repos.length,
      });

      // Combine recent activity
      const activity = [
        ...prs.slice(0, 5).map((p: any) => ({ ...p, _type: 'pr', _time: p.updatedAt })),
        ...issues.slice(0, 5).map((i: any) => ({ ...i, _type: 'issue', _time: i.updatedAt })),
        ...notifs.slice(0, 3).map((n: any) => ({ ...n, _type: 'notification', _time: n.updatedAt })),
      ].sort((a, b) => new Date(b._time).getTime() - new Date(a._time).getTime()).slice(0, 8);

      setRecentActivity(activity);
    } catch (e) {
      console.error('Failed to load stats', e);
    } finally {
      setLoading(false);
    }
  };

  if (loading) return <div className="loading-spinner"><div className="spinner" /></div>;

  return (
    <div className="fade-in">
      {settings.accounts.length === 0 ? (
        <div className="empty-state" style={{ marginTop: 48 }}>
          <svg viewBox="0 0 24 24" fill="currentColor" width="64" height="64" className="icon">
            <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 5-5v3h4v4h-4v3zm7-3l-5 5v-3h-4v-4h4v-3l5 5z"/>
          </svg>
          <h3>Welcome to Oh My GitHub</h3>
          <p>Get started by adding your GitHub account in Settings. You'll need a Personal Access Token with repo, notifications, and workflow scopes.</p>
          <button className="btn btn-primary" onClick={() => onNavigate('notifications')} style={{ marginTop: 8 }}>
            Browse without an account
          </button>
        </div>
      ) : (
        <>
          <div className="dashboard-grid" style={{ marginBottom: 24 }}>
            <div className="dashboard-card">
              <div className="dashboard-card-title">Notifications</div>
              <div className="dashboard-stat">
                <span className="stat-value">{stats.notifications}</span>
                <span className="stat-label">Unread</span>
              </div>
              <button className="btn btn-sm" onClick={() => onNavigate('notifications')} style={{ marginTop: 8 }}>
                View all
              </button>
            </div>
            <div className="dashboard-card">
              <div className="dashboard-card-title">Pull Requests</div>
              <div className="dashboard-stat">
                <span className="stat-value">{stats.openPRs}</span>
                <span className="stat-label">Open</span>
              </div>
              <button className="btn btn-sm" onClick={() => onNavigate('pull-requests')} style={{ marginTop: 8 }}>
                View all
              </button>
            </div>
            <div className="dashboard-card">
              <div className="dashboard-card-title">Issues</div>
              <div className="dashboard-stat">
                <span className="stat-value">{stats.openIssues}</span>
                <span className="stat-label">Open</span>
              </div>
              <button className="btn btn-sm" onClick={() => onNavigate('issues')} style={{ marginTop: 8 }}>
                View all
              </button>
            </div>
            <div className="dashboard-card">
              <div className="dashboard-card-title">Repositories</div>
              <div className="dashboard-stat">
                <span className="stat-value">{stats.repos}</span>
                <span className="stat-label">Total</span>
              </div>
              <button className="btn btn-sm" onClick={() => onNavigate('repositories')} style={{ marginTop: 8 }}>
                View all
              </button>
            </div>
          </div>

          {/* 同步状态卡片 */}
          {syncStates.length > 0 && (
            <div className="dashboard-card" style={{ marginBottom: 24 }}>
              <div className="dashboard-card-title" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <span>Sync Status</span>
                <button className="btn btn-ghost btn-sm" onClick={loadSyncState}>Refresh</button>
              </div>
              <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', marginTop: 8 }}>
                {syncStates.map(s => {
                  const isSyncing = s.syncing;
                  const lastSyncDate = s.lastSync > 0 ? new Date(s.lastSync * 1000) : null;
                  const lastFullDate = s.lastFullSync > 0 ? new Date(s.lastFullSync * 1000) : null;
                  const nextIn = s.nextSyncIn;
                  const kindLabel = s.kind === 'mine' ? 'My Repos' : s.kind === 'starred' ? 'Starred' : s.kind;

                  return (
                    <div key={s.kind} style={{
                      flex: '1 1 200px', minWidth: 200,
                      padding: 12, borderRadius: 8,
                      border: '1px solid var(--border-muted)',
                      background: 'var(--bg-secondary, var(--bg-elevated))',
                    }}>
                      {/* 标题行 */}
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                        <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>{kindLabel}</span>
                        <span style={{ fontSize: 11, color: 'var(--text-tertiary)' }}>· {s.totalCount} repos</span>
                        {isSyncing && (
                          <span style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 11, color: 'var(--accent)' }}>
                            <span className="spinner" style={{ width: 10, height: 10, borderWidth: 1.5 }} />
                            Syncing
                          </span>
                        )}
                      </div>

                      {/* 进度条 */}
                      <div style={{
                        height: 4, borderRadius: 2, overflow: 'hidden',
                        background: 'var(--border-muted)', marginBottom: 8,
                      }}>
                        <div style={{
                          height: '100%', borderRadius: 2,
                          background: isSyncing ? 'var(--accent)' : (s.needsSync || s.needsFull ? 'var(--warn, #f0ad4e)' : 'var(--success, #28a745)'),
                          transition: 'width 0.5s ease',
                          width: isSyncing ? '60%' : (s.needsSync || s.needsFull ? '100%' : nextIn > 0 ? '100%' : '100%'),
                        }} />
                      </div>

                      {/* 状态文字 */}
                      <div style={{ fontSize: 11, color: 'var(--text-secondary)', display: 'flex', flexDirection: 'column', gap: 2 }}>
                        {isSyncing ? (
                          <span>正在同步...</span>
                        ) : s.needsSync || s.needsFull ? (
                          <span style={{ color: 'var(--warn, #f0ad4e)' }}>
                            {s.needsFull ? '需要全量校正' : '需要增量同步'}
                          </span>
                        ) : lastSyncDate ? (
                          <span>最后同步: {formatRelativeTime(lastSyncDate.toISOString())}</span>
                        ) : (
                          <span>尚未同步</span>
                        )}
                        {nextIn > 0 && !isSyncing && !s.needsSync && !s.needsFull && (
                          <span style={{ color: 'var(--text-tertiary)' }}>
                            下次同步: {Math.floor(nextIn / 60)}m {nextIn % 60}s 后
                          </span>
                        )}
                        {lastFullDate && (
                          <span style={{ color: 'var(--text-tertiary)' }}>
                            全量校正: {formatRelativeTime(lastFullDate.toISOString())}
                          </span>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          <div className="dashboard-card">
            <div className="dashboard-card-title" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span>Recent Activity</span>
              <button className="btn btn-ghost btn-sm" onClick={loadStats}>Refresh</button>
            </div>
            {recentActivity.length === 0 ? (
              <div className="empty-state" style={{ padding: 24 }}>
                <p>No recent activity found</p>
              </div>
            ) : (
              recentActivity.map((item, i) => (
                <div key={i} className="dashboard-stat">
                  <span className={`state-icon ${item._type === 'pr' ? (item.draft ? 'draft' : 'open') : item.state === 'open' ? 'open' : 'closed'}`}>
                    <svg viewBox="0 0 16 16" fill="currentColor" width="16" height="16">
                      {item._type === 'pr' ? (
                        <path d="M7.177 3.073L9.573.677A.25.25 0 0110 .854v4.792a.25.25 0 01-.427.177L7.177 3.427a.25.25 0 010-.354zM3.75 2.5a.75.75 0 100 1.5.75.75 0 000-1.5z"/>
                      ) : item._type === 'issue' ? (
                        <path d="M8 9.5a1.5 1.5 0 100-3 1.5 1.5 0 000 3zM8 0a8 8 0 110 16A8 8 0 018 0z"/>
                      ) : (
                        <path d="M12 22c1.1 0 2-.9 2-2h-4c0 1.1.89 2 2 2zm6-6v-5c0-3.07-1.64-5.64-4.5-6.32V4c0-.83-.67-1.5-1.5-1.5s-1.5.67-1.5 1.5v.68C7.63 5.36 6 7.92 6 11v5l-2 2v1h16v-1l-2-2z"/>
                      )}
                    </svg>
                  </span>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: 13, color: 'var(--text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {item.title}
                    </div>
                    <div style={{ fontSize: 11, color: 'var(--text-tertiary)' }}>
                      {item.repo} · {formatRelativeTime(item._time)}
                    </div>
                  </div>
                </div>
              ))
            )}
          </div>
        </>
      )}
    </div>
  );
}
