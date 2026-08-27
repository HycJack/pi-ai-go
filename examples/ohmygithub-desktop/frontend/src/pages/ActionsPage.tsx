import React, { useState, useCallback, useEffect, useRef } from 'react';
import { API, WorkflowRun, Job, Repo, CachedRepoResponse, formatRelativeTime } from '../lib/api';

interface ActionsPageProps {
  addToast: (message: string, type?: string) => void;
  initialRepo?: string;
}

export default function ActionsPage({ addToast, initialRepo }: ActionsPageProps) {
  const [repoInput, setRepoInput] = useState(initialRepo || '');
  const [runs, setRuns] = useState<WorkflowRun[]>([]);
  const [selectedRun, setSelectedRun] = useState<WorkflowRun | null>(null);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [logs, setLogs] = useState('');
  const [loading, setLoading] = useState(false);
  const [loadingJobs, setLoadingJobs] = useState(false);

  const [myRepos, setMyRepos] = useState<Repo[]>([]);
  const [repoDropdownOpen, setRepoDropdownOpen] = useState(false);
  const [repoSearch, setRepoSearch] = useState('');
  const [reposLoading, setReposLoading] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setReposLoading(true);
      try {
        const str = await API.GetMyRepos('updated');
        if (cancelled) return;
        const resp: CachedRepoResponse<Repo> = JSON.parse(str);
        setMyRepos(resp.data || []);
      } catch {
        if (!cancelled) setMyRepos([]);
      } finally {
        if (!cancelled) setReposLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    if (!repoDropdownOpen) return;
    const handler = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setRepoDropdownOpen(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [repoDropdownOpen]);

  const loadRuns = useCallback(async () => {
    if (!repoInput.trim()) return;
    setLoading(true);
    try {
      const str = await API.GetWorkflowRuns(repoInput.trim());
      setRuns(JSON.parse(str));
    } catch (e) {
      addToast('Failed to load workflow runs', 'error');
    } finally {
      setLoading(false);
    }
  }, [repoInput, addToast]);

  const handleSelectRun = async (run: WorkflowRun) => {
    setSelectedRun(run);
    setLoadingJobs(true);
    setLogs('');
    try {
      const str = await API.GetWorkflowRunJobs(repoInput.trim(), run.id);
      setJobs(JSON.parse(str));
    } catch {
      setJobs([]);
    } finally {
      setLoadingJobs(false);
    }
  };

  const handleViewLogs = async (jobId: number) => {
    try {
      const str = await API.GetWorkflowLogs(repoInput.trim(), jobId);
      setLogs(str || '[No logs available]');
    } catch {
      setLogs('[Failed to load logs]');
    }
  };

  const handlePickRepo = (fullName: string) => {
    setRepoInput(fullName);
    setRepoDropdownOpen(false);
    setRepoSearch('');
  };

  const filteredRepos = repoSearch
    ? myRepos.filter(r =>
        r.fullName.toLowerCase().includes(repoSearch.toLowerCase()) ||
        (r.description && r.description.toLowerCase().includes(repoSearch.toLowerCase()))
      )
    : myRepos.slice(0, 100);

  const getStatusBadge = (status: string, conclusion: string) => {
    if (status === 'completed') {
      const cls = conclusion === 'success' ? 'success' : conclusion === 'failure' ? 'failure' : 'neutral';
      return <span className={`status-badge ${cls}`}>{conclusion}</span>;
    }
    return <span className="status-badge in_progress">{status}</span>;
  };

  return (
    <div className="fade-in" style={{ display: 'flex', gap: 16, height: '100%' }}>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div className="filter-bar" style={{ flexWrap: 'wrap', alignItems: 'center' }}>
          <span className="filter-label">Repository:</span>

          <div ref={dropdownRef} style={{ position: 'relative' }}>
            <button
              className="btn btn-ghost btn-sm"
              onClick={() => setRepoDropdownOpen(v => !v)}
              title="Choose from your repos"
              style={{ display: 'flex', alignItems: 'center', gap: 6 }}
            >
              <svg viewBox="0 0 16 16" fill="currentColor" width="14" height="14">
                <path d="M2 2.5A2.5 2.5 0 0 1 4.5 0h8.75a.75.75 0 0 1 .75.75v12.5a.75.75 0 0 1-.75.75h-2.5a.75.75 0 0 1 0-1.5h1.75v-2h-8a1 1 0 0 0-.714 1.7.75.75 0 1 1-1.072 1.05A2.495 2.495 0 0 1 2 11.5Zm10.5-1h-8a1 1 0 0 0-1 1v6.708A2.486 2.486 0 0 1 4.5 9h8ZM5 12.25a.25.25 0 0 1 .25-.25h3.5a.25.25 0 0 1 .25.25v3.25a.25.25 0 0 1-.4.2l-1.45-1.087a.249.249 0 0 0-.3 0L5.4 15.7a.25.25 0 0 1-.4-.2Z"/>
              </svg>
              {reposLoading ? 'Loading...' : 'My Repos'}
              <svg viewBox="0 0 16 16" fill="currentColor" width="12" height="12" style={{ opacity: 0.6 }}>
                <path d="M12.78 6.22a.75.75 0 0 1 0 1.06l-4.25 4.25a.75.75 0 0 1-1.06 0L3.22 7.28a.751.751 0 0 1 .018-1.042.751.751 0 0 1 1.042-.018L8 9.94l3.72-3.72a.75.75 0 0 1 1.06 0Z"/>
              </svg>
            </button>

            {repoDropdownOpen && (
              <div style={{
                position: 'absolute', top: '100%', left: 0, marginTop: 4,
                background: 'var(--bg-elevated)', border: '1px solid var(--border-muted)',
                borderRadius: 6, boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
                minWidth: 320, maxWidth: 420, zIndex: 20, maxHeight: 400, display: 'flex', flexDirection: 'column',
              }}>
                <div style={{ padding: 8, borderBottom: '1px solid var(--border-muted)', position: 'sticky', top: 0, background: 'var(--bg-elevated)' }}>
                  <input
                    className="input"
                    style={{ width: '100%', fontSize: 12 }}
                    placeholder="Search your repositories..."
                    value={repoSearch}
                    onChange={e => setRepoSearch(e.target.value)}
                    autoFocus
                  />
                </div>
                <div style={{ overflow: 'auto', flex: 1 }}>
                  {reposLoading ? (
                    <div style={{ padding: 16, textAlign: 'center', color: 'var(--text-tertiary)', fontSize: 12 }}>Loading repos...</div>
                  ) : filteredRepos.length === 0 ? (
                    <div style={{ padding: 16, textAlign: 'center', color: 'var(--text-tertiary)', fontSize: 12 }}>
                      {repoSearch ? 'No matching repos' : 'No cached repos. Sync repositories first.'}
                    </div>
                  ) : (
                    filteredRepos.map(r => (
                      <div
                        key={r.fullName}
                        onClick={() => handlePickRepo(r.fullName)}
                        style={{
                          padding: '8px 12px', cursor: 'pointer',
                          borderBottom: '1px solid var(--border-subtle)',
                          fontSize: 12,
                          background: repoInput === r.fullName ? 'var(--bg-overlay)' : 'transparent',
                        }}
                        onMouseOver={e => (e.currentTarget.style.background = 'var(--bg-overlay)')}
                        onMouseOut={e => (e.currentTarget.style.background = repoInput === r.fullName ? 'var(--bg-overlay)' : 'transparent')}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ fontWeight: 600 }}>{r.fullName}</span>
                          {r.private && <span className="tag" style={{ padding: '0 6px', fontSize: 10 }}>Private</span>}
                          {r.language && <span className="tag" style={{ padding: '0 6px', fontSize: 10 }}>{r.language}</span>}
                        </div>
                        {r.description && (
                          <div style={{ color: 'var(--text-tertiary)', fontSize: 11, marginTop: 2, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                            {r.description}
                          </div>
                        )}
                      </div>
                    ))
                  )}
                </div>
              </div>
            )}
          </div>

          <input
            className="input"
            style={{ width: 280, fontSize: 13 }}
            value={repoInput}
            onChange={e => setRepoInput(e.target.value)}
            placeholder="owner/repo"
          />
          <button className="btn btn-primary btn-sm" onClick={loadRuns}>Load</button>
        </div>

        {loading ? (
          <div className="loading-spinner"><div className="spinner" /></div>
        ) : runs.length === 0 ? (
          <div className="empty-state">
            <svg viewBox="0 0 24 24" fill="currentColor" width="48" height="48" className="icon">
              <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 14.5v-9l6 4.5-6 4.5z"/>
            </svg>
            <h3>No workflow runs</h3>
            <p>Choose a repository from "My Repos" or enter owner/repo manually, then click Load.</p>
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8, padding: '8px 0' }}>
            {runs.map(run => (
              <div key={run.id} className="workflow-card" onClick={() => handleSelectRun(run)}
                style={{ cursor: 'pointer', borderColor: selectedRun?.id === run.id ? 'var(--border-accent)' : undefined }}>
                <div className="workflow-card-header">
                  <span className="workflow-name">{run.name}</span>
                  {getStatusBadge(run.status, run.conclusion)}
                </div>
                <div className="workflow-meta">
                  <span>{run.headBranch}</span>
                  <span>trigger: {run.event}</span>
                  <span>by {run.actor}</span>
                  <span>{formatRelativeTime(run.createdAt)}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {(selectedRun || jobs.length > 0) && (
        <div style={{ width: 400, minWidth: 400, borderLeft: '1px solid var(--border-muted)', padding: 16, overflow: 'auto' }}>
          {selectedRun && (
            <div style={{ marginBottom: 16 }}>
              <h3 style={{ fontSize: 14, fontWeight: 600, marginBottom: 4 }}>{selectedRun.name}</h3>
              <div className="workflow-meta" style={{ marginBottom: 8 }}>
                <span>{selectedRun.headBranch}</span>
                <span>{selectedRun.event}</span>
              </div>
            </div>
          )}

          {loadingJobs ? (
            <div className="loading-spinner"><div className="spinner" /></div>
          ) : jobs.length > 0 ? (
            <div>
              <h3 style={{ fontSize: 13, fontWeight: 600, marginBottom: 8, color: 'var(--text-secondary)' }}>Jobs</h3>
              {jobs.map(job => (
                <div key={job.id} style={{ marginBottom: 12, padding: 8, background: 'var(--bg-overlay)', borderRadius: 'var(--radius-sm)' }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 4 }}>
                    <span style={{ fontSize: 13, fontWeight: 500 }}>{job.name}</span>
                    {getStatusBadge(job.status, job.conclusion || '')}
                  </div>
                  {job.steps.map((step, i) => (
                    <div key={i} className="job-step">
                      <div className={`step-indicator ${step.conclusion === 'success' ? 'completed' : step.conclusion === 'failure' ? 'failed' : step.status === 'in_progress' ? 'running' : 'pending'}`} />
                      <span style={{ flex: 1 }}>{step.name}</span>
                    </div>
                  ))}
                  <button className="btn btn-ghost btn-sm" onClick={() => handleViewLogs(job.id)}
                    style={{ marginTop: 4 }}>View logs</button>
                </div>
              ))}
            </div>
          ) : null}

          {logs && (
            <div className="code-block" style={{ marginTop: 16 }}>
              <div className="code-header">Logs</div>
              <div className="code-content" style={{ maxHeight: 400, overflow: 'auto', whiteSpace: 'pre-wrap', fontSize: 11 }}>
                {logs}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
