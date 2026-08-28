import React, { useState, useEffect, useCallback } from 'react';
import { API, Issue, formatRelativeTime } from '../lib/api';
import { Button } from '../components/ui/button';
import { Select } from '../components/ui/select';
import { Badge } from '../components/ui/badge';
import { CircleDot, RefreshCw } from 'lucide-react';
import { cn } from '@/lib/utils';

interface IssuesPageProps {
  onSelect: (item: any) => void;
  addToast: (message: string, type?: string) => void;
  activeRepo?: string;
}

export default function IssuesPage({ onSelect, addToast, activeRepo }: IssuesPageProps) {
  const [issues, setIssues] = useState<Issue[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<'open' | 'closed' | 'all'>('open');
  const [sort, setSort] = useState<'updated' | 'created'>('updated');

  const loadIssues = useCallback(async () => {
    setLoading(true);
    try {
      const str = await API.GetIssues(filter, sort, activeRepo || '');
      setIssues(JSON.parse(str));
    } catch (e) {
      addToast('Failed to load issues', 'error');
      setIssues([]);
    } finally {
      setLoading(false);
    }
  }, [filter, sort, activeRepo, addToast]);

  useEffect(() => {
    loadIssues();
  }, [loadIssues]);

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
        <Button variant="ghost" size="sm" onClick={loadIssues}>
          <RefreshCw className="h-3.5 w-3.5" />
          Refresh
        </Button>
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-border border-t-primary" />
        </div>
      ) : issues.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
          <CircleDot className="mb-3 h-12 w-12 opacity-40" />
          <h3 className="mb-1 text-base font-semibold text-secondary-foreground">
            No issues found
          </h3>
          <p className="text-sm">No {filter} issues match your criteria.</p>
        </div>
      ) : (
        <div className="overflow-hidden rounded-md border border-border">
          {issues.map((issue) => (
            <div
              key={issue.id}
              onClick={() => onSelect({ ...issue, _type: 'issue' })}
              className="flex cursor-pointer items-start gap-3 border-b border-border px-4 py-3 transition-colors hover:bg-muted/50 last:border-b-0"
            >
              <CircleDot
                className={cn(
                  'mt-0.5 h-4 w-4 shrink-0',
                  issue.state === 'open' ? 'text-success' : 'text-destructive'
                )}
              />
              <div className="flex h-8 w-8 shrink-0 overflow-hidden rounded-full bg-border">
                <img src={issue.avatarUrl} alt="" className="h-full w-full object-cover" />
              </div>
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium truncate">{issue.title}</div>
                <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                  <span>#{issue.number}</span>
                  <span>by {issue.user}</span>
                  <span className="font-medium text-primary">{issue.repo}</span>
                  <span>{formatRelativeTime(issue.createdAt)}</span>
                  {issue.comments > 0 && <span>💬 {issue.comments}</span>}
                </div>
              </div>
              {issue.labels.length > 0 && (
                <div className="flex max-w-[200px] flex-wrap gap-1">
                  {issue.labels.slice(0, 3).map((l, i) => (
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
