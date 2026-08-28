import React, { useState, useCallback, useEffect, useRef } from 'react';
import { API, WorkflowRun, Job, Repo, CachedRepoResponse, formatRelativeTime } from '../lib/api';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Badge } from '../components/ui/badge';
import { ScrollArea } from '../components/ui/scroll-area';
import { PlayCircle, ChevronDown, FolderGit2, Loader2, FileText } from 'lucide-react';
import { cn } from '@/lib/utils';

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
    return () => {
      cancelled = true;
    };
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
    ? myRepos.filter(
        (r) =>
          r.fullName.toLowerCase().includes(repoSearch.toLowerCase()) ||
          (r.description && r.description.toLowerCase().includes(repoSearch.toLowerCase()))
      )
    : myRepos.slice(0, 100);

  const getStatusBadge = (status: string, conclusion: string) => {
    if (status === 'completed') {
      const variant = conclusion === 'success' ? 'success' : conclusion === 'failure' ? 'destructive' : 'secondary';
      return <Badge variant={variant} className="text-xs">{conclusion}</Badge>;
    }
    return <Badge variant="default" className="text-xs animate-pulse">{status}</Badge>;
  };

  return (
    <div className="animate-fade-in flex gap-4" style={{ height: '100%' }}>
      {/* Left Panel - Runs */}
      <div className="flex-1 min-w-0">
        <div className="mb-4 flex flex-wrap items-center gap-2 rounded-md border border-border bg-background p-2">
          <span className="text-xs font-medium text-muted-foreground">Repository:</span>

          <div ref={dropdownRef} className="relative">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setRepoDropdownOpen((v) => !v)}
              title="Choose from your repos"
            >
              <FolderGit2 className="h-3.5 w-3.5" />
              {reposLoading ? 'Loading...' : 'My Repos'}
              <ChevronDown className="h-3 w-3 opacity-60" />
            </Button>

            {repoDropdownOpen && (
              <div className="absolute top-full left-0 z-20 mt-1 flex max-h-[400px] min-w-[320px] max-w-[420px] flex-col rounded-md border border-border bg-background shadow-lg">
                <div className="sticky top-0 border-b border-border bg-background p-2">
                  <Input
                    className="h-7 text-xs"
                    placeholder="Search your repositories..."
                    value={repoSearch}
                    onChange={(e) => setRepoSearch(e.target.value)}
                    autoFocus
                  />
                </div>
                <ScrollArea className="flex-1">
                  {reposLoading ? (
                    <div className="p-4 text-center text-xs text-muted-foreground">Loading repos...</div>
                  ) : filteredRepos.length === 0 ? (
                    <div className="p-4 text-center text-xs text-muted-foreground">
                      {repoSearch ? 'No matching repos' : 'No cached repos. Sync repositories first.'}
                    </div>
                  ) : (
                    filteredRepos.map((r) => (
                      <div
                        key={r.fullName}
                        onClick={() => handlePickRepo(r.fullName)}
                        className={cn(
                          'cursor-pointer border-b border-border px-3 py-2 text-xs transition-colors hover:bg-muted',
                          repoInput === r.fullName && 'bg-muted'
                        )}
                      >
                        <div className="flex items-center gap-1.5">
                          <span className="font-semibold">{r.fullName}</span>
                          {r.private && (
                            <Badge variant="secondary" className="h-4 text-xs px-1">Private</Badge>
                          )}
                          {r.language && (
                            <Badge variant="outline" className="h-4 text-xs px-1">{r.language}</Badge>
                          )}
                        </div>
                        {r.description && (
                          <div className="mt-0.5 truncate text-xs text-muted-foreground">
                            {r.description}
                          </div>
                        )}
                      </div>
                    ))
                  )}
                </ScrollArea>
              </div>
            )}
          </div>

          <Input
            className="w-[280px] text-sm"
            value={repoInput}
            onChange={(e) => setRepoInput(e.target.value)}
            placeholder="owner/repo"
          />
          <Button size="sm" onClick={loadRuns}>
            Load
          </Button>
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-6 w-6 animate-spin text-primary" />
          </div>
        ) : runs.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
            <PlayCircle className="mb-3 h-12 w-12 opacity-40" />
            <h3 className="mb-1 text-base font-semibold text-secondary-foreground">
              No workflow runs
            </h3>
            <p className="text-sm">
              Choose a repository from "My Repos" or enter owner/repo manually, then click Load.
            </p>
          </div>
        ) : (
          <div className="space-y-2 py-2">
            {runs.map((run) => (
              <div
                key={run.id}
                onClick={() => handleSelectRun(run)}
                className={cn(
                  'cursor-pointer rounded-md border border-border bg-card p-3 transition-colors hover:border-primary/50',
                  selectedRun?.id === run.id && 'border-primary'
                )}
              >
                <div className="mb-1 flex items-center justify-between">
                  <span className="text-sm font-medium">{run.name}</span>
                  {getStatusBadge(run.status, run.conclusion)}
                </div>
                <div className="flex flex-wrap gap-x-3 text-xs text-muted-foreground">
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

      {/* Right Panel - Jobs/Logs */}
      {(selectedRun || jobs.length > 0) && (
        <div className="w-[400px] min-w-[400px] overflow-auto border-l border-border p-4">
          {selectedRun && (
            <div className="mb-4">
              <h3 className="mb-1 text-sm font-semibold">{selectedRun.name}</h3>
              <div className="flex gap-3 text-xs text-muted-foreground">
                <span>{selectedRun.headBranch}</span>
                <span>{selectedRun.event}</span>
              </div>
            </div>
          )}

          {loadingJobs ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-5 w-5 animate-spin text-primary" />
            </div>
          ) : jobs.length > 0 ? (
            <div>
              <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Jobs
              </h3>
              {jobs.map((job) => (
                <div
                  key={job.id}
                  className="mb-3 rounded-md bg-muted/50 p-3"
                >
                  <div className="mb-2 flex items-center justify-between">
                    <span className="text-sm font-medium">{job.name}</span>
                    {getStatusBadge(job.status, job.conclusion || '')}
                  </div>
                  {job.steps.map((step, i) => (
                    <div key={i} className="flex items-center gap-2 py-1 text-sm">
                      <div
                        className={cn(
                          'h-2 w-2 rounded-full shrink-0',
                          step.conclusion === 'success'
                            ? 'bg-success'
                            : step.conclusion === 'failure'
                              ? 'bg-destructive'
                              : step.status === 'in_progress'
                                ? 'bg-primary animate-pulse'
                                : 'bg-muted-foreground'
                        )}
                      />
                      <span className="flex-1">{step.name}</span>
                    </div>
                  ))}
                  <Button
                    variant="ghost"
                    size="sm"
                    className="mt-1 text-xs"
                    onClick={() => handleViewLogs(job.id)}
                  >
                    <FileText className="h-3 w-3" />
                    View logs
                  </Button>
                </div>
              ))}
            </div>
          ) : null}

          {logs && (
            <div className="mt-4 rounded-md border border-border overflow-hidden">
              <div className="bg-muted px-3 py-2 text-sm font-medium border-b border-border">
                Logs
              </div>
              <div className="max-h-[400px] overflow-auto whitespace-pre-wrap p-3 font-mono text-xs">
                {logs}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
