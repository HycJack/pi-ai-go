import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { API, StarredRepo, StarGroup, CachedRepoResponse, formatRelativeTime } from '../lib/api';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Select } from '../components/ui/select';
import { Badge } from '../components/ui/badge';
import { ScrollArea } from '../components/ui/scroll-area';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../components/ui/dialog';
import { Label } from '../components/ui/label';
import { Star, RefreshCw, Loader2, FolderPlus, X, Pencil } from 'lucide-react';
import { cn } from '@/lib/utils';

export interface StarredFilters {
  keyword: string;
  language: string;
  sort: 'starred' | 'name' | 'stars' | 'updated';
  groupID: string;
}

interface StarredReposPageProps {
  addToast: (message: string, type?: string) => void;
  onSelectRepo: (repo: StarredRepo) => void;
  starGroups: StarGroup[];
  onGroupsChange: () => void;
  filters: StarredFilters;
  onFiltersChange: (f: StarredFilters) => void;
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

export default function StarredReposPage({
  addToast,
  onSelectRepo,
  starGroups,
  onGroupsChange,
  filters,
  onFiltersChange,
}: StarredReposPageProps) {
  const [repos, setRepos] = useState<StarredRepo[]>([]);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [cachedAt, setCachedAt] = useState(0);
  const { keyword, language: languageFilter, sort, groupID: activeGroupID } = filters;
  const setKeyword = (v: string) => onFiltersChange({ ...filters, keyword: v });
  const setLanguageFilter = (v: string) => onFiltersChange({ ...filters, language: v });
  const setSort = (v: StarredFilters['sort']) => onFiltersChange({ ...filters, sort: v });
  const setActiveGroupID = (v: string) => onFiltersChange({ ...filters, groupID: v });
  const [showNewGroupInput, setShowNewGroupInput] = useState(false);
  const [newGroupName, setNewGroupName] = useState('');
  const [renameTarget, setRenameTarget] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState('');
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

  useEffect(() => {
    loadRepos();
  }, [loadRepos]);

  const languages = useMemo(() => {
    const langs = new Set(repos.map((r) => r.language).filter(Boolean));
    return Array.from(langs).sort();
  }, [repos]);

  const filtered = useMemo(() => {
    const result = repos.filter((r) => {
      if (keyword) {
        const k = keyword.toLowerCase();
        if (
          !r.fullName.toLowerCase().includes(k) &&
          !(r.description || '').toLowerCase().includes(k)
        ) {
          return false;
        }
      }
      if (languageFilter && r.language !== languageFilter) return false;
      if (activeGroupID && !r.groups.includes(activeGroupID)) return false;
      return true;
    });

    if (sort === 'name') {
      result.sort((a, b) => a.fullName.localeCompare(b.fullName));
    } else if (sort === 'stars') {
      result.sort((a, b) => (b.stars || 0) - (a.stars || 0));
    } else if (sort === 'updated') {
      result.sort(
        (a, b) => new Date(b.updatedAt || 0).getTime() - new Date(a.updatedAt || 0).getTime()
      );
    }

    return result;
  }, [repos, keyword, languageFilter, activeGroupID, sort]);

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
      setRepos((prev) =>
        prev.map((r) => {
          if (r.fullName !== repoFullName) return r;
          const set = new Set(r.groups);
          if (currentlyIn) set.delete(groupID);
          else set.add(groupID);
          return { ...r, groups: Array.from(set) };
        })
      );
      onGroupsChange();
    } catch (e: any) {
      addToast('Update group failed: ' + (e?.message || 'error'), 'error');
    }
  };

  const handleUnstar = async (repoFullName: string) => {
    if (!confirm(`Unstar ${repoFullName}?`)) return;
    try {
      await API.UnstarRepo(repoFullName);
      setRepos((prev) => prev.filter((r) => r.fullName !== repoFullName));
      addToast(`Unstarred ${repoFullName}`, 'info');
    } catch (e: any) {
      addToast('Unstar failed: ' + (e?.message || 'error'), 'error');
    }
  };

  return (
    <div className="animate-fade-in">
      {/* Filter Bar */}
      <div className="mb-4 flex flex-wrap items-center gap-2 rounded-md border border-border bg-background p-2">
        <Input
          className="h-8 w-[200px] text-xs"
          placeholder="Filter by name or description..."
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
        />
        <div className="h-5 w-px bg-border" />
        <Select
          className="h-8 w-auto text-xs"
          value={languageFilter}
          onChange={(e) => setLanguageFilter(e.target.value)}
        >
          <option value="">All Languages</option>
          {languages.map((lang) => (
            <option key={lang} value={lang}>
              {lang}
            </option>
          ))}
        </Select>
        <Select
          className="h-8 w-auto text-xs"
          value={sort}
          onChange={(e) => setSort(e.target.value as any)}
        >
          <option value="starred">Recently starred</option>
          <option value="updated">Recently updated</option>
          <option value="stars">Most stars</option>
          <option value="name">Name</option>
        </Select>
        <div className="h-5 w-px bg-border" />
        <span className="text-xs font-medium text-muted-foreground">Group:</span>
        <Button
          variant={activeGroupID === '' ? 'default' : 'ghost'}
          size="sm"
          onClick={() => setActiveGroupID('')}
        >
          All
        </Button>
        {starGroups.map((g) => (
          <Button
            key={g.id}
            variant={activeGroupID === g.id ? 'default' : 'ghost'}
            size="sm"
            onClick={() => setActiveGroupID(g.id)}
            title={`${g.repos.length} repos`}
          >
            {g.name}
          </Button>
        ))}
        <div className="flex-1" />
        <span className="text-xs text-muted-foreground">
          {filtered.length}/{repos.length} repos
          {cachedAt > 0 && (
            <span className="ml-2">· cached {new Date(cachedAt * 1000).toLocaleString()}</span>
          )}
          {syncing && <span className="ml-2 text-primary">· syncing…</span>}
        </span>
        <Button variant="ghost" size="sm" onClick={() => setShowNewGroupInput(true)}>
          <FolderPlus className="h-3.5 w-3.5" />
          New Group
        </Button>
        <Button variant="ghost" size="sm" onClick={handleForceSync} disabled={syncing}>
          {syncing ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
          ) : (
            <RefreshCw className="h-3.5 w-3.5" />
          )}
          {syncing ? 'Syncing…' : 'Sync'}
        </Button>
        <Button variant="ghost" size="sm" onClick={loadRepos}>
          <RefreshCw className="h-3.5 w-3.5" />
          Refresh
        </Button>
      </div>

      {/* New Group Dialog */}
      {showNewGroupInput && (
        <Dialog open onOpenChange={(open) => !open && setShowNewGroupInput(false)}>
          <DialogContent className="max-w-[420px]">
            <DialogHeader>
              <DialogTitle>Create Star Group</DialogTitle>
            </DialogHeader>
            <div className="space-y-2">
              <Label>Group Name</Label>
              <Input
                placeholder="e.g. Frontend frameworks"
                value={newGroupName}
                onChange={(e) => setNewGroupName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') handleCreateGroup();
                  if (e.key === 'Escape') setShowNewGroupInput(false);
                }}
                autoFocus
              />
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setShowNewGroupInput(false)}>
                Cancel
              </Button>
              <Button onClick={handleCreateGroup} disabled={!newGroupName.trim()}>
                Create Group
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}

      {/* Group Management */}
      {starGroups.length > 0 && (
        <div className="mb-3 flex flex-wrap gap-1.5 text-xs text-muted-foreground">
          {starGroups.map((g) => (
            <div
              key={g.id}
              className="flex items-center gap-1.5 rounded-md border border-border px-2 py-0.5"
            >
              {renameTarget === g.id ? (
                <div className="flex items-center gap-1">
                  <Input
                    className="h-5 w-[120px] text-xs"
                    value={renameValue}
                    onChange={(e) => setRenameValue(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') handleRenameGroup(g.id);
                      if (e.key === 'Escape') setRenameTarget(null);
                    }}
                    autoFocus
                  />
                  <Button variant="ghost" size="sm" className="h-6 px-2 text-xs" onClick={() => handleRenameGroup(g.id)}>
                    OK
                  </Button>
                  <Button variant="ghost" size="sm" className="h-6 px-2 text-xs" onClick={() => setRenameTarget(null)}>
                    ×
                  </Button>
                </div>
              ) : (
                <>
                  <span>
                    <strong>{g.name}</strong> · {g.repos.length}
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 w-6 p-0 text-xs"
                    onClick={() => {
                      setRenameTarget(g.id);
                      setRenameValue(g.name);
                    }}
                    title="Rename"
                  >
                    <Pencil className="h-3 w-3" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 w-6 p-0 text-xs text-destructive"
                    onClick={() => handleDeleteGroup(g.id, g.name)}
                    title="Delete"
                  >
                    <X className="h-3 w-3" />
                  </Button>
                </>
              )}
            </div>
          ))}
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-primary" />
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
          <Star className="mb-3 h-12 w-12 opacity-40" />
          <h3 className="mb-1 text-base font-semibold text-secondary-foreground">
            No starred repositories
          </h3>
          <p className="text-sm">
            {activeGroupID
              ? 'This group has no repos yet.'
              : 'Star repos on GitHub to see them here.'}
          </p>
        </div>
      ) : (
        <div className="overflow-hidden rounded-md border border-border">
          {filtered.map((repo) => (
            <div
              key={repo.fullName}
              className="group relative flex cursor-pointer gap-3 border-b border-border px-4 py-3 transition-colors hover:bg-muted/50 last:border-b-0"
              onClick={() => onSelectRepo(repo)}
            >
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-[15px] font-semibold text-primary">{repo.fullName}</span>
                  {repo.groups.length > 0 && (
                    <span className="text-xs text-primary">
                      {repo.groups
                        .map((gid) => starGroups.find((g) => g.id === gid)?.name)
                        .filter(Boolean)
                        .join(', ')}
                    </span>
                  )}
                </div>
                {repo.description && (
                  <div className="mt-1 line-clamp-2 text-sm text-muted-foreground">
                    {repo.description}
                  </div>
                )}
                <div className="mt-1.5 flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
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
                  <span>{repo.forks.toLocaleString()} forks</span>
                  <span>Updated {formatRelativeTime(repo.updatedAt)}</span>
                </div>
              </div>
              {/* Group action menu */}
              <div
                className="flex items-start gap-1 opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
                onClick={(e) => e.stopPropagation()}
              >
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-6 text-xs"
                  onClick={() => setGroupMenuFor(groupMenuFor === repo.fullName ? null : repo.fullName)}
                >
                  Group ▾
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-6 text-xs text-destructive"
                  onClick={() => handleUnstar(repo.fullName)}
                >
                  Unstar
                </Button>
              </div>
              {groupMenuFor === repo.fullName && (
                <div
                  className="absolute right-2 top-9 z-10 min-w-[180px] rounded-md border border-border bg-background p-1.5 shadow-lg cursor-default"
                  onClick={(e) => e.stopPropagation()}
                >
                  {starGroups.length === 0 ? (
                    <div className="px-3 py-2 text-xs text-muted-foreground">
                      No groups yet. Create one first.
                    </div>
                  ) : (
                    starGroups.map((g) => {
                      const inGroup = repo.groups.includes(g.id);
                      return (
                        <label
                          key={g.id}
                          className="flex cursor-pointer items-center gap-2 rounded-sm px-2 py-1 text-xs hover:bg-muted"
                        >
                          <input
                            type="checkbox"
                            checked={inGroup}
                            onChange={() => handleToggleRepoInGroup(g.id, repo.fullName, inGroup)}
                            className="h-3.5 w-3.5"
                          />
                          <span className="flex-1">{g.name}</span>
                          <span className="text-xs text-muted-foreground">({g.repos.length})</span>
                        </label>
                      );
                    })
                  )}
                  <div className="mt-1 border-t border-border pt-1">
                    <Button
                      variant="ghost"
                      size="sm"
                      className="w-full justify-start text-xs"
                      onClick={() => {
                        setGroupMenuFor(null);
                        setShowNewGroupInput(true);
                      }}
                    >
                      + New group
                    </Button>
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
