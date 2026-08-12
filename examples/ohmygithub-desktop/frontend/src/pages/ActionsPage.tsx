import React, { useState, useCallback } from 'react';
import { API, WorkflowRun, Job, formatRelativeTime } from '../lib/api';

interface ActionsPageProps {
  addToast: (message: string, type?: string) => void;
  initialRepo?: string;
}

export default function ActionsPage({ addToast, initialRepo }: ActionsPageProps) {
  const [repoInput, setRepoInput] = useState(initialRepo || 'octocat/Hello-World');
  const [runs, setRuns] = useState<WorkflowRun[]>([]);
  const [selectedRun, setSelectedRun] = useState<WorkflowRun | null>(null);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [logs, setLogs] = useState('');
  const [loading, setLoading] = useState(false);
  const [loadingJobs, setLoadingJobs] = useState(false);

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
        <div className="filter-bar">
          <span className="filter-label">Repository:</span>
          <input className="input" style={{ width: 280, fontSize: 13 }} value={repoInput}
            onChange={e => setRepoInput(e.target.value)}
            placeholder="owner/repo" />
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
            <p>Enter a repository and click Load to view its Actions.</p>
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
