package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================================
// Types
// ============================================================================

type GitHubAccount struct {
	Token     string `json:"token"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatarUrl"`
}

type AppSettings struct {
	Accounts       []GitHubAccount `json:"accounts"`
	ActiveAccount  int             `json:"activeAccount"`
	Theme          string          `json:"theme"`          // "dark" | "light" | "system"
	FontSize       int             `json:"fontSize"`       // 12-20
	CodeFont       string          `json:"codeFont"`       // font family
	Bookmarks      []Bookmark      `json:"bookmarks"`
	WindowWidth    int             `json:"windowWidth"`
	WindowHeight   int             `json:"windowHeight"`
}

type Bookmark struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Icon    string `json:"icon"`
	Order   int    `json:"order"`
}

type Notification struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Repo      string `json:"repo"`
	Type      string `json:"type"` // "issue", "pr", "release", "discussion"
	State     string `json:"state"`
	URL       string `json:"url"`
	UpdatedAt string `json:"updatedAt"`
	Read      bool   `json:"read"`
}

type PullRequest struct {
	ID         int    `json:"id"`
	Number     int    `json:"number"`
	Title      string `json:"title"`
	Repo       string `json:"repo"`
	State      string `json:"state"`
	User       string `json:"user"`
	AvatarURL  string `json:"avatarUrl"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
	Draft      bool   `json:"draft"`
	Labels     []Label `json:"labels"`
	Mergeable  string `json:"mergeable"`
	ReviewStatus string `json:"reviewStatus"`
}

type Issue struct {
	ID        int      `json:"id"`
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	Repo      string   `json:"repo"`
	State     string   `json:"state"`
	User      string   `json:"user"`
	AvatarURL string   `json:"avatarUrl"`
	CreatedAt string   `json:"createdAt"`
	Labels    []Label  `json:"labels"`
	Comments  int      `json:"comments"`
	Body      string   `json:"body"`
}

type Label struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type Repo struct {
	Name        string `json:"name"`
	Owner       string `json:"owner"`
	FullName    string `json:"fullName"`
	Description string `json:"description"`
	Language    string `json:"language"`
	Stars       int    `json:"stars"`
	Forks       int    `json:"forks"`
	OpenIssues  int    `json:"openIssues"`
	Private     bool   `json:"private"`
	UpdatedAt   string `json:"updatedAt"`
}

type SearchResult struct {
	TotalCount int    `json:"totalCount"`
	Items      []Repo `json:"items"`
}

type WorkflowRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	HeadBranch string `json:"headBranch"`
	Event      string `json:"event"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
	Actor      string `json:"actor"`
	HTMLURL    string `json:"htmlUrl"`
	Jobs       []Job  `json:"jobs,omitempty"`
}

type Job struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Conclusion  string     `json:"conclusion"`
	StartedAt   string     `json:"startedAt"`
	CompletedAt string     `json:"completedAt"`
	Steps       []JobStep  `json:"steps"`
}

type JobStep struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Number     int    `json:"number"`
}

type FileContent struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"` // "file" | "dir"
	Content  string `json:"content,omitempty"`
	Size     int    `json:"size"`
	HTMLURL  string `json:"htmlUrl"`
	Encoding string `json:"encoding,omitempty"`
}

type DiffContent struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`
	Content   string `json:"content,omitempty"`
}

// ============================================================================
// App struct
// ============================================================================

type App struct {
	ctx       context.Context
	settings  AppSettings
	settingsMu sync.RWMutex
	settingsPath string
	httpClient *http.Client
}

func NewApp() *App {
	home, _ := os.UserHomeDir()
	settingsDir := filepath.Join(home, ".ohmygithub")
	os.MkdirAll(settingsDir, 0755)
	return &App{
		settingsPath: filepath.Join(settingsDir, "settings.json"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.loadSettings()
}

func (a *App) shutdown(ctx context.Context) {
	a.saveSettings()
}

// ============================================================================
// Settings
// ============================================================================

func (a *App) loadSettings() {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	data, err := os.ReadFile(a.settingsPath)
	if err != nil {
		a.settings = AppSettings{
			Theme:    "dark",
			FontSize: 14,
			CodeFont: "JetBrains Mono, Fira Code, monospace",
		}
		return
	}

	json.Unmarshal(data, &a.settings)

	// Defaults
	if a.settings.Theme == "" {
		a.settings.Theme = "dark"
	}
	if a.settings.FontSize == 0 {
		a.settings.FontSize = 14
	}
	if a.settings.CodeFont == "" {
		a.settings.CodeFont = "JetBrains Mono, Fira Code, monospace"
	}
}

func (a *App) saveSettings() {
	a.settingsMu.RLock()
	data, err := json.MarshalIndent(a.settings, "", "  ")
	a.settingsMu.RUnlock()
	if err != nil {
		return
	}
	os.WriteFile(a.settingsPath, data, 0644)
}

// ============================================================================
// Settings API
// ============================================================================

func (a *App) GetSettings() string {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	data, _ := json.Marshal(a.settings)
	return string(data)
}

func (a *App) UpdateSettings(jsonStr string) error {
	var newSettings AppSettings
	if err := json.Unmarshal([]byte(jsonStr), &newSettings); err != nil {
		return err
	}
	a.settingsMu.Lock()
	a.settings = newSettings
	a.settingsMu.Unlock()
	a.saveSettings()
	return nil
}

func (a *App) AddAccount(token string) (string, error) {
	// Verify token with GitHub API
	user, avatar, err := a.fetchCurrentUser(token)
	if err != nil {
		return "", fmt.Errorf("token verification failed: %w", err)
	}

	account := GitHubAccount{
		Token:     token,
		Username:  user,
		AvatarURL: avatar,
	}

	a.settingsMu.Lock()
	a.settings.Accounts = append(a.settings.Accounts, account)
	if len(a.settings.Accounts) == 1 {
		a.settings.ActiveAccount = 0
	}
	a.settingsMu.Unlock()
	a.saveSettings()

	return user, nil
}

func (a *App) RemoveAccount(index int) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	if index < 0 || index >= len(a.settings.Accounts) {
		return fmt.Errorf("invalid account index")
	}

	a.settings.Accounts = append(a.settings.Accounts[:index], a.settings.Accounts[index+1:]...)
	if a.settings.ActiveAccount >= len(a.settings.Accounts) {
		a.settings.ActiveAccount = 0
	}
	return nil
}

func (a *App) SwitchAccount(index int) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	if index < 0 || index >= len(a.settings.Accounts) {
		return fmt.Errorf("invalid account index")
	}
	a.settings.ActiveAccount = index
	return nil
}

// ============================================================================
// Bookmarks
// ============================================================================

func (a *App) AddBookmark(title, urlStr, icon string) (string, error) {
	id := fmt.Sprintf("bm-%d", time.Now().UnixNano())

	a.settingsMu.Lock()
	a.settings.Bookmarks = append(a.settings.Bookmarks, Bookmark{
		ID:    id,
		Title: title,
		URL:   urlStr,
		Icon:  icon,
		Order: len(a.settings.Bookmarks),
	})
	a.settingsMu.Unlock()
	a.saveSettings()

	return id, nil
}

func (a *App) RemoveBookmark(id string) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	for i, b := range a.settings.Bookmarks {
		if b.ID == id {
			a.settings.Bookmarks = append(a.settings.Bookmarks[:i], a.settings.Bookmarks[i+1:]...)
			break
		}
	}
	return nil
}

func (a *App) ReorderBookmarks(ids []string) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	bmMap := make(map[string]Bookmark)
	for _, b := range a.settings.Bookmarks {
		bmMap[b.ID] = b
	}

	newBookmarks := make([]Bookmark, 0, len(ids))
	for i, id := range ids {
		if b, ok := bmMap[id]; ok {
			b.Order = i
			newBookmarks = append(newBookmarks, b)
		}
	}
	a.settings.Bookmarks = newBookmarks
	return nil
}

// ============================================================================
// GitHub API helpers
// ============================================================================

func (a *App) getActiveToken() (string, error) {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()

	if len(a.settings.Accounts) == 0 {
		return "", fmt.Errorf("no GitHub account configured. Please add a token in Settings")
	}
	if a.settings.ActiveAccount >= len(a.settings.Accounts) {
		return "", fmt.Errorf("invalid active account")
	}
	return a.settings.Accounts[a.settings.ActiveAccount].Token, nil
}

func (a *App) githubAPI(method, path string, body io.Reader) ([]byte, error) {
	token, err := a.getActiveToken()
	if err != nil {
		return nil, err
	}
	return a.githubAPIWithToken(method, path, body, token)
}

func (a *App) githubAPIWithToken(method, path string, body io.Reader, token string) ([]byte, error) {
	urlStr := "https://api.github.com" + path
	req, err := http.NewRequestWithContext(a.ctx, method, urlStr, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "OhMyGitHub-Desktop/1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, string(data))
	}

	return data, nil
}

func (a *App) githubGraphQL(query string, variables map[string]interface{}) ([]byte, error) {
	token, err := a.getActiveToken()
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(a.ctx, "POST", "https://api.github.com/graphql", strings.NewReader(string(bodyJSON)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "OhMyGitHub-Desktop/1.0")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GraphQL error (%d): %s", resp.StatusCode, string(data))
	}

	return data, nil
}

func (a *App) fetchCurrentUser(token string) (string, string, error) {
	req, err := http.NewRequestWithContext(a.ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "OhMyGitHub-Desktop/1.0")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)

	var result struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", "", err
	}

	return result.Login, result.AvatarURL, nil
}

// ============================================================================
// GitHub API: Notifications
// ============================================================================

func (a *App) GetNotifications() (string, error) {
	data, err := a.githubAPI("GET", "/notifications?participating=true&per_page=50", nil)
	if err != nil {
		return "", err
	}

	var raw []struct {
		ID       string `json:"id"`
		Subject  struct {
			Title string `json:"title"`
			Type  string `json:"type"`
			URL   string `json:"url"`
		} `json:"subject"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		Reason      string `json:"reason"`
		Unread      bool   `json:"unread"`
		UpdatedAt   string `json:"updated_at"`
		LastReadAt  string `json:"last_read_at"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return "", err
	}

	notifications := make([]Notification, 0, len(raw))
	for _, n := range raw {
		notifications = append(notifications, Notification{
			ID:        n.ID,
			Title:     n.Subject.Title,
			Repo:      n.Repository.FullName,
			Type:      strings.ToLower(n.Subject.Type),
			URL:       n.Subject.URL,
			UpdatedAt: n.UpdatedAt,
			Read:      !n.Unread,
		})
	}

	result, _ := json.Marshal(notifications)
	return string(result), nil
}

func (a *App) MarkNotificationRead(id string) error {
	_, err := a.githubAPI("PATCH", "/notifications/threads/"+id, nil)
	return err
}

func (a *App) MarkAllNotificationsRead() error {
	_, err := a.githubAPI("PUT", "/notifications", strings.NewReader(`{"last_read_at":"`+time.Now().UTC().Format(time.RFC3339)+`"}`))
	return err
}

// ============================================================================
// GitHub API: Pull Requests
// ============================================================================

func (a *App) GetPullRequests(state, sort string) (string, error) {
	if state == "" {
		state = "all"
	}
	if sort == "" {
		sort = "updated"
	}

	data, err := a.githubAPI("GET", fmt.Sprintf("/search/issues?q=is:pr+is:%s&sort=%s&per_page=50", state, sort), nil)
	if err != nil {
		return "", err
	}

	var searchResult struct {
		Items []struct {
			ID             int      `json:"id"`
			Number         int      `json:"number"`
			Title          string   `json:"title"`
			State          string   `json:"state"`
			User           struct {
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
			} `json:"user"`
			CreatedAt      string   `json:"created_at"`
			UpdatedAt      string   `json:"updated_at"`
			PullRequest    struct {
				Draft bool   `json:"draft"`
				URL   string `json:"url"`
			} `json:"pull_request"`
			Labels         []struct {
				Name  string `json:"name"`
				Color string `json:"color"`
			} `json:"labels"`
			RepositoryURL  string `json:"repository_url"`
			Repository     string `json:"-"`
		} `json:"items"`
	}

	if err := json.Unmarshal(data, &searchResult); err != nil {
		return "", err
	}

	prs := make([]PullRequest, 0, len(searchResult.Items))
	for _, item := range searchResult.Items {
		// Extract repo name from repository_url
		repoFull := strings.TrimPrefix(item.RepositoryURL, "https://api.github.com/repos/")
		labels := make([]Label, len(item.Labels))
		for i, l := range item.Labels {
			labels[i] = Label{Name: l.Name, Color: l.Color}
		}
		prs = append(prs, PullRequest{
			ID:        item.ID,
			Number:    item.Number,
			Title:     item.Title,
			Repo:      repoFull,
			State:     item.State,
			User:      item.User.Login,
			AvatarURL: item.User.AvatarURL,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
			Draft:     item.PullRequest.Draft,
			Labels:    labels,
		})
	}

	result, _ := json.Marshal(prs)
	return string(result), nil
}

func (a *App) GetPRDiff(repo string, number int) (string, error) {
	urlStr := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d", repo, number)
	token, err := a.getActiveToken()
	if err != nil {
		return "", err
	}

	req, _ := http.NewRequestWithContext(a.ctx, "GET", urlStr, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3.diff")
	req.Header.Set("User-Agent", "OhMyGitHub-Desktop/1.0")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	return string(data), nil
}

func (a *App) GetPRFiles(repo string, number int) (string, error) {
	data, err := a.githubAPI("GET", fmt.Sprintf("/repos/%s/pulls/%d/files", repo, number), nil)
	if err != nil {
		return "", err
	}

	var raw []struct {
		Filename  string `json:"filename"`
		Status    string `json:"status"`
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
		Patch     string `json:"patch"`
	}

	json.Unmarshal(data, &raw)
	diffs := make([]DiffContent, len(raw))
	for i, f := range raw {
		diffs[i] = DiffContent{
			Filename:  f.Filename,
			Status:    f.Status,
			Additions: f.Additions,
			Deletions: f.Deletions,
			Patch:     f.Patch,
		}
	}
	result, _ := json.Marshal(diffs)
	return string(result), nil
}

// ============================================================================
// GitHub API: Issues
// ============================================================================

func (a *App) GetIssues(state, sort string) (string, error) {
	if state == "" {
		state = "all"
	}
	if sort == "" {
		sort = "updated"
	}

	data, err := a.githubAPI("GET", fmt.Sprintf("/search/issues?q=is:issue+is:%s&sort=%s&per_page=50", state, sort), nil)
	if err != nil {
		return "", err
	}

	var searchResult struct {
		Items []struct {
			ID            int      `json:"id"`
			Number        int      `json:"number"`
			Title         string   `json:"title"`
			State         string   `json:"state"`
			User          struct {
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
			} `json:"user"`
			CreatedAt     string   `json:"created_at"`
			RepositoryURL string   `json:"repository_url"`
			Labels        []struct {
				Name  string `json:"name"`
				Color string `json:"color"`
			} `json:"labels"`
			Comments      int      `json:"comments"`
			Body          string   `json:"body"`
		} `json:"items"`
	}

	json.Unmarshal(data, &searchResult)

	issues := make([]Issue, 0, len(searchResult.Items))
	for _, item := range searchResult.Items {
		repoFull := strings.TrimPrefix(item.RepositoryURL, "https://api.github.com/repos/")
		labels := make([]Label, len(item.Labels))
		for i, l := range item.Labels {
			labels[i] = Label{Name: l.Name, Color: l.Color}
		}
		issues = append(issues, Issue{
			ID:        item.ID,
			Number:    item.Number,
			Title:     item.Title,
			Repo:      repoFull,
			State:     item.State,
			User:      item.User.Login,
			AvatarURL: item.User.AvatarURL,
			CreatedAt: item.CreatedAt,
			Labels:    labels,
			Comments:  item.Comments,
			Body:      item.Body,
		})
	}

	result, _ := json.Marshal(issues)
	return string(result), nil
}

// ============================================================================
// GitHub API: Actions / Workflows
// ============================================================================

func (a *App) GetWorkflowRuns(repo string) (string, error) {
	data, err := a.githubAPI("GET", fmt.Sprintf("/repos/%s/actions/runs?per_page=20", repo), nil)
	if err != nil {
		return "", err
	}

	var raw struct {
		WorkflowRuns []struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			HeadBranch string `json:"head_branch"`
			Event      string `json:"event"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			CreatedAt  string `json:"created_at"`
			UpdatedAt  string `json:"updated_at"`
			Actor      struct {
				Login string `json:"login"`
			} `json:"actor"`
			HTMLURL string `json:"html_url"`
		} `json:"workflow_runs"`
	}

	json.Unmarshal(data, &raw)

	runs := make([]WorkflowRun, len(raw.WorkflowRuns))
	for i, r := range raw.WorkflowRuns {
		runs[i] = WorkflowRun{
			ID:         r.ID,
			Name:       r.Name,
			HeadBranch: r.HeadBranch,
			Event:      r.Event,
			Status:     r.Status,
			Conclusion: r.Conclusion,
			CreatedAt:  r.CreatedAt,
			UpdatedAt:  r.UpdatedAt,
			Actor:      r.Actor.Login,
			HTMLURL:    r.HTMLURL,
		}
	}

	result, _ := json.Marshal(runs)
	return string(result), nil
}

func (a *App) GetWorkflowRunJobs(repo string, runID int64) (string, error) {
	data, err := a.githubAPI("GET", fmt.Sprintf("/repos/%s/actions/runs/%d/jobs", repo, runID), nil)
	if err != nil {
		return "", err
	}

	var raw struct {
		Jobs []struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			Status      string `json:"status"`
			Conclusion  string `json:"conclusion"`
			StartedAt   string `json:"started_at"`
			CompletedAt string `json:"completed_at"`
			Steps       []struct {
				Name       string `json:"name"`
				Status     string `json:"status"`
				Conclusion string `json:"conclusion"`
				Number     int    `json:"number"`
			} `json:"steps"`
		} `json:"jobs"`
	}

	json.Unmarshal(data, &raw)

	jobs := make([]Job, len(raw.Jobs))
	for i, j := range raw.Jobs {
		steps := make([]JobStep, len(j.Steps))
		for si, s := range j.Steps {
			steps[si] = JobStep{
				Name:       s.Name,
				Status:     s.Status,
				Conclusion: s.Conclusion,
				Number:     s.Number,
			}
		}
		jobs[i] = Job{
			ID:          j.ID,
			Name:        j.Name,
			Status:      j.Status,
			Conclusion:  j.Conclusion,
			StartedAt:   j.StartedAt,
			CompletedAt: j.CompletedAt,
			Steps:       steps,
		}
	}

	result, _ := json.Marshal(jobs)
	return string(result), nil
}

func (a *App) GetWorkflowLogs(repo string, jobID int64) (string, error) {
	token, err := a.getActiveToken()
	if err != nil {
		return "", err
	}

	urlStr := fmt.Sprintf("https://api.github.com/repos/%s/actions/jobs/%d/logs", repo, jobID)
	req, _ := http.NewRequestWithContext(a.ctx, "GET", urlStr, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "OhMyGitHub-Desktop/1.0")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	return string(data), nil
}

// ============================================================================
// GitHub API: Repositories
// ============================================================================

func (a *App) GetMyRepos(sort string) (string, error) {
	if sort == "" {
		sort = "updated"
	}
	data, err := a.githubAPI("GET", fmt.Sprintf("/user/repos?sort=%s&per_page=50&type=all", sort), nil)
	if err != nil {
		return "", err
	}

	var raw []struct {
		Name        string `json:"name"`
		Owner       struct {
			Login string `json:"login"`
		} `json:"owner"`
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		Language    string `json:"language"`
		Stars       int    `json:"stargazers_count"`
		Forks       int    `json:"forks_count"`
		OpenIssues  int    `json:"open_issues_count"`
		Private     bool   `json:"private"`
		UpdatedAt   string `json:"updated_at"`
	}

	json.Unmarshal(data, &raw)

	repos := make([]Repo, len(raw))
	for i, r := range raw {
		repos[i] = Repo{
			Name:        r.Name,
			Owner:       r.Owner.Login,
			FullName:    r.FullName,
			Description: r.Description,
			Language:    r.Language,
			Stars:       r.Stars,
			Forks:       r.Forks,
			OpenIssues:  r.OpenIssues,
			Private:     r.Private,
			UpdatedAt:   r.UpdatedAt,
		}
	}

	result, _ := json.Marshal(repos)
	return string(result), nil
}

func (a *App) SearchRepos(query string) (string, error) {
	encoded := url.QueryEscape(query)
	data, err := a.githubAPI("GET", fmt.Sprintf("/search/repositories?q=%s&per_page=30", encoded), nil)
	if err != nil {
		return "", err
	}

	var raw struct {
		TotalCount int `json:"total_count"`
		Items      []struct {
			Name        string `json:"name"`
			Owner       struct {
				Login string `json:"login"`
			} `json:"owner"`
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			Language    string `json:"language"`
			Stars       int    `json:"stargazers_count"`
			Forks       int    `json:"forks_count"`
			OpenIssues  int    `json:"open_issues_count"`
			Private     bool   `json:"private"`
			UpdatedAt   string `json:"updated_at"`
		} `json:"items"`
	}

	json.Unmarshal(data, &raw)

	items := make([]Repo, len(raw.Items))
	for i, item := range raw.Items {
		items[i] = Repo{
			Name:        item.Name,
			Owner:       item.Owner.Login,
			FullName:    item.FullName,
			Description: item.Description,
			Language:    item.Language,
			Stars:       item.Stars,
			Forks:       item.Forks,
			OpenIssues:  item.OpenIssues,
			Private:     item.Private,
			UpdatedAt:   item.UpdatedAt,
		}
	}

	result, _ := json.Marshal(SearchResult{TotalCount: raw.TotalCount, Items: items})
	return string(result), nil
}

// ============================================================================
// GitHub API: File content browsing
// ============================================================================

func (a *App) GetRepoContents(repo, path string) (string, error) {
	if path == "" {
		path = ""
	}

	data, err := a.githubAPI("GET", fmt.Sprintf("/repos/%s/contents/%s", repo, path), nil)
	if err != nil {
		return "", err
	}

	// Check if it's an array (directory) or single object (file)
	if len(data) > 0 && data[0] == '[' {
		var raw []struct {
			Name    string `json:"name"`
			Path    string `json:"path"`
			Type    string `json:"type"`
			Size    int    `json:"size"`
			HTMLURL string `json:"html_url"`
		}
		json.Unmarshal(data, &raw)

		files := make([]FileContent, len(raw))
		for i, f := range raw {
			files[i] = FileContent{
				Name:    f.Name,
				Path:    f.Path,
				Type:    f.Type,
				Size:    f.Size,
				HTMLURL: f.HTMLURL,
			}
		}
		result, _ := json.Marshal(files)
		return string(result), nil
	}

	var raw struct {
		Name     string `json:"name"`
		Path     string `json:"path"`
		Type     string `json:"type"`
		Content  string `json:"content"`
		Size     int    `json:"size"`
		Encoding string `json:"encoding"`
		HTMLURL  string `json:"html_url"`
	}
	json.Unmarshal(data, &raw)

	files := []FileContent{{
		Name:     raw.Name,
		Path:     raw.Path,
		Type:     raw.Type,
		Content:  raw.Content,
		Size:     raw.Size,
		Encoding: raw.Encoding,
		HTMLURL:  raw.HTMLURL,
	}}
	result, _ := json.Marshal(files)
	return string(result), nil
}

// ============================================================================
// UI helpers
// ============================================================================

func (a *App) OpenExternal(urlStr string) error {
	runtime.BrowserOpenURL(a.ctx, urlStr)
	return nil
}

func (a *App) ShowMessage(title, message string) error {
	_, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Title:   title,
		Message: message,
	})
	return err
}
