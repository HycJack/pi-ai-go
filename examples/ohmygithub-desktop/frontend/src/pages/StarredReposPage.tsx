import React, { useState, useEffect, useCallback } from 'react';
import { API, StarredRepo, StarGroup, CachedRepoResponse, formatRelativeTime } from '../lib/api';

interface StarredReposPageProps {
  addToast: (message: string, type?: string) => void;
  onSelectRepo: (repo: StarredRepo) => void;
  starGroups: StarGroup[];
  onGroupsChange: () => void; // 分组变更后通知父组件刷新 settings
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

export default function StarredReposPage({ addToast, onSelectRepo, starGroups, onGroupsChange }: StarredReposPageProps) {
  const [repos, setRepos] = useState<StarredRepo[]>([]);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [cachedAt, setCachedAt] = useState(0);
  const [keyword, setKeyword] = useState('');
  const [activeGroupID, setActiveGroupID] = useState<string>(''); // '' = 全部
  const [showNewGroupInput, setShowNewGroupInput] = useState(false);
  const [newGroupName, setNewGroupName] = useState('');
  const [renameTarget, setRenameTarget] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState('');
  // 记录每个 repo 的下拉菜单展开状态
  const [groupMenuFor, setGroupMenuFor] = useState<string | null>(null);

  const loadRepos = useCallback(async () => {
    setLoading(true);
    try {
      const str = await API.GetStarredRepos();
      const resp: CachedRepoResponse<StarredRepo> = JSON.parse(str);
      setRepos(Array.isArray(resp.data) ? resp.data : []);
      setCachedAt(resp.cachedAt || 0);
      setSyncing(resp.syncing || false);
    } catch (e: any) {
      const msg = e?.message || e?.error || (typeof e === 'string' ? e : 'unknown error');
      addToast('Failed to load starred repos: ' + msg, 'error');
      setRepos([]);
    } finally {
      setLoading(false);
    }
  }, [addToast]);

  // 手动强制同步 starred repos
  const handleForceSync = useCallback(async () => {
    setSyncing(true);
    try {
      await API.SyncRepos('starred');
      await loadRepos();
      addToast('Starred repos synced', 'success');
    } catch (e: any) {
      const msg = e?.message || e?.error || (typeof e === 'string' ? e : 'unknown error');
      addToast('Sync failed: ' + msg, 'error');
      setSyncing(false);
    }
  }, [loadRepos, addToast]);

  useEffect(() => { loadRepos(); }, [loadRepos]);

  // 按关键字 + 分组过滤
  const filtered = repos.filter(r => {
    if (keyword) {
      const k = keyword.toLowerCase();
      if (!r.fullName.toLowerCase().includes(k) &&
          !(r.description || '').toLowerCase().includes(k)) {
        return false;
      }
    }
    if (activeGroupID && !r.groups.includes(activeGroupID)) return false;
    return true;
  });

  const handleCreateGroup = async () => {
    const name = newGroupName.trim();
    if (!name) return;
    try {
      await API.CreateStarGroup(name);
      setNewGroupName('');
      setShowNewGroupInput(false);
      onGroupsChange();
      addToast(`Group "${name}" created`, 'success');
    } catch (e: any) {
      addToast('Create group failed: ' + (e?.message || 'error'), 'error');
    }
  };

  const handleDeleteGroup = async (id: string, name: string) => {
    if (!confirm(`Delete group "${name}"? Repos will not be unstarred.`)) return;
    try {
      await API.DeleteStarGroup(id);
      if (activeGroupID === id) setActiveGroupID('');
      onGroupsChange();
      addToast(`Group "${name}" deleted`, 'info');
    } catch (e: any) {
      addToast('Delete group failed: ' + (e?.message || 'error'), 'error');
    }
  };

  const handleRenameGroup = async (id: string) => {
    const name = renameValue.trim();
    if (!name) return;
    try {
      await API.RenameStarGroup(id, name);
      setRenameTarget(null);
      setRenameValue('');
      onGroupsChange();
      addToast('Group renamed', 'success');
    } catch (e: any) {
      addToast('Rename failed: ' + (e?.message || 'error'), 'error');
    }
  };

  const handleToggleRepoInGroup = async (groupID: string, repoFullName: string, currentlyIn: boolean) => {
    try {
      if (currentlyIn) {
        await API.RemoveRepoFromStarGroup(groupID, repoFullName);
      } else {
        await API.AddRepoToStarGroup(groupID, repoFullName);
      }
      // 更新本地 repos 状态，避免重新拉取
      setRepos(prev => prev.map(r => {
        if (r.fullName !== repoFullName) return r;
        const set = new Set(r.groups);
        if (currentlyIn) set.delete(groupID); else set.add(groupID);
        return { ...r, groups: Array.from(set) };
      }));
      onGroupsChange();
    } catch (e: any) {
      addToast('Update group failed: ' + (e?.message || 'error'), 'error');
    }
  };

  const handleUnstar = async (repoFullName: string) => {
    if (!confirm(`Unstar ${repoFullName}?`)) return;
    try {
      await API.UnstarRepo(repoFullName);
      setRepos(prev => prev.filter(r => r.fullName !== repoFullName));
      addToast(`Unstarred ${repoFullName}`, 'info');
    } catch (e: any) {
      addToast('Unstar failed: ' + (e?.message || 'error'), 'error');
    }
  };

  return (
    <div className="fade-in">
      {/* 顶部过滤栏 */}
      <div className="filter-bar">
        <input
          className="input"
          style={{ width: 220, fontSize: 12 }}
          placeholder="Filter by name or description..."
          value={keyword}
          onChange={e => setKeyword(e.target.value)}
        />
        <div style={{ width: 1, height: 20, background: 'var(--border-muted)' }} />
        <span className="filter-label">Group:</span>
        <button
          className={`btn btn-sm ${activeGroupID === '' ? 'btn-primary' : 'btn-ghost'}`}
          onClick={() => setActiveGroupID('')}
        >All</button>
        {starGroups.map(g => (
          <button
            key={g.id}
            className={`btn btn-sm ${activeGroupID === g.id ? 'btn-primary' : 'btn-ghost'}`}
            onClick={() => setActiveGroupID(g.id)}
            title={`${g.repos.length} repos`}
          >{g.name}</button>
        ))}
        <div style={{ flex: 1 }} />
        <span style={{ fontSize: 11, color: 'var(--text-tertiary)' }}>
          {repos.length} repos
          {cachedAt > 0 && <span style={{ marginLeft: 8 }}>· cached {new Date(cachedAt * 1000).toLocaleString()}</span>}
          {syncing && <span style={{ marginLeft: 8, color: 'var(--accent)' }}>· syncing…</span>}
        </span>
        <button className="btn btn-ghost btn-sm" onClick={() => setShowNewGroupInput(v => !v)}>+ New Group</button>
        <button className="btn btn-ghost btn-sm" onClick={handleForceSync} disabled={syncing} title="Force sync from GitHub">
          {syncing ? 'Syncing…' : 'Sync'}
        </button>
        <button className="btn btn-ghost btn-sm" onClick={loadRepos}>Refresh</button>
      </div>

      {/* 新建分组输入框 */}
      {showNewGroupInput && (
        <div style={{ display: 'flex', gap: 8, marginBottom: 12, alignItems: 'center' }}>
          <input
            className="input"
            style={{ width: 240, fontSize: 12 }}
            placeholder="Group name (e.g. Frontend frameworks)"
            value={newGroupName}
            onChange={e => setNewGroupName(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') handleCreateGroup(); if (e.key === 'Escape') setShowNewGroupInput(false); }}
            autoFocus
          />
          <button className="btn btn-primary btn-sm" onClick={handleCreateGroup}>Create</button>
          <button className="btn btn-ghost btn-sm" onClick={() => setShowNewGroupInput(false)}>Cancel</button>
        </div>
      )}

      {/* 分组管理列表 */}
      {starGroups.length > 0 && (
        <div style={{
          display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: 12,
          fontSize: 11, color: 'var(--text-secondary)'
        }}>
          {starGroups.map(g => (
            <div key={g.id} style={{
              border: '1px solid var(--border-muted)', borderRadius: 6,
              padding: '2px 8px', display: 'flex', alignItems: 'center', gap: 6,
            }}>
              {renameTarget === g.id ? (
                <>
                  <input
                    className="input"
                    style={{ width: 120, fontSize: 11, padding: '1px 4px' }}
                    value={renameValue}
                    onChange={e => setRenameValue(e.target.value)}
                    onKeyDown={e => { if (e.key === 'Enter') handleRenameGroup(g.id); if (e.key === 'Escape') setRenameTarget(null); }}
                    autoFocus
                  />
                  <button className="btn btn-ghost btn-sm" style={{ padding: '0 4px' }} onClick={() => handleRenameGroup(g.id)}>OK</button>
                  <button className="btn btn-ghost btn-sm" style={{ padding: '0 4px' }} onClick={() => setRenameTarget(null)}>×</button>
                </>
              ) : (
                <>
                  <span><strong>{g.name}</strong> · {g.repos.length}</span>
                  <button
                    className="btn btn-ghost btn-sm"
                    style={{ padding: '0 4px', fontSize: 10 }}
                    onClick={() => { setRenameTarget(g.id); setRenameValue(g.name); }}
                    title="Rename"
                  >✎</button>
                  <button
                    className="btn btn-ghost btn-sm"
                    style={{ padding: '0 4px', fontSize: 10 }}
                    onClick={() => handleDeleteGroup(g.id, g.name)}
                    title="Delete"
                  >×</button>
                </>
              )}
            </div>
          ))}
        </div>
      )}

      {loading ? (
        <div className="loading-spinner"><div className="spinner" /></div>
      ) : filtered.length === 0 ? (
        <div className="empty-state">
          <svg viewBox="0 0 24 24" fill="currentColor" width="48" height="48" className="icon">
            <path d="M12 .587l3.668 7.431 8.2 1.192-5.934 5.787 1.401 8.169L12 18.896l-7.335 3.868 1.401-8.169L.132 9.21l8.2-1.192z"/>
          </svg>
          <h3>No starred repositories</h3>
          <p>{activeGroupID ? 'This group has no repos yet.' : 'Star repos on GitHub to see them here.'}</p>
        </div>
      ) : (
        <div style={{ border: '1px solid var(--border-muted)', borderRadius: 'var(--radius-md)', overflow: 'hidden' }}>
          {filtered.map(repo => (
            <div
              key={repo.fullName}
              className="repo-card"
              style={{ position: 'relative', cursor: 'pointer' }}
              onClick={() => onSelectRepo(repo)}
            >
              <div className="repo-card-content">
                <div className="repo-card-name">
                  {repo.fullName}
                  {repo.groups.length > 0 && (
                    <span style={{ marginLeft: 8, fontSize: 10, color: 'var(--text-accent)' }}>
                      {repo.groups.map(gid => starGroups.find(g => g.id === gid)?.name).filter(Boolean).join(', ')}
                    </span>
                  )}
                </div>
                {repo.description && <div className="repo-card-desc">{repo.description}</div>}
                <div className="repo-card-meta">
                  {repo.language && (
                    <span>
                      <span style={{
                        display: 'inline-block', width: 12, height: 12,
                        borderRadius: '50%', background: languageColors[repo.language] || '#8b949e',
                        marginRight: 4, verticalAlign: 'middle'
                      }} />
                      {repo.language}
                    </span>
                  )}
                  <span>★ {repo.stars.toLocaleString()}</span>
                  <span>⑂ {repo.forks.toLocaleString()}</span>
                  <span>Updated {formatRelativeTime(repo.updatedAt)}</span>
                </div>
              </div>
              {/* 分组操作菜单 */}
              <div style={{ position: 'absolute', top: 8, right: 8, display: 'flex', gap: 4 }} onClick={e => e.stopPropagation()}>
                <button
                  className="btn btn-ghost btn-sm"
                  title="Assign to groups"
                  onClick={() => setGroupMenuFor(groupMenuFor === repo.fullName ? null : repo.fullName)}
                >Group ▾</button>
                <button
                  className="btn btn-ghost btn-sm"
                  title="Unstar"
                  onClick={() => handleUnstar(repo.fullName)}
                >Unstar</button>
              </div>
              {groupMenuFor === repo.fullName && (
                <div style={{
                  position: 'absolute', top: 36, right: 8, zIndex: 10,
                  background: 'var(--bg-elevated)', border: '1px solid var(--border-muted)',
                  borderRadius: 6, boxShadow: '0 4px 12px rgba(0,0,0,0.15)', padding: 4, minWidth: 180,
                }}>
                  {starGroups.length === 0 ? (
                    <div style={{ padding: '8px 12px', fontSize: 11, color: 'var(--text-secondary)' }}>
                      No groups yet. Create one first.
                    </div>
                  ) : (
                    starGroups.map(g => {
                      const inGroup = repo.groups.includes(g.id);
                      return (
                        <label
                          key={g.id}
                          style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '4px 8px', cursor: 'pointer', fontSize: 12 }}
                          onMouseEnter={e => (e.currentTarget.style.background = 'var(--bg-hover)')}
                          onMouseLeave={e => (e.currentTarget.style.background = '')}
                        >
                          <input
                            type="checkbox"
                            checked={inGroup}
                            onChange={() => handleToggleRepoInGroup(g.id, repo.fullName, inGroup)}
                          />
                          <span>{g.name}</span>
                          <span style={{ color: 'var(--text-secondary)', fontSize: 10 }}>({g.repos.length})</span>
                        </label>
                      );
                    })
                  )}
                  <div style={{ borderTop: '1px solid var(--border-muted)', marginTop: 4, paddingTop: 4 }}>
                    <button
                      className="btn btn-ghost btn-sm"
                      style={{ width: '100%', fontSize: 11 }}
                      onClick={() => { setGroupMenuFor(null); setShowNewGroupInput(true); }}
                    >+ New group</button>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
