// Wails bindings helper
import { GetSettings, UpdateSettings, AddAccount, RemoveAccount, SwitchAccount } from '../../wailsjs/go/main/App';
import { AddBookmark, RemoveBookmark, ReorderBookmarks } from '../../wailsjs/go/main/App';
import { GetNotifications, MarkNotificationRead, MarkAllNotificationsRead } from '../../wailsjs/go/main/App';
import { GetPullRequests, GetPRDiff, GetPRFiles } from '../../wailsjs/go/main/App';
import { GetIssues } from '../../wailsjs/go/main/App';
import { GetWorkflowRuns, GetWorkflowRunJobs, GetWorkflowLogs } from '../../wailsjs/go/main/App';
import { GetMyRepos, SearchRepos } from '../../wailsjs/go/main/App';
import { GetRepoContents } from '../../wailsjs/go/main/App';
import { OpenExternal, ShowMessage } from '../../wailsjs/go/main/App';

export const API = {
  GetSettings,
  UpdateSettings,
  AddAccount,
  RemoveAccount,
  SwitchAccount,
  AddBookmark,
  RemoveBookmark,
  ReorderBookmarks,
  GetNotifications,
  MarkNotificationRead,
  MarkAllNotificationsRead,
  GetPullRequests,
  GetPRDiff,
  GetPRFiles,
  GetIssues,
  GetWorkflowRuns,
  GetWorkflowRunJobs,
  GetWorkflowLogs,
  GetMyRepos,
  SearchRepos,
  GetRepoContents,
  OpenExternal,
  ShowMessage,
};

// Types
export interface Notification {
  id: string;
  title: string;
  repo: string;
  type: string;
  state: string;
  url: string;
  updatedAt: string;
  read: boolean;
}

export interface PullRequest {
  id: number;
  number: number;
  title: string;
  repo: string;
  state: string;
  user: string;
  avatarUrl: string;
  createdAt: string;
  updatedAt: string;
  draft: boolean;
  labels: Label[];
  mergeable: string;
  reviewStatus: string;
}

export interface Issue {
  id: number;
  number: number;
  title: string;
  repo: string;
  state: string;
  user: string;
  avatarUrl: string;
  createdAt: string;
  labels: Label[];
  comments: number;
  body: string;
}

export interface Label {
  name: string;
  color: string;
}

export interface Repo {
  name: string;
  owner: string;
  fullName: string;
  description: string;
  language: string;
  stars: number;
  forks: number;
  openIssues: number;
  private: boolean;
  updatedAt: string;
}

export interface SearchResult {
  totalCount: number;
  items: Repo[];
}

export interface WorkflowRun {
  id: number;
  name: string;
  headBranch: string;
  event: string;
  status: string;
  conclusion: string;
  createdAt: string;
  updatedAt: string;
  actor: string;
  htmlUrl: string;
  jobs?: Job[];
}

export interface Job {
  id: number;
  name: string;
  status: string;
  conclusion: string;
  startedAt: string;
  completedAt: string;
  steps: JobStep[];
}

export interface JobStep {
  name: string;
  status: string;
  conclusion: string;
  number: number;
}

export interface FileContent {
  name: string;
  path: string;
  type: string;
  content?: string;
  size: number;
  htmlUrl: string;
  encoding?: string;
}

export interface DiffContent {
  filename: string;
  status: string;
  additions: number;
  deletions: number;
  patch: string;
  content?: string;
}

export interface AppSettings {
  accounts: GitHubAccount[];
  activeAccount: number;
  theme: string;
  fontSize: number;
  codeFont: string;
  bookmarks: Bookmark[];
  windowWidth: number;
  windowHeight: number;
}

export interface GitHubAccount {
  token: string;
  username: string;
  avatarUrl: string;
}

export interface Bookmark {
  id: string;
  title: string;
  url: string;
  icon: string;
  order: number;
}

export function formatRelativeTime(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const diff = now.getTime() - date.getTime();
  const seconds = Math.floor(diff / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (seconds < 60) return 'just now';
  if (minutes < 60) return `${minutes}m ago`;
  if (hours < 24) return `${hours}h ago`;
  if (days < 7) return `${days}d ago`;
  if (days < 30) return `${Math.floor(days / 7)}w ago`;
  return date.toLocaleDateString();
}

export function getStateIcon(type: string): string {
  switch (type) {
    case 'issue': return 'CircleDot';
    case 'pull_request':
    case 'pr': return 'GitPullRequest';
    case 'release': return 'Tag';
    case 'discussion': return 'MessageCircle';
    default: return 'Bell';
  }
}
