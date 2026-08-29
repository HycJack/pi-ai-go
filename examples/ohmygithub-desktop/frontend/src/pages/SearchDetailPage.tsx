import React, { useState, useEffect, useCallback } from 'react';
import { API, Repo } from '../lib/api';
import { Button } from '../components/ui/button';
import { ScrollArea } from '../components/ui/scroll-area';
import {
  Star,
  GitFork,
  ExternalLink,
  Loader2,
  CircleDot,
  GitBranch,
  Calendar,
  Eye,
  RefreshCw,
} from 'lucide-react';

interface SearchDetailPageProps {
  repoFullName: string;
  addToast: (message: string, type?: string) => void;
  onOpenExternal: (url: string) => void;
}

interface RepoDetail extends Repo {
  defaultBranch?: string;
  size?: number;
  watchers?: number;
  topics?: string[];
  homepage?: string;
  license?: string;
  createdAt?: string;
  pushedAt?: string;
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

export default function SearchDetailPage({ repoFullName, addToast, onOpenExternal }: SearchDetailPageProps) {
  const [repo, setRepo] = useState<RepoDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState<'star' | 'unstar' | 'fork' | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const str = await API.GetRepo(repoFullName);
      const data: RepoDetail = JSON.parse(str);
      setRepo(data);
    } catch (e: any) {
      const msg = e?.message || e?.error || (typeof e === 'string' ? e : 'unknown error');
      addToast('Failed to load repo: ' + msg, 'error');
      setRepo(null);
    } finally {
      setLoading(false);
    }
  }, [repoFullName, addToast]);

  useEffect(() => {
    load();
  }, [load]);

  const handleStar = useCallback(async () => {
    if (!repo) return;
    setActing('star');
    try {
      await API.StarRepo(repo.fullName);
      addToast(`Starred ${repo.fullName}`, 'success');
    } catch (e: any) {
      const msg = e?.message || e?.error || 'unknown error';
      addToast('Star failed: ' + msg, 'error');
    } finally {
      setActing(null);
    }
  }, [repo, addToast]);

  const handleUnstar = useCallback(async () => {
    if (!repo) return;
    setActing('unstar');
    try {
      await API.UnstarRepo(repo.fullName);
      addToast(`Unstarred ${repo.fullName}`, 'success');
    } catch (e: any) {
      const msg = e?.message || e?.error || 'unknown error';
      addToast('Unstar failed: ' + msg, 'error');
    } finally {
      setActing(null);
    }
  }, [repo, addToast]);

  const handleFork = useCallback(async () => {
    if (!repo) return;
    if (!confirm(`Fork ${repo.fullName} to your account?`)) return;
    setActing('fork');
    try {
      const str = await API.ForkRepo(repo.fullName);
      const out = JSON.parse(str);
      addToast(`Forked to ${out.fullName}`, 'success');
      if (out.htmlUrl) onOpenExternal(out.htmlUrl);
    } catch (e: any) {
      const msg = e?.message || e?.error || 'unknown error';
      addToast('Fork failed: ' + msg, 'error');
    } finally {
      setActing(null);
    }
  }, [repo, addToast, onOpenExternal]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <Loader2 className="h-6 w-6 animate-spin text-primary" />
      </div>
    );
  }

  if (!repo) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
        <CircleDot className="mb-3 h-10 w-10 opacity-40" />
        <h3 className="mb-1 text-base font-semibold text-secondary-foreground">
          Repository not found
        </h3>
        <p className="text-sm">Could not load {repoFullName}</p>
      </div>
    );
  }

  return (
    <div className="animate-fade-in pb-8 flex flex-col gap-3">
      {/* Header card */}
      <div className="flex flex-col gap-3 bg-card border border-border rounded-md px-4 py-4">
        <div className="flex items-start gap-3 flex-wrap">
          <div className="flex-1 min-w-[240px]">
            <div className="flex items-center gap-2 flex-wrap">
              <h2 className="text-base font-semibold text-foreground">{repo.fullName}</h2>
              {repo.private ? (
                <span className="rounded bg-secondary px-2 py-0.5 text-xs font-medium text-secondary-foreground">
                  Private
                </span>
              ) : (
                <span className="rounded bg-secondary px-2 py-0.5 text-xs font-medium text-secondary-foreground">
                  Public
                </span>
              )}
            </div>
            {repo.description && (
              <p className="mt-1.5 text-sm text-secondary-foreground">{repo.description}</p>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2 shrink-0">
            <Button
              variant="default"
              size="sm"
              onClick={handleStar}
              disabled={acting !== null}
              title={`Star ${repo.fullName}`}
            >
              {acting === 'star' ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <Star className="h-3.5 w-3.5" />
              )}
              Star
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleUnstar}
              disabled={acting !== null}
              title={`Unstar ${repo.fullName}`}
            >
              {acting === 'unstar' ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <Star className="h-3.5 w-3.5" />
              )}
              Unstar
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleFork}
              disabled={acting !== null}
              title={`Fork ${repo.fullName}`}
            >
              {acting === 'fork' ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <GitFork className="h-3.5 w-3.5" />
              )}
              Fork
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={load}
              disabled={loading}
              title="Refresh"
            >
              <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
            </Button>
            {repo.htmlUrl && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => onOpenExternal(repo.htmlUrl!)}
                title="Open in browser"
              >
                <ExternalLink className="h-3.5 w-3.5" />
                Open
              </Button>
            )}
          </div>
        </div>

        {/* Stats */}
        <div className="flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
          {repo.language && (
            <span className="flex items-center gap-1.5">
              <span
                className="h-2.5 w-2.5 rounded-full"
                style={{ backgroundColor: languageColors[repo.language] || '#6b7280' }}
              />
              <span className="font-medium text-foreground">{repo.language}</span>
            </span>
          )}
          <span className="flex items-center gap-1">
            <Star className="h-3 w-3" />
            <strong className="text-foreground">{repo.stars.toLocaleString()}</strong> stars
          </span>
          <span className="flex items-center gap-1">
            <GitFork className="h-3 w-3" />
            <strong className="text-foreground">{repo.forks.toLocaleString()}</strong> forks
          </span>
          <span className="flex items-center gap-1">
            <CircleDot className="h-3 w-3" />
            <strong className="text-foreground">{repo.openIssues.toLocaleString()}</strong> issues
          </span>
          {repo.watchers !== undefined && (
            <span className="flex items-center gap-1">
              <Eye className="h-3 w-3" />
              <strong className="text-foreground">{repo.watchers.toLocaleString()}</strong> watchers
            </span>
          )}
          {repo.defaultBranch && (
            <span className="flex items-center gap-1">
              <GitBranch className="h-3 w-3" />
              <strong className="text-foreground">{repo.defaultBranch}</strong>
            </span>
          )}
          {repo.license && (
            <span className="flex items-center gap-1">
              <strong className="text-foreground">{repo.license}</strong>
            </span>
          )}
          {repo.updatedAt && (
            <span className="flex items-center gap-1">
              <Calendar className="h-3 w-3" />
              Updated {new Date(repo.updatedAt).toLocaleDateString()}
            </span>
          )}
        </div>

        {repo.topics && repo.topics.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {repo.topics.map((t) => (
              <span
                key={t}
                className="rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary"
              >
                {t}
              </span>
            ))}
          </div>
        )}

        {repo.homepage && (
          <button
            className="text-xs text-primary hover:underline self-start"
            onClick={() => onOpenExternal(repo.homepage!)}
          >
            {repo.homepage}
          </button>
        )}
      </div>

      {/* Empty body — detail actions all in header card */}
      <ScrollArea className="rounded-md border border-border bg-card p-4 text-sm text-muted-foreground">
        Use the buttons above to Star, Unstar, or Fork this repository on GitHub. Open it in
        your browser for the full README, issues, and pull requests.
      </ScrollArea>
    </div>
  );
}
