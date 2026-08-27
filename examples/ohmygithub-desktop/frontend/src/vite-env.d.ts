/// <reference types="vite/client" />

// Wails bindings
declare namespace Go {
  namespace main {
    namespace App {
      function GetSettings(): Promise<string>;
      function UpdateSettings(json: string): Promise<void>;
      function AddAccount(token: string): Promise<string>;
      function RemoveAccount(index: number): Promise<void>;
      function SwitchAccount(index: number): Promise<void>;

      // Bookmarks
      function AddBookmark(title: string, url: string, icon: string): Promise<string>;
      function RemoveBookmark(id: string): Promise<void>;
      function ReorderBookmarks(ids: string[]): Promise<void>;

      // GitHub API
      function GetNotifications(): Promise<string>;
      function MarkNotificationRead(id: string): Promise<void>;
      function MarkAllNotificationsRead(): Promise<void>;

      function GetPullRequests(state: string, sort: string, repo: string): Promise<string>;
      function GetPRDiff(repo: string, number: number): Promise<string>;
      function GetPRFiles(repo: string, number: number): Promise<string>;

      function GetIssues(state: string, sort: string, repo: string): Promise<string>;

      function GetWorkflowRuns(repo: string): Promise<string>;
      function GetWorkflowRunJobs(repo: string, runID: number): Promise<string>;
      function GetWorkflowLogs(repo: string, jobID: number): Promise<string>;

      function GetMyRepos(sort: string): Promise<string>;
      function SearchRepos(query: string): Promise<string>;
      function GetStarredRepos(): Promise<string>;
      function SyncRepos(kind: string): Promise<void>;
      function GetStarGroups(): Promise<string>;
      function CreateStarGroup(name: string): Promise<string>;
      function DeleteStarGroup(id: string): Promise<void>;
      function RenameStarGroup(id: string, name: string): Promise<void>;
      function AddRepoToStarGroup(groupID: string, repoFullName: string): Promise<void>;
      function RemoveRepoFromStarGroup(groupID: string, repoFullName: string): Promise<void>;
      function ReorderStarGroups(ids: string[]): Promise<void>;
      function StarRepo(repoFullName: string): Promise<void>;
      function UnstarRepo(repoFullName: string): Promise<void>;

      function GetRepoContents(repo: string, path: string): Promise<string>;

      function OpenExternal(url: string): Promise<void>;
      function ShowMessage(title: string, message: string): Promise<void>;
    }
  }
}

interface Window {
  go: typeof Go;
}
