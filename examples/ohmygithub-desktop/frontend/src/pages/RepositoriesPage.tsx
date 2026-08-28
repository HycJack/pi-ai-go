import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { API, Repo, CachedRepoResponse } from '../lib/api';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Select } from '../components/ui/select';
import { BookMarked, RefreshCw, Loader2, Star, GitFork, X } from 'lucide-react';

export interface RepoFilters {
  keyword: string;
  language: string;
  sort: 'updated' | 'created' | 'full_name';
}

interface RepositoriesPageProps {
  onSelect: (repo: Repo) => void;
  addToast: (message: string, type?: string) => void;
  filters: RepoFilters;
  onFiltersChange: (f: RepoFilters) => void;
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

export default function RepositoriesPage({ onSelect, addToast, filters, onFiltersChange }: RepositoriesPageProps) {
  const [repos, setRepos] = useState<Repo[]>([]);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [cachedAt, setCachedAt] = useState(0);
  const { keyword, language: languageFilter, sort } = filters;
  const setKeyword = (v: string) => onFiltersChange({ ...filters, keyword: v });
  const setLanguageFilter = (v: string) => onFiltersChange({ ...filters, language: v });
  const setSort = (v: RepoFilters['sort']) => onFiltersChange({ ...filters, sort: v });

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

  // Get unique languages from repos
  const languages = useMemo(() => {
    const langs = new Set(repos.map(r => r.language).filter(Boolean));
    return Array.from(langs).sort();
  }, [repos]);

  // Filter repos
  const filteredRepos = useMemo(() => {
    let result = repos;

    // Filter by keyword
    if (keyword.trim()) {
      const kw = keyword.toLowerCase();
      result = result.filter(r =>
        r.name.toLowerCase().includes(kw) ||
        r.fullName.toLowerCase().includes(kw) ||
        r.description?.toLowerCase().includes(kw)
      );
    }

    // Filter by language
    if (languageFilter) {
      result = result.filter(r => r.language === languageFilter);
    }

    return result;
  }, [repos, keyword, languageFilter]);

  return (
    <div className="animate-fade-in">
      {/* Filter Bar */}
      <div className="mb-4 flex flex-wrap items-center gap-2 rounded-md border border-border bg-background p-2">
        {/* Local Search */}
        <div className="relative">
          <Input
            className="h-8 w-[200px] pl-8 text-xs"
            placeholder="Filter by name..."
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
          />
          {keyword ? (
            <button
              className="absolute left-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              onClick={() => setKeyword('')}
            >
              <X className="h-3.5 w-3.5" />
            </button>
          ) : (
            <BookMarked className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          )}
        </div>

        <div className="h-5 w-px bg-border" />

        {/* Language Filter */}
        <Select
          className="h-8 w-auto text-xs"
          value={languageFilter}
          onChange={(e) => setLanguageFilter(e.target.value)}
        >
          <option value="">All Languages</option>
          {languages.map(lang => (
            <option key={lang} value={lang}>{lang}</option>
          ))}
        </Select>

        <div className="h-5 w-px bg-border" />

        {/* Sort */}
        <Select
          className="h-8 w-auto text-xs"
          value={sort}
          onChange={(e) => setSort(e.target.value as any)}
        >
          <option value="updated">Recently updated</option>
          <option value="created">Recently created</option>
          <option value="full_name">Name</option>
        </Select>

        <div className="flex-1" />

        {/* Stats */}
        <span className="text-xs text-muted-foreground">
          {filteredRepos.length}/{repos.length} repos
          {cachedAt > 0 && (
            <span className="ml-2">· cached {new Date(cachedAt * 1000).toLocaleString()}</span>
          )}
          {syncing && <span className="ml-2 text-primary">· syncing…</span>}
        </span>

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
      </div>

      {/* Repo List */}
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-primary" />
        </div>
      ) : filteredRepos.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
          <BookMarked className="mb-3 h-8 w-8 opacity-50" />
          <p className="text-sm">
            {repos.length === 0 ? 'No repositories found' : 'No repos match filters'}
          </p>
        </div>
      ) : (
        <div className="space-y-1.5">
          {filteredRepos.map((repo) => (
            <div
              key={repo.fullName}
              className="group flex items-start gap-3 rounded-lg border border-border bg-background p-3 transition-colors hover:border-primary/50 hover:bg-accent cursor-pointer"
              onClick={() => onSelect(repo)}
            >
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-foreground group-hover:text-primary transition-colors">
                    {repo.name}
                  </span>
                  {repo.private && (
                    <span className="rounded bg-secondary px-1.5 py-0.5 text-xs font-medium text-secondary-foreground">
                      Private
                    </span>
                  )}
                </div>
                <p className="mt-0.5 text-xs text-muted-foreground line-clamp-1">
                  {repo.description || 'No description'}
                </p>
                <div className="mt-1.5 flex items-center gap-3 text-xs text-muted-foreground">
                  {repo.language && (
                    <span className="flex items-center gap-1">
                      <span
                        className="h-2 w-2 rounded-full"
                        style={{ backgroundColor: languageColors[repo.language] || '#6b7280' }}
                      />
                      {repo.language}
                    </span>
                  )}
                  <span className="flex items-center gap-1">
                    <Star className="h-3 w-3" />
                    {repo.stars}
                  </span>
                  <span className="flex items-center gap-1">
                    <GitFork className="h-3 w-3" />
                    {repo.forks}
                  </span>
                  {repo.updatedAt && (
                    <span>Updated {new Date(repo.updatedAt).toLocaleDateString()}</span>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
