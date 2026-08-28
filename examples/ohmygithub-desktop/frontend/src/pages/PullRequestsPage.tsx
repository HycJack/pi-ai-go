import React, { useState, useEffect, useCallback } from 'react';
import { API, PullRequest, formatRelativeTime } from '../lib/api';
import { Button } from '../components/ui/button';
import { Select } from '../components/ui/select';
import { Badge } from '../components/ui/badge';
import { GitPullRequest, RefreshCw } from 'lucide-react';
import { cn } from '@/lib/utils';

interface PullRequestsPageProps {
  onSelect: (item: any) => void;
  addToast: (message: string, type?: string) => void;
  activeRepo?: string;
}

export default function PullRequestsPage({ onSelect, addToast, activeRepo }: PullRequestsPageProps) {
  const [prs, setPrs] = useState<PullRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<'open' | 'closed' | 'all'>('open');
  const [sort, setSort] = useState<'updated' | 'created'>('updated');

  const loadPRs = useCallback(async () => {
    setLoading(true);
    try {
      const str = await API.GetPullRequests(filter, sort, activeRepo || '');
      setPrs(JSON.parse(str));
    } catch (e) {
      addToast('Failed to load pull requests', 'error');
      setPrs([]);
    } finally {
      setLoading(false);
    }
  }, [filter, sort, activeRepo, addToast]);

  useEffect(() => {
    loadPRs();
  }, [loadPRs]);

  const handleClick = async (pr: PullRequest) => {
    try {
      const diffStr = await API.GetPRDiff(pr.repo, pr.number);
      onSelect({ ...pr, _type: 'pr', _diff: diffStr });
    } catch {
      onSelect({ ...pr, _type: 'pr' });
    }
  };

  return (
    <div className="animate-fade-in">
      {/* Filter Bar */}
      <div className="mb-4 flex flex-wrap items-center gap-2 rounded-md border border-border bg-background p-2">
        <span className="text-xs font-medium text-muted-foreground">State:</span>
        <Button
          variant={filter === 'open' ? 'default' : 'ghost'}
          size="sm"
          onClick={() => setFilter('open')}
        >
          Open
        </Button>
        <Button
          variant={filter === 'closed' ? 'default' : 'ghost'}
          size="sm"
          onClick={() => setFilter('closed')}
        >
          Closed
        </Button>
        <Button
          variant={filter === 'all' ? 'default' : 'ghost'}
          size="sm"
          onClick={() => setFilter('all')}
        >
          All
        </Button>
        <div className="h-5 w-px bg-border" />
        <span className="text-xs font-medium text-muted-foreground">Sort:</span>
        <Select
          className="w-auto text-xs"
          value={sort}
          onChange={(e) => setSort(e.target.value as any)}
        >
          <option value="updated">Recently updated</option>
          <option value="created">Recently created</option>
        </Select>
        <div className="flex-1" />
        {activeRepo && (
          <span className="text-xs font-medium text-primary">Scope: {activeRepo}</span>
        )}
        <Button variant="ghost" size="sm" onClick={loadPRs}>
          <RefreshCw className="h-3.5 w-3.5" />
          Refresh
        </Button>
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-border border-t-primary" />
        </div>
      ) : prs.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
          <GitPullRequest className="mb-3 h-12 w-12 opacity-40" />
          <h3 className="mb-1 text-base font-semibold text-secondary-foreground">
            No pull requests found
          </h3>
          <p className="text-sm">No {filter} PRs match your criteria.</p>
        </div>
      ) : (
        <div className="overflow-hidden rounded-md border border-border">
          {prs.map((pr) => (
            <div
              key={pr.id}
              onClick={() => handleClick(pr)}
              className="flex cursor-pointer items-start gap-3 border-b border-border px-4 py-3 transition-colors hover:bg-muted/50 last:border-b-0"
            >
              <GitPullRequest
                className={cn(
                  'mt-0.5 h-4 w-4 shrink-0',
                  pr.draft
                    ? 'text-muted-foreground'
                    : pr.state === 'open'
                      ? 'text-success'
                      : 'text-destructive'
                )}
              />
              <div className="flex h-8 w-8 shrink-0 overflow-hidden rounded-full bg-border">
                <img src={pr.avatarUrl} alt="" className="h-full w-full object-cover" />
              </div>
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium truncate">{pr.title}</div>
                <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                  <span>#{pr.number}</span>
                  <span>by {pr.user}</span>
                  <span className="font-medium text-primary">{pr.repo}</span>
                  <span>{formatRelativeTime(pr.updatedAt)}</span>
                  {pr.draft && <Badge variant="secondary" className="text-xs">Draft</Badge>}
                </div>
              </div>
              {pr.labels.length > 0 && (
                <div className="flex max-w-[200px] flex-wrap gap-1">
                  {pr.labels.slice(0, 3).map((l, i) => (
                    <Badge
                      key={i}
                      variant="outline"
                      className="text-xs"
                      style={{
                        background: `#${l.color}22`,
                        borderColor: `#${l.color}44`,
                        color: `#${l.color}`,
                      }}
                    >
                      {l.name}
                    </Badge>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
