# Oh My GitHub Desktop

An unofficial GitHub Desktop client built with **Wails v2 + Go + React + TypeScript**.

Inspired by [ohmygit-hub/ohmygithub](https://github.com/ohmygit-hub/ohmygithub), this app brings GitHub to your desktop with a minimal, keyboard-friendly interface.

## Features

### Sidebar Navigation
- **Overview** — Dashboard with stats cards and recent activity feed
- **Notifications** — Unread/read notifications with auto-refresh (every 60s)
- **Pull Requests** — Browse open/closed/all PRs with labels and author avatars
- **Issues** — Browse issues with state filtering and label display
- **Actions** — Real-time workflow runs, jobs, steps, and live logs per repository
- **Repositories** — Search GitHub repos, browse your repos, filter by language

### Right Preview Panel
- Click any PR, Issue, or Notification to open a detail panel on the right
- PR diffs loaded automatically via the GitHub diff API
- Quick link to open on GitHub.com

### Bookmarks
- Bookmark any repository or page directly from the sidebar
- Add, view, and remove bookmarks inline

### Multi-Account Support
- Add multiple GitHub Personal Access Tokens
- Switch between accounts instantly
- Each account keeps its own auth context

### Settings
- Theme (Dark / Light / System)
- Font size (12–20px)
- Code font family
- Account management (add/remove/switch)

## Getting Started

### Prerequisites

- [Wails v2](https://wails.io/docs/gettingstarted/installation) (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- Go 1.23+
- Node.js 18+

### Development

```bash
# Install frontend dependencies
cd frontend && npm install

# Run in development mode (hot reload)
cd .. && wails dev

# Build for production
wails build
```

### Adding a GitHub Token

1. Open the app and click **Settings** (gear icon in sidebar footer)
2. Generate a GitHub Personal Access Token with these scopes:
   - `repo` (full control of private repositories)
   - `notifications` (access notifications)
   - `workflow` (view Actions)
3. Paste the token and click **Add**

Alternatively, run the app and click "Sign in with GitHub" in the sidebar.

## Project Structure

```
ohmygithub-desktop/
├── main.go                  # Wails entry point
├── app.go                   # Go backend (GitHub API client, settings, bookmarks)
├── wails.json               # Wails config
├── go.mod / go.sum          # Go dependencies
├── frontend/
│   ├── index.html
│   ├── src/
│   │   ├── App.tsx          # Main app with routing
│   │   ├── main.tsx         # React entry
│   │   ├── style.css        # GitHub-dark inspired theme
│   │   ├── lib/
│   │   │   └── api.ts       # API types + Wails bindings
│   │   ├── components/
│   │   │   ├── Sidebar.tsx       # Navigation sidebar
│   │   │   ├── PreviewPanel.tsx  # Right panel for PR/issue details
│   │   │   ├── SettingsModal.tsx  # Settings + account management
│   │   │   └── Toast.tsx         # Toast notifications
│   │   └── pages/
│   │       ├── OverviewPage.tsx      # Dashboard
│   │       ├── NotificationsPage.tsx  # Notifications
│   │       ├── PullRequestsPage.tsx   # PR list
│   │       ├── IssuesPage.tsx         # Issue list
│   │       ├── ActionsPage.tsx        # Workflow runs + live jobs
│   │       └── RepositoriesPage.tsx   # Repo browser + search
│   └── wailsjs/             # Auto-generated Wails bindings
└── build/                   # Wails build cache
```

## Go Backend API

| Method | Description |
|---|---|
| `GetSettings()` / `UpdateSettings()` | App settings CRUD |
| `AddAccount(token)` / `RemoveAccount(index)` / `SwitchAccount(index)` | Account management |
| `AddBookmark()` / `RemoveBookmark()` | Bookmark management |
| `GetNotifications()` | GitHub notifications |
| `GetPullRequests(state, sort)` | PR search via GitHub Issues API |
| `GetPRDiff(repo, number)` | Raw diff for a PR |
| `GetPRFiles(repo, number)` | Changed files in a PR |
| `GetIssues(state, sort)` | Issue search |
| `GetWorkflowRuns(repo)` | Actions workflow runs |
| `GetWorkflowRunJobs(repo, runID)` | Jobs for a workflow run |
| `GetWorkflowLogs(repo, jobID)` | Job logs |
| `GetMyRepos(sort)` | User's repositories |
| `SearchRepos(query)` | Repository search |
| `OpenExternal(url)` | Open URL in system browser |

## Tech Stack

### Backend
- **Wails v2** — Desktop shell
- **Go 1.23+** — GitHub REST API client (direct HTTP, no external SDK)

### Frontend
- **React 18** + **TypeScript 4**
- **Vite 3** — Build tool
- **Custom CSS** — GitHub-dark inspired design system (no UI framework)

## License

MIT
