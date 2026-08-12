import React, { useState, useEffect, useCallback } from 'react';
import { API, Repo, formatRelativeTime } from '../lib/api';

interface RepoDetailPageProps {
  repoFullName: string;
  addToast: (message: string, type?: string) => void;
  onNavigate: (page: string) => void;
  onOpenExternal: (url: string) => void;
}

const languageColors: Record<string, string> = {
  TypeScript: '#3178c6',
  JavaScript: '#f1e05a',
  Go: '#00ADD8',
  Rust: '#dea584',
  Python: '#3572A5',
  Java: '#b07219',
  'C#': '#178600',
  Ruby: '#701516',
  Swift: '#F05138',
  Kotlin: '#A97BFF',
  HTML: '#e34c26',
  CSS: '#563d7c',
  Shell: '#89e051',
  Dockerfile: '#384d54',
};

// RepoDetailPage 仓库详情页：展示仓库元信息 + 快捷入口（PR / Issues / Actions / 在 GitHub 打开）
export default function RepoDetailPage({ repoFullName, addToast, onNavigate, onOpenExternal }: RepoDetailPageProps) {
  const [repo, setRepo] = useState<Repo | null>(null);
  const [readme, setReadme] = useState<string>('');
  const [loading, setLoading] = useState(true);

  const loadDetail = useCallback(async () => {
    setLoading(true);
    try {
      // 用 repo: 限定符精确搜索指定仓库
      const str = await API.SearchRepos(`repo:${repoFullName}`);
      const result = JSON.parse(str);
      const found = result.items?.find((r: Repo) => r.fullName === repoFullName);
      setRepo(found || result.items?.[0] || null);
    } catch (e: any) {
      addToast('Failed to load repo detail: ' + (e?.message || 'error'), 'error');
    } finally {
      setLoading(false);
    }
  }, [repoFullName, addToast]);

  const loadReadme = useCallback(async () => {
    try {
      const str = await API.GetRepoContents(repoFullName, 'README.md');
      const data = JSON.parse(str);
      if (data?.content && data.encoding === 'base64') {
        // base64 解码
        try {
          const decoded = atob(data.content.replace(/\s/g, ''));
          // 简单处理 UTF-8（非 ASCII 字符可能乱码，但基础展示够用）
          setReadme(decoded);
        } catch {
          setReadme('');
        }
      } else {
        setReadme('');
      }
    } catch {
      setReadme('');
    }
  }, [repoFullName]);

  useEffect(() => {
    loadDetail();
    loadReadme();
  }, [loadDetail, loadReadme]);

  if (loading) {
    return <div className="loading-spinner"><div className="spinner" /></div>;
  }

  if (!repo) {
    return (
      <div className="empty-state">
        <h3>Repository not found</h3>
        <p>Could not load details for "{repoFullName}".</p>
        <button className="btn btn-primary btn-sm" onClick={loadDetail}>Retry</button>
      </div>
    );
  }

  const langColor = languageColors[repo.language] || '#8b949e';
  const githubUrl = `https://github.com/${repo.fullName}`;

  return (
    <div className="fade-in" style={{ paddingBottom: 32 }}>
      {/* 仓库头部信息 */}
      <div style={{
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-muted)',
        borderRadius: 'var(--radius-md)',
        padding: 20,
        marginBottom: 16,
      }}>
        <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 16, flexWrap: 'wrap' }}>
          <div style={{ flex: 1, minWidth: 200 }}>
            <h2 style={{ margin: 0, fontSize: 20, fontWeight: 600, color: 'var(--text-primary)' }}>
              {repo.fullName}
            </h2>
            {repo.description && (
              <p style={{ margin: '8px 0 0', color: 'var(--text-secondary)', fontSize: 13 }}>
                {repo.description}
              </p>
            )}
            <div style={{ display: 'flex', gap: 16, marginTop: 12, fontSize: 12, color: 'var(--text-secondary)', flexWrap: 'wrap' }}>
              {repo.language && (
                <span>
                  <span style={{ display: 'inline-block', width: 12, height: 12, borderRadius: '50%', background: langColor, marginRight: 4, verticalAlign: 'middle' }} />
                  {repo.language}
                </span>
              )}
              <span>★ {repo.stars.toLocaleString()} stars</span>
              <span>⑂ {repo.forks.toLocaleString()} forks</span>
              <span>! {repo.openIssues.toLocaleString()} open issues</span>
              <span>{repo.private ? 'Private' : 'Public'}</span>
              <span>Updated {formatRelativeTime(repo.updatedAt)}</span>
            </div>
          </div>
          <button
            className="btn btn-ghost btn-sm"
            onClick={() => onOpenExternal(githubUrl)}
            title="Open on GitHub"
          >
            Open on GitHub ↗
          </button>
        </div>
      </div>

      {/* 快捷入口卡片 */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))',
        gap: 12,
        marginBottom: 16,
      }}>
        <button
          className="repo-card"
          style={{ cursor: 'pointer', textAlign: 'left', background: 'var(--bg-elevated)' }}
          onClick={() => onNavigate('pull-requests')}
        >
          <div style={{ fontSize: 24, marginBottom: 4 }}>🔀</div>
          <div style={{ fontWeight: 600, color: 'var(--text-primary)' }}>Pull Requests</div>
          <div style={{ fontSize: 11, color: 'var(--text-secondary)', marginTop: 2 }}>
            View PRs involving you in this repo
          </div>
        </button>
        <button
          className="repo-card"
          style={{ cursor: 'pointer', textAlign: 'left', background: 'var(--bg-elevated)' }}
          onClick={() => onNavigate('issues')}
        >
          <div style={{ fontSize: 24, marginBottom: 4 }}>◯</div>
          <div style={{ fontWeight: 600, color: 'var(--text-primary)' }}>Issues</div>
          <div style={{ fontSize: 11, color: 'var(--text-secondary)', marginTop: 2 }}>
            View issues involving you in this repo
          </div>
        </button>
        <button
          className="repo-card"
          style={{ cursor: 'pointer', textAlign: 'left', background: 'var(--bg-elevated)' }}
          onClick={() => onNavigate('actions')}
        >
          <div style={{ fontSize: 24, marginBottom: 4 }}>▶</div>
          <div style={{ fontWeight: 600, color: 'var(--text-primary)' }}>Actions</div>
          <div style={{ fontSize: 11, color: 'var(--text-secondary)', marginTop: 2 }}>
            View workflow runs and logs
          </div>
        </button>
      </div>

      {/* README 预览 */}
      {readme && (
        <div style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-muted)',
          borderRadius: 'var(--radius-md)',
          padding: 20,
        }}>
          <h3 style={{ margin: '0 0 12px', fontSize: 14, color: 'var(--text-secondary)', textTransform: 'uppercase', letterSpacing: 0.5 }}>
            README.md
          </h3>
          <pre style={{
            margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-word',
            fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--text-primary)',
            maxHeight: 400, overflow: 'auto',
          }}>
            {readme.slice(0, 5000)}{readme.length > 5000 ? '\n\n...(truncated)' : ''}
          </pre>
        </div>
      )}
    </div>
  );
}
