import React, { useState, useEffect, useCallback } from 'react';
import { API, Repo, CachedRepoResponse } from '../lib/api';
import { Button } from '../components/ui/button';
import { Select } from '../components/ui/select';
import { Badge } from '../components/ui/badge';
import { BookMarked, RefreshCw, Loader2, Star, GitFork } from 'lucide-react';

interface RepositoriesPageProps {
  searchQuery: string;
  onSearchChange: (query: string) => void;
  onSelect: (repo: Repo) => void;
  addToast: (message: string, type?: string) => void;
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

export default function RepositoriesPage({
  searchQuery,
  onSearchChange,
  onSelect,
  addToast,
}: RepositoriesPageProps) {
  const [repos, setRepos] = useState<Repo[]>([]);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [cachedAt, setCachedAt] = useState(0);
  const [sort, setSort] = useState<'updated' | 'created' | 'full_name'>('updated');
  const [searchResults, setSearchResults] = useState<{ totalCount: number; items: Repo[] } | null>(null);
  const [searching, setSearching] = useState(false);

  const loadRepos = useCallback(async () => {
    setLoading(true);
    try {
      const str = await API.GetMyRepos(sort);
      const resp: CachedRepoResponse<Repo> = JSON.parse(str);
      setRepos(Array.isArray(resp.data) ? resp.data : []);
      setCachedAt(resp.cachedAt || 0);
      setSyncing(resp.syncing || false);
    } catch (e: any) {
      const msg = e?.message || e?.error || (typeof e === 'string' ? e : 'unknown error');
      addToast('Failed to load repositories: ' + msg, 'error');
      setRepos([]);
    } finally {
      setLoading(false);
    }
  }, [sort, addToast]);

  const handleForceSync = useCallback(async () => {
    setSyncing(true);
    try {
      await API.SyncRepos('mine');
      await loadRepos();
      addToast('Repositories synced', 'success');
    } catch (e: any) {
      const msg = e?.message || e?.error || (typeof e === 'string' ? e : 'unknown error');
      addToast('Sync failed: ' + msg, 'error');
      setSyncing(false);
    }
  }, [loadRepos, addToast]);

  useEffect(() => {
    loadRepos();
  }, [loadRepos]);

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

  return (
    <div className="animate-fade-in">
      {/* Filter Bar */}
      <div className="mb-4 flex flex-wrap items-center gap-2 rounded-md border border-border bg-background p-2">
        <span className="text-xs font-medium text-muted-foreground">Sort:</span>
        <Select
          className="w-auto text-xs"
          value={sort}
          onChange={(e) => setSort(e.target.value as any)}
        >
          <option value="updated">Recently updated</option>
          <option value="created">Recently created</option>
          <option value="full_name">Name</option>
        </Select>
        <div className="flex-1" />
        {!searchResults && (
          <span className="text-xs text-muted-foreground">
            {repos.length} repos
            {cachedAt > 0 && (
              <span className="ml-2">· cached {new Date(cachedAt * 1000).toLocaleString()}</span>
            )}
            {syncing && <span className="ml-2 text-primary">· syncing…</span>}
          </span>
        )}
        {searchResults && (
          <span className="text-xs text-muted-foreground">{searchResults.totalCount} results</span>
        )}
        <Button variant="ghost" size="sm" onClick={handleForceSync} disabled={syncing} title="Force sync from GitHub">
          {syncing ? (
            <>
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              Syncing…
            </>
          ) : (
            <>
              <RefreshCw className="h-3.5 w-3.5" />
              Sync
            </>
          )}
        </Button>
        <Button variant="ghost" size="sm" onClick={loadRepos}>
          <RefreshCw className="h-3.5 w-3.5" />
          Refresh
        </Button>
      </div>

      {loading || searching ? (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-primary" />
        </div>
      ) : displayRepos.items.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
          <BookMarked className="mb-3 h-12 w-12 opacity-40" />
          <h3 className="mb-1 text-base font-semibold text-secondary-foreground">
            No repositories found
          </h3>
          <p className="text-sm">Try a different search or sort option.</p>
        </div>
      ) : (
        <div className="overflow-hidden rounded-md border border-border">
          {displayRepos.items.map((repo) => (
            <div
              key={repo.fullName}
              onClick={() => onSelect(repo)}
              className="flex cursor-pointer gap-3 border-b border-border px-4 py-3 transition-colors hover:bg-muted/50 last:border-b-0"
            >
              <div className="flex-1 min-w-0">
                <div className="mb-1 text-[15px] font-semibold text-primary">{repo.fullName}</div>
                {repo.description && (
                  <div className="mb-2 line-clamp-2 text-sm text-muted-foreground">
                    {repo.description}
                  </div>
                )}
                <div className="flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
                  {repo.language && (
                    <span className="flex items-center gap-1">
                      <span
                        className="inline-block h-3 w-3 rounded-full"
                        style={{ background: languageColors[repo.language] || '#8b949e' }}
                      />
                      {repo.language}
                    </span>
                  )}
                  <span className="flex items-center gap-1">
                    <Star className="h-3 w-3" />
                    {repo.stars.toLocaleString()}
                  </span>
                  <span className="flex items-center gap-1">
                    <GitFork className="h-3 w-3" />
                    {repo.forks.toLocaleString()}
                  </span>
                  <Badge variant={repo.private ? 'secondary' : 'outline'} className="text-xs">
                    {repo.private ? 'Private' : 'Public'}
                  </Badge>
                  <span className="ml-auto">
                    Updated {new Date(repo.updatedAt).toLocaleDateString()}
                  </span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
