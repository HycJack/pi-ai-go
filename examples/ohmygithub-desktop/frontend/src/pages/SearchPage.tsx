import React, { useState, useCallback, useEffect } from 'react';
import { API, Repo, SearchResult } from '../lib/api';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Search as SearchIcon, Loader2, Star, GitFork, ExternalLink, BookMarked } from 'lucide-react';

interface SearchPageProps {
  addToast: (message: string, type?: string) => void;
  onSelectRepo: (repo: Repo) => void;
  query: string;
  onQueryChange: (q: string) => void;
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

export default function SearchPage({ addToast, onSelectRepo, query, onQueryChange }: SearchPageProps) {
  const [results, setResults] = useState<Repo[]>([]);
  const [totalCount, setTotalCount] = useState(0);
  const [loading, setLoading] = useState(false);
  const [actionPending, setActionPending] = useState<string | null>(null);

  const doSearch = useCallback(
    async (q: string) => {
      const trimmed = q.trim();
      if (!trimmed) {
        setResults([]);
        setTotalCount(0);
        return;
      }
      setLoading(true);
      try {
        const str = await API.SearchRepos(trimmed);
        const result: SearchResult = JSON.parse(str);
        setResults(result.items || []);
        setTotalCount(result.totalCount || 0);
      } catch (e: any) {
        const msg = e?.message || e?.error || (typeof e === 'string' ? e : 'unknown error');
        addToast('Search failed: ' + msg, 'error');
        setResults([]);
        setTotalCount(0);
      } finally {
        setLoading(false);
      }
    },
    [addToast]
  );

  // Debounce search when query changes from outside (header input)
  useEffect(() => {
    const timer = setTimeout(() => doSearch(query), 400);
    return () => clearTimeout(timer);
  }, [query, doSearch]);

  const handleStar = useCallback(
    async (e: React.MouseEvent, repo: Repo) => {
      e.stopPropagation();
      setActionPending(repo.fullName);
      try {
        await API.StarRepo(repo.fullName);
        addToast(`Starred ${repo.fullName}`, 'success');
      } catch (err: any) {
        const msg = err?.message || err?.error || 'unknown error';
        addToast('Star failed: ' + msg, 'error');
      } finally {
        setActionPending(null);
      }
    },
    [addToast]
  );

  const handleFork = useCallback(
    async (e: React.MouseEvent, repo: Repo) => {
      e.stopPropagation();
      if (!confirm(`Fork ${repo.fullName} to your account?`)) return;
      setActionPending(repo.fullName);
      try {
        const str = await API.ForkRepo(repo.fullName);
        const out = JSON.parse(str);
        addToast(`Forked to ${out.fullName}`, 'success');
        await API.OpenExternal(out.htmlUrl || repo.htmlUrl || '');
      } catch (err: any) {
        const msg = err?.message || err?.error || 'unknown error';
        addToast('Fork failed: ' + msg, 'error');
      } finally {
        setActionPending(null);
      }
    },
    [addToast]
  );

  const handleOpen = useCallback(
    async (e: React.MouseEvent, repo: Repo) => {
      e.stopPropagation();
      if (repo.htmlUrl) {
        await API.OpenExternal(repo.htmlUrl);
      }
    },
    []
  );

  return (
    <div className="animate-fade-in">
      {/* Search Bar */}
      <div className="mb-4 flex flex-wrap items-center gap-2 rounded-md border border-border bg-background p-2">
        <div className="relative flex-1 min-w-[240px]">
          <SearchIcon className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground pointer-events-none" />
          <Input
            className="h-9 pl-9 text-sm"
            placeholder="Search GitHub repositories (e.g. react, language:go stars:>1000)"
            value={query}
            onChange={(e) => onQueryChange(e.target.value)}
            autoFocus
          />
        </div>
        <span className="text-xs text-muted-foreground">
          {loading ? (
            <>
              <Loader2 className="inline h-3 w-3 animate-spin" /> Searching…
            </>
          ) : query.trim() ? (
            `${totalCount.toLocaleString()} results`
          ) : (
            'Type to search'
          )}
        </span>
      </div>

      {/* Results */}
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-primary" />
        </div>
      ) : !query.trim() ? (
        <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
          <SearchIcon className="mb-3 h-10 w-10 opacity-40" />
          <h3 className="mb-1 text-base font-semibold text-secondary-foreground">
            Search GitHub
          </h3>
          <p className="text-sm">
            Find any public repository. Use qualifiers like{' '}
            <code className="rounded bg-muted px-1 py-0.5 text-xs">language:rust</code>,{' '}
            <code className="rounded bg-muted px-1 py-0.5 text-xs">stars:&gt;5000</code>, or{' '}
            <code className="rounded bg-muted px-1 py-0.5 text-xs">user:torvalds</code>.
          </p>
        </div>
      ) : results.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
          <BookMarked className="mb-3 h-8 w-8 opacity-40" />
          <p className="text-sm">No repositories found for "{query}"</p>
        </div>
      ) : (
        <div className="space-y-1.5">
          {results.map((repo) => {
            const pending = actionPending === repo.fullName;
            return (
              <div
                key={repo.fullName}
                className="group flex items-start gap-3 rounded-lg border border-border bg-background p-3 transition-colors hover:border-primary/50 hover:bg-accent cursor-pointer"
                onClick={() => onSelectRepo(repo)}
              >
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-foreground group-hover:text-primary transition-colors">
                      {repo.fullName}
                    </span>
                    {repo.private && (
                      <span className="rounded bg-secondary px-1.5 py-0.5 text-xs font-medium text-secondary-foreground">
                        Private
                      </span>
                    )}
                  </div>
                  {repo.description && (
                    <p className="mt-0.5 text-xs text-muted-foreground line-clamp-2">
                      {repo.description}
                    </p>
                  )}
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
                      {repo.stars.toLocaleString()}
                    </span>
                    <span className="flex items-center gap-1">
                      <GitFork className="h-3 w-3" />
                      {repo.forks.toLocaleString()}
                    </span>
                    {repo.updatedAt && (
                      <span>Updated {new Date(repo.updatedAt).toLocaleDateString()}</span>
                    )}
                  </div>
                </div>
                <div
                  className="flex shrink-0 items-center gap-1.5 opacity-0 transition-opacity group-hover:opacity-100"
                  onClick={(e) => e.stopPropagation()}
                >
                  <Button
                    variant="outline"
                    size="sm"
                    className="h-8"
                    disabled={pending}
                    onClick={(e) => handleStar(e, repo)}
                    title={`Star ${repo.fullName}`}
                  >
                    {pending ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Star className="h-3.5 w-3.5" />
                    )}
                    Star
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="h-8"
                    disabled={pending}
                    onClick={(e) => handleFork(e, repo)}
                    title={`Fork ${repo.fullName}`}
                  >
                    {pending ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <GitFork className="h-3.5 w-3.5" />
                    )}
                    Fork
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8"
                    onClick={(e) => handleOpen(e, repo)}
                    title="Open in browser"
                  >
                    <ExternalLink className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
