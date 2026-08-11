import React, { useState, useEffect, useCallback } from 'react';
import { API, Issue, formatRelativeTime } from '../lib/api';

interface IssuesPageProps {
  onSelect: (item: any) => void;
  addToast: (message: string, type?: string) => void;
}

export default function IssuesPage({ onSelect, addToast }: IssuesPageProps) {
  const [issues, setIssues] = useState<Issue[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<'open' | 'closed' | 'all'>('open');
  const [sort, setSort] = useState<'updated' | 'created'>('updated');

  const loadIssues = useCallback(async () => {
    setLoading(true);
    try {
      const str = await API.GetIssues(filter, sort);
      setIssues(JSON.parse(str));
    } catch (e) {
      addToast('Failed to load issues', 'error');
      setIssues([]);
    } finally {
      setLoading(false);
    }
  }, [filter, sort, addToast]);

  useEffect(() => { loadIssues(); }, [loadIssues]);

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
        <button className="btn btn-ghost btn-sm" onClick={loadIssues}>Refresh</button>
      </div>

      {loading ? (
        <div className="loading-spinner"><div className="spinner" /></div>
      ) : issues.length === 0 ? (
        <div className="empty-state">
          <svg viewBox="0 0 24 24" fill="currentColor" width="48" height="48" className="icon">
            <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 15c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5z"/>
          </svg>
          <h3>No issues found</h3>
          <p>No {filter} issues match your criteria.</p>
        </div>
      ) : (
        <div style={{ border: '1px solid var(--border-muted)', borderRadius: 'var(--radius-md)', overflow: 'hidden' }}>
          {issues.map((issue) => (
            <div key={issue.id} className="list-item" onClick={() => onSelect({ ...issue, _type: 'issue' })}>
              <div className={`state-icon ${issue.state === 'open' ? 'open' : 'closed'}`}>
                <svg viewBox="0 0 16 16" fill="currentColor" width="16" height="16">
                  <path d="M8 9.5a1.5 1.5 0 100-3 1.5 1.5 0 000 3zM8 0a8 8 0 110 16A8 8 0 018 0z"/>
                </svg>
              </div>
              <div className="list-item-avatar">
                <img src={issue.avatarUrl} alt="" />
              </div>
              <div className="list-item-content">
                <div className="list-item-title">{issue.title}</div>
                <div className="list-item-meta">
                  <span>#{issue.number}</span>
                  <span>by {issue.user}</span>
                  <span className="repo-name">{issue.repo}</span>
                  <span>{formatRelativeTime(issue.createdAt)}</span>
                  {issue.comments > 0 && <span>💬 {issue.comments}</span>}
                </div>
              </div>
              {issue.labels.length > 0 && (
                <div style={{ display: 'flex', gap: 4, alignItems: 'flex-start', flexWrap: 'wrap', maxWidth: 200 }}>
                  {issue.labels.slice(0, 3).map((l, i) => (
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
