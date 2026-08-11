import React, { useState, useEffect } from 'react';
import { API, PullRequest, Issue, Notification, formatRelativeTime } from '../lib/api';

interface PreviewPanelProps {
  open: boolean;
  item: any;
  onClose: () => void;
}

export default function PreviewPanel({ open, item, onClose }: PreviewPanelProps) {
  const [diffContent, setDiffContent] = useState<string>('');
  const [fileContent, setFileContent] = useState<string>('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setDiffContent('');
    setFileContent('');
    if (!item) return;

    if (item._type === 'pr' && item._diff) {
      setDiffContent(item._diff);
    }
  }, [item]);

  if (!open || !item) return null;

  const pr = item as PullRequest;
  const issue = item as Issue;
  const notif = item as Notification;

  const renderDetail = () => {
    if (item._type === 'pr') {
      return (
        <div>
          <div className="detail-header">
            <div className="detail-title">
              <span className={`state-icon ${pr.draft ? 'draft' : pr.state === 'open' ? 'open' : 'closed'}`} style={{ marginRight: 8 }}>
                <svg viewBox="0 0 16 16" fill="currentColor" width="16" height="16">
                  <path d="M7.177 3.073L9.573.677A.25.25 0 0110 .854v4.792a.25.25 0 01-.427.177L7.177 3.427a.25.25 0 010-.354zM3.75 2.5a.75.75 0 100 1.5.75.75 0 000-1.5zM2.5 3.25a1.25 1.25 0 112.5 0 1.25 1.25 0 01-2.5 0zM3.75 12a.75.75 0 100 1.5.75.75 0 000-1.5zM2.5 12.75a1.25 1.25 0 112.5 0 1.25 1.25 0 01-2.5 0zM10 12a.75.75 0 100 1.5.75.75 0 000-1.5zM8.75 12.75a1.25 1.25 0 112.5 0 1.25 1.25 0 01-2.5 0z"/>
                </svg>
              </span>
              {pr.title}
            </div>
            <div className="detail-meta">
              <span>#{pr.number}</span>
              <span>by {pr.user}</span>
              <span>{formatRelativeTime(pr.createdAt)}</span>
              <span>{pr.repo}</span>
            </div>
          </div>

          {pr.labels.length > 0 && (
            <div style={{ marginBottom: 16, display: 'flex', gap: 4, flexWrap: 'wrap' }}>
              {pr.labels.map((l, i) => (
                <span key={i} className="label" style={{
                  background: `#${l.color}22`,
                  borderColor: `#${l.color}44`,
                  color: `#${l.color}`,
                }}>
                  {l.name}
                </span>
              ))}
            </div>
          )}

          <div style={{ marginBottom: 16, display: 'flex', gap: 8 }}>
            <button className="btn btn-sm" onClick={() => API.OpenExternal(`https://github.com/${pr.repo}/pull/${pr.number}`)}>
              Open on GitHub
            </button>
          </div>
        </div>
      );
    }

    if (item._type === 'issue') {
      return (
        <div>
          <div className="detail-header">
            <div className="detail-title">
              <span className={`state-icon ${issue.state === 'open' ? 'open' : 'closed'}`} style={{ marginRight: 8 }}>
                <svg viewBox="0 0 16 16" fill="currentColor" width="16" height="16">
                  <path d="M8 9.5a1.5 1.5 0 100-3 1.5 1.5 0 000 3zM8 0a8 8 0 110 16A8 8 0 018 0zM1.5 8a6.5 6.5 0 1013 0 6.5 6.5 0 00-13 0z"/>
                </svg>
              </span>
              {issue.title}
            </div>
            <div className="detail-meta">
              <span>#{issue.number}</span>
              <span>by {issue.user}</span>
              <span>{formatRelativeTime(issue.createdAt)}</span>
              <span>{issue.repo}</span>
            </div>
          </div>
          {issue.labels.length > 0 && (
            <div style={{ marginBottom: 16, display: 'flex', gap: 4, flexWrap: 'wrap' }}>
              {issue.labels.map((l, i) => (
                <span key={i} className="label" style={{
                  background: `#${l.color}22`,
                  borderColor: `#${l.color}44`,
                  color: `#${l.color}`,
                }}>
                  {l.name}
                </span>
              ))}
            </div>
          )}
          {issue.body && (
            <div className="detail-body">{issue.body}</div>
          )}
        </div>
      );
    }

    if (item._type === 'notification') {
      return (
        <div>
          <div className="detail-header">
            <div className="detail-title">{notif.title}</div>
            <div className="detail-meta">
              <span>{notif.repo}</span>
              <span>{notif.type}</span>
              <span>{formatRelativeTime(notif.updatedAt)}</span>
            </div>
          </div>
        </div>
      );
    }

    return <div className="empty-state"><h3>No details available</h3></div>;
  };

  return (
    <div className="preview-panel">
      <div className="preview-header">
        <button className="btn btn-ghost btn-sm btn-icon" onClick={onClose}>
          <svg viewBox="0 0 24 24" fill="currentColor" width="16" height="16">
            <path d="M15.41 7.41L14 6l-6 6 6 6 1.41-1.41L10.83 12z"/>
          </svg>
        </button>
        <h3>Preview</h3>
      </div>
      <div className="preview-body" style={{ overflow: 'auto' }}>
        {renderDetail()}
      </div>
    </div>
  );
}
