import React, { useState, useEffect, useCallback } from 'react';
import { API, Repo } from '../lib/api';

interface RepositoriesPageProps {
  searchQuery: string;
  onSearchChange: (query: string) => void;
  onSelect: (repo: Repo) => void;
  addToast: (message: string, type?: string) => void;
}

export default function RepositoriesPage({ searchQuery, onSearchChange, onSelect, addToast }: RepositoriesPageProps) {
  const [repos, setRepos] = useState<Repo[]>([]);
  const [loading, setLoading] = useState(true);
  const [sort, setSort] = useState<'updated' | 'created' | 'full_name'>('updated');
  const [searchResults, setSearchResults] = useState<{ totalCount: number; items: Repo[] } | null>(null);
  const [searching, setSearching] = useState(false);

  const loadRepos = useCallback(async () => {
    setLoading(true);
    try {
      const str = await API.GetMyRepos(sort);
      setRepos(JSON.parse(str));
    } catch (e) {
      addToast('Failed to load repositories', 'error');
    } finally {
      setLoading(false);
    }
  }, [sort, addToast]);

  useEffect(() => { loadRepos(); }, [loadRepos]);

  const handleSearch = useCallback(async () => {
    if (!searchQuery.trim()) {
      setSearchResults(null);
      loadRepos();
      return;
    }
    setSearching(true);
    try {
      const str = await API.SearchRepos(searchQuery.trim());
      const result = JSON.parse(str);
      setSearchResults(result);
      if (result.totalCount === 0) {
        addToast('No repositories found', 'info');
      }
    } catch (e) {
      addToast('Search failed', 'error');
    } finally {
      setSearching(false);
    }
  }, [searchQuery, addToast, loadRepos]);

  useEffect(() => {
    const timer = setTimeout(() => {
      if (searchQuery) handleSearch();
      else setSearchResults(null);
    }, 500);
    return () => clearTimeout(timer);
  }, [searchQuery, handleSearch]);

  const displayRepos = searchResults || { items: repos, totalCount: repos.length };

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

  return (
    <div className="fade-in">
      <div className="filter-bar">
        <span className="filter-label">Sort:</span>
        <select className="select" style={{ width: 'auto', fontSize: 12 }} value={sort} onChange={e => setSort(e.target.value as any)}>
          <option value="updated">Recently updated</option>
          <option value="created">Recently created</option>
          <option value="full_name">Name</option>
        </select>
        <div style={{ flex: 1 }} />
        {searchResults && <span style={{ fontSize: 12, color: 'var(--text-secondary)' }}>{searchResults.totalCount} results</span>}
        <button className="btn btn-ghost btn-sm" onClick={loadRepos}>Refresh</button>
      </div>

      {loading || searching ? (
        <div className="loading-spinner"><div className="spinner" /></div>
      ) : displayRepos.items.length === 0 ? (
        <div className="empty-state">
          <svg viewBox="0 0 24 24" fill="currentColor" width="48" height="48" className="icon">
            <path d="M18 2H6c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 18H6V4h2v8l2.5-1.5L13 12V4h5v16z"/>
          </svg>
          <h3>No repositories found</h3>
          <p>Try a different search or sort option.</p>
        </div>
      ) : (
        <div style={{ border: '1px solid var(--border-muted)', borderRadius: 'var(--radius-md)', overflow: 'hidden' }}>
          {displayRepos.items.map((repo) => (
            <div key={repo.fullName} className="repo-card" onClick={() => onSelect(repo)}>
              <div className="repo-card-content">
                <div className="repo-card-name">{repo.fullName}</div>
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
                  {repo.private && <span className="tag">Private</span>}
                  {!repo.private && <span className="tag">Public</span>}
                  <span style={{ marginLeft: 'auto' }}>Updated {new Date(repo.updatedAt).toLocaleDateString()}</span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
