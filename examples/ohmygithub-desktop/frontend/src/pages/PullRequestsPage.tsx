import React, { useState, useEffect, useCallback } from 'react';
import { API, PullRequest, formatRelativeTime } from '../lib/api';

interface PullRequestsPageProps {
  onSelect: (item: any) => void;
  addToast: (message: string, type?: string) => void;
  activeRepo?: string;
}

export default function PullRequestsPage({ onSelect, addToast, activeRepo }: PullRequestsPageProps) {
  const [prs, setPrs] = useState<PullRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<'open' | 'closed' | 'all'>('open');
  const [sort, setSort] = useState<'updated' | 'created'>('updated');

  const loadPRs = useCallback(async () => {
    setLoading(true);
    try {
      const str = await API.GetPullRequests(filter, sort, activeRepo || '');
      setPrs(JSON.parse(str));
    } catch (e) {
      addToast('Failed to load pull requests', 'error');
      setPrs([]);
    } finally {
      setLoading(false);
    }
  }, [filter, sort, activeRepo, addToast]);

  useEffect(() => { loadPRs(); }, [loadPRs]);

  const handleClick = async (pr: PullRequest) => {
    try {
      const diffStr = await API.GetPRDiff(pr.repo, pr.number);
      onSelect({ ...pr, _type: 'pr', _diff: diffStr });
    } catch {
      onSelect({ ...pr, _type: 'pr' });
    }
  };

  return (
    <div className="fade-in">
      <div className="filter-bar">
        <span className="filter-label">State:</span>
        <button className={`btn btn-sm ${filter === 'open' ? 'btn-primary' : 'btn-ghost'}`} onClick={() => setFilter('open')}>Open</button>
        <button className={`btn btn-sm ${filter === 'closed' ? 'btn-primary' : 'btn-ghost'}`} onClick={() => setFilter('closed')}>Closed</button>
        <button className={`btn btn-sm ${filter === 'all' ? 'btn-primary' : 'btn-ghost'}`} onClick={() => setFilter('all')}>All</button>
        <div style={{ width: 1, height: 20, background: 'var(--border-muted)' }} />
        <span className="filter-label">Sort:</span>
        <select className="select" style={{ width: 'auto', fontSize: 12 }} value={sort} onChange={e => setSort(e.target.value as any)}>
          <option value="updated">Recently updated</option>
          <option value="created">Recently created</option>
        </select>
        <div style={{ flex: 1 }} />
        {activeRepo && (
          <span className="filter-label" style={{ color: 'var(--text-accent)' }}>
            Scope: {activeRepo}
          </span>
        )}
        <button className="btn btn-ghost btn-sm" onClick={loadPRs}>Refresh</button>
      </div>

      {loading ? (
        <div className="loading-spinner"><div className="spinner" /></div>
      ) : prs.length === 0 ? (
        <div className="empty-state">
          <svg viewBox="0 0 24 24" fill="currentColor" width="48" height="48" className="icon">
            <path d="M21 8c0-1.66-1.34-3-3-3s-3 1.34-3 3c0 1.3.84 2.4 2 2.82V15c0 .55-.45 1-1 1h-2.18c-.41-1.16-1.52-2-2.82-2s-2.4.84-2.82 2H8c-.55 0-1-.45-1-1v-4.18c1.16-.41 2-1.52 2-2.82C9 7.34 7.66 6 6 6S3 7.34 3 9c0 1.3.84 2.4 2 2.82V15c0 1.66 1.34 3 3 3h2.18c.41 1.16 1.52 2 2.82 2s2.4-.84 2.82-2H16c1.66 0 3-1.34 3-3v-4.18A2.99 2.99 0 0 0 21 8z"/>
          </svg>
          <h3>No pull requests found</h3>
          <p>No {filter} PRs match your criteria.</p>
        </div>
      ) : (
        <div style={{ border: '1px solid var(--border-muted)', borderRadius: 'var(--radius-md)', overflow: 'hidden' }}>
          {prs.map((pr) => (
            <div key={pr.id} className="list-item" onClick={() => handleClick(pr)}>
              <div className={`state-icon ${pr.draft ? 'draft' : pr.state === 'open' ? 'open' : 'closed'}`}>
                <svg viewBox="0 0 16 16" fill="currentColor" width="16" height="16">
                  <path d="M7.177 3.073L9.573.677A.25.25 0 0110 .854v4.792a.25.25 0 01-.427.177L7.177 3.427a.25.25 0 010-.354zM3.75 2.5a.75.75 0 100 1.5.75.75 0 000-1.5zM2.5 3.25a1.25 1.25 0 112.5 0 1.25 1.25 0 01-2.5 0zM3.75 12a.75.75 0 100 1.5.75.75 0 000-1.5zM2.5 12.75a1.25 1.25 0 112.5 0 1.25 1.25 0 01-2.5 0z"/>
                </svg>
              </div>
              <div className="list-item-avatar">
                <img src={pr.avatarUrl} alt="" />
              </div>
              <div className="list-item-content">
                <div className="list-item-title">{pr.title}</div>
                <div className="list-item-meta">
                  <span>#{pr.number}</span>
                  <span>by {pr.user}</span>
                  <span className="repo-name">{pr.repo}</span>
                  <span>{formatRelativeTime(pr.updatedAt)}</span>
                  {pr.draft && <span className="tag">Draft</span>}
                </div>
              </div>
              {pr.labels.length > 0 && (
                <div style={{ display: 'flex', gap: 4, alignItems: 'flex-start', flexWrap: 'wrap', maxWidth: 200 }}>
                  {pr.labels.slice(0, 3).map((l, i) => (
                    <span key={i} className="label" style={{
                      background: `#${l.color}22`,
                      borderColor: `#${l.color}44`,
                      color: `#${l.color}`,
                      fontSize: 10,
                    }}>{l.name}</span>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
