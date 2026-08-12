package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
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
	Accounts      []GitHubAccount `json:"accounts"`
	ActiveAccount int             `json:"activeAccount"`
	Theme         string          `json:"theme"`    // "dark" | "light" | "system"
	FontSize      int             `json:"fontSize"` // 12-20
	CodeFont      string          `json:"codeFont"` // font family
	Bookmarks     []Bookmark      `json:"bookmarks"`
	StarGroups    []StarGroup     `json:"starGroups"` // 本地 star 仓库分组
	WindowWidth   int             `json:"windowWidth"`
	WindowHeight  int             `json:"windowHeight"`
}

// StarGroup 本地维护的 star 仓库分组（仅记录 repo 全名，仓库元数据由 GetStarredRepos 实时拉取）。
type StarGroup struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Repos []string `json:"repos"` // "owner/name" 列表
	Order int      `json:"order"`
}

type Bookmark struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Icon  string `json:"icon"`
	Order int    `json:"order"`
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
	ID           int     `json:"id"`
	Number       int     `json:"number"`
	Title        string  `json:"title"`
	Repo         string  `json:"repo"`
	State        string  `json:"state"`
	User         string  `json:"user"`
	AvatarURL    string  `json:"avatarUrl"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
	Draft        bool    `json:"draft"`
	Labels       []Label `json:"labels"`
	Mergeable    string  `json:"mergeable"`
	ReviewStatus string  `json:"reviewStatus"`
}

type Issue struct {
	ID        int     `json:"id"`
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	Repo      string  `json:"repo"`
	State     string  `json:"state"`
	User      string  `json:"user"`
	AvatarURL string  `json:"avatarUrl"`
	CreatedAt string  `json:"createdAt"`
	Labels    []Label `json:"labels"`
	Comments  int     `json:"comments"`
	Body      string  `json:"body"`
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
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
	StartedAt   string    `json:"startedAt"`
	CompletedAt string    `json:"completedAt"`
	Steps       []JobStep `json:"steps"`
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
	ctx          context.Context
	settings     AppSettings
	settingsMu   sync.RWMutex
	settingsPath string
	httpClient   *http.Client
	db           *sql.DB
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

	// 初始化 SQLite 缓存数据库
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".ohmygithub", "cache.db")
	db, err := initDB(dbPath)
	if err != nil {
		fmt.Printf("WARNING: failed to init cache db: %v\n", err)
	} else {
		a.db = db
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.saveSettings()
	if a.db != nil {
		a.db.Close()
	}
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
		a.ensureSettingsDefaults()
		return
	}

	json.Unmarshal(data, &a.settings)
	a.ensureSettingsDefaults()
}

// ensureSettingsDefaults 确保所有切片/映射字段非 nil，避免前端访问 .map() 时崩溃。
func (a *App) ensureSettingsDefaults() {
	if a.settings.Accounts == nil {
		a.settings.Accounts = []GitHubAccount{}
	}
	if a.settings.Bookmarks == nil {
		a.settings.Bookmarks = []Bookmark{}
	}
	if a.settings.StarGroups == nil {
		a.settings.StarGroups = []StarGroup{}
	}
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
// Star Groups (本地分组管理)
// ============================================================================

// CreateStarGroup 创建一个 star 分组
func (a *App) CreateStarGroup(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("group name cannot be empty")
	}
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	id := shortID("stargroup:" + name + ":" + time.Now().Format(time.RFC3339Nano))
	a.settings.StarGroups = append(a.settings.StarGroups, StarGroup{
		ID:    id,
		Name:  name,
		Repos: []string{},
		Order: len(a.settings.StarGroups),
	})
	if err := a.saveSettingsLocked(); err != nil {
		return "", err
	}
	return id, nil
}

// DeleteStarGroup 删除分组（不会动 GitHub 上的 star）
func (a *App) DeleteStarGroup(id string) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	for i, g := range a.settings.StarGroups {
		if g.ID == id {
			a.settings.StarGroups = append(a.settings.StarGroups[:i], a.settings.StarGroups[i+1:]...)
			break
		}
	}
	// 重新整理 order
	for i := range a.settings.StarGroups {
		a.settings.StarGroups[i].Order = i
	}
	return a.saveSettingsLocked()
}

// RenameStarGroup 重命名分组
func (a *App) RenameStarGroup(id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("group name cannot be empty")
	}
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	for i := range a.settings.StarGroups {
		if a.settings.StarGroups[i].ID == id {
			a.settings.StarGroups[i].Name = name
			return a.saveSettingsLocked()
		}
	}
	return fmt.Errorf("group not found: %s", id)
}

// AddRepoToStarGroup 将 repo（"owner/name"）加入指定分组；分组不存在时返回错误。
// 同一 repo 可属于多个分组。
func (a *App) AddRepoToStarGroup(groupID, repoFullName string) error {
	repoFullName = strings.TrimSpace(repoFullName)
	if repoFullName == "" {
		return fmt.Errorf("repo name cannot be empty")
	}
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	for i := range a.settings.StarGroups {
		if a.settings.StarGroups[i].ID == groupID {
			// 去重
			for _, r := range a.settings.StarGroups[i].Repos {
				if r == repoFullName {
					return nil // 已存在，幂等
				}
			}
			a.settings.StarGroups[i].Repos = append(a.settings.StarGroups[i].Repos, repoFullName)
			return a.saveSettingsLocked()
		}
	}
	return fmt.Errorf("group not found: %s", groupID)
}

// RemoveRepoFromStarGroup 从指定分组移除 repo
func (a *App) RemoveRepoFromStarGroup(groupID, repoFullName string) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	for i := range a.settings.StarGroups {
		if a.settings.StarGroups[i].ID == groupID {
			for j, r := range a.settings.StarGroups[i].Repos {
				if r == repoFullName {
					a.settings.StarGroups[i].Repos = append(
						a.settings.StarGroups[i].Repos[:j],
						a.settings.StarGroups[i].Repos[j+1:]...,
					)
					return a.saveSettingsLocked()
				}
			}
			return nil // 不存在，幂等
		}
	}
	return fmt.Errorf("group not found: %s", groupID)
}

// ReorderStarGroups 重排分组顺序
func (a *App) ReorderStarGroups(ids []string) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	gMap := make(map[string]StarGroup, len(a.settings.StarGroups))
	for _, g := range a.settings.StarGroups {
		gMap[g.ID] = g
	}
	newGroups := make([]StarGroup, 0, len(ids))
	for i, id := range ids {
		if g, ok := gMap[id]; ok {
			g.Order = i
			newGroups = append(newGroups, g)
		}
	}
	// 保留未在 ids 中的分组（追加到末尾）
	for _, g := range a.settings.StarGroups {
		if _, ok := gMap[g.ID]; !ok {
			continue
		}
		found := false
		for _, id := range ids {
			if id == g.ID {
				found = true
				break
			}
		}
		if !found {
			g.Order = len(newGroups)
			newGroups = append(newGroups, g)
		}
	}
	a.settings.StarGroups = newGroups
	return a.saveSettingsLocked()
}

// GetStarGroups 返回当前所有分组（含 repo 列表），由前端通过 GetSettings 也可获取，
// 这里单独提供方法便于主动刷新。
func (a *App) GetStarGroups() (string, error) {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	result, _ := json.Marshal(a.settings.StarGroups)
	return string(result), nil
}

// saveSettingsLocked 在已持有 settingsMu 写锁的情况下保存配置。
func (a *App) saveSettingsLocked() error {
	data, err := json.MarshalIndent(a.settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.settingsPath, data, 0644)
}

// shortID 生成确定性 base32 短哈希（取 sha1 前 8 字节 → 13 字符）。
func shortID(seed string) string {
	sum := sha1.Sum([]byte(seed))
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	return strings.ToLower(enc.EncodeToString(sum[:8]))
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

// githubAPIPaged 分页拉取列表型端点，自动合并多页 JSON 数组。
// maxPages 限制最多拉取页数（0 表示只拉一页）；返回合并后的 JSON 数组字节。
// 注意：仅适用于返回顶层数组的端点（如 /user/repos, /user/starred）。
func (a *App) githubAPIPaged(pathTemplate string, perPage, maxPages int) ([]byte, error) {
	if perPage <= 0 {
		perPage = 100
	}
	if maxPages <= 0 {
		maxPages = 1
	}

	var combined []json.RawMessage
	for page := 1; page <= maxPages; page++ {
		// 用 & 或 ? 拼接 page/per_page 参数
		sep := "&"
		if !strings.Contains(pathTemplate, "?") {
			sep = "?"
		}
		urlPath := fmt.Sprintf("%s%spage=%d&per_page=%d", pathTemplate, sep, page, perPage)
		data, err := a.githubAPI("GET", urlPath, nil)
		if err != nil {
			// 首页失败直接返回错误；后续页失败则用已收集的数据
			if page == 1 {
				return nil, err
			}
			break
		}

		var pageItems []json.RawMessage
		if err := json.Unmarshal(data, &pageItems); err != nil {
			if page == 1 {
				return nil, err
			}
			break
		}
		combined = append(combined, pageItems...)

		// 不足一页说明已是最后一页
		if len(pageItems) < perPage {
			break
		}
	}

	return json.Marshal(combined)
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
		ID      string `json:"id"`
		Subject struct {
			Title string `json:"title"`
			Type  string `json:"type"`
			URL   string `json:"url"`
		} `json:"subject"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		Reason     string `json:"reason"`
		Unread     bool   `json:"unread"`
		UpdatedAt  string `json:"updated_at"`
		LastReadAt string `json:"last_read_at"`
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

func (a *App) GetPullRequests(state, sort, repo string) (string, error) {
	if state == "" {
		state = "all"
	}
	if sort == "" {
		sort = "updated"
	}

	// repo 非空时按指定仓库过滤；为空时限定到当前用户参与的 PR
	var q string
	if repo = strings.TrimSpace(repo); repo != "" {
		q = fmt.Sprintf("is:pr+is:%s+repo:%s&sort=%s&per_page=100", state, url.PathEscape(repo), sort)
	} else {
		q = fmt.Sprintf("is:pr+is:%s+involves:@me&sort=%s&per_page=100", state, sort)
	}
	data, err := a.githubAPI("GET", fmt.Sprintf("/search/issues?q=%s", q), nil)
	if err != nil {
		return "", err
	}

	var searchResult struct {
		Items []struct {
			ID     int    `json:"id"`
			Number int    `json:"number"`
			Title  string `json:"title"`
			State  string `json:"state"`
			User   struct {
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
			} `json:"user"`
			CreatedAt   string `json:"created_at"`
			UpdatedAt   string `json:"updated_at"`
			PullRequest struct {
				Draft bool   `json:"draft"`
				URL   string `json:"url"`
			} `json:"pull_request"`
			Labels []struct {
				Name  string `json:"name"`
				Color string `json:"color"`
			} `json:"labels"`
			RepositoryURL string `json:"repository_url"`
			Repository    string `json:"-"`
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

func (a *App) GetIssues(state, sort, repo string) (string, error) {
	if state == "" {
		state = "all"
	}
	if sort == "" {
		sort = "updated"
	}

	// repo 非空时按指定仓库过滤；为空时限定到当前用户参与的 issue
	var q string
	if repo = strings.TrimSpace(repo); repo != "" {
		q = fmt.Sprintf("is:issue+is:%s+repo:%s&sort=%s&per_page=100", state, url.PathEscape(repo), sort)
	} else {
		q = fmt.Sprintf("is:issue+is:%s+involves:@me&sort=%s&per_page=100", state, sort)
	}
	data, err := a.githubAPI("GET", fmt.Sprintf("/search/issues?q=%s", q), nil)
	if err != nil {
		return "", err
	}

	var searchResult struct {
		Items []struct {
			ID     int    `json:"id"`
			Number int    `json:"number"`
			Title  string `json:"title"`
			State  string `json:"state"`
			User   struct {
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
			} `json:"user"`
			CreatedAt     string `json:"created_at"`
			RepositoryURL string `json:"repository_url"`
			Labels        []struct {
				Name  string `json:"name"`
				Color string `json:"color"`
			} `json:"labels"`
			Comments int    `json:"comments"`
			Body     string `json:"body"`
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

	// GitHub Actions logs API 返回的是 zip 压缩流（每个 step 一个文件）。
	// 解压并按文件名排序合并，便于阅读。
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("GitHub logs API error (%d): %s", resp.StatusCode, string(body))
	}

	// 尝试作为 zip 解压；若不是 zip（部分情况返回纯文本），直接返回原文。
	combined, zipErr := unzipConcat(body)
	if zipErr != nil {
		return string(body), nil
	}
	return combined, nil
}

// unzipConcat 解压 zip 数据，按文件名排序后拼接为单个字符串。
func unzipConcat(data []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	// 按文件名排序（通常是 0_*.txt, 1_*.txt, ...）
	sort.Slice(r.File, func(i, j int) bool {
		return r.File[i].Name < r.File[j].Name
	})
	var sb strings.Builder
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		buf, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("===== %s =====\n", f.Name))
		sb.Write(buf)
		if !strings.HasSuffix(sb.String(), "\n") {
			sb.WriteByte('\n')
		}
	}
	return sb.String(), nil
}

// ============================================================================
// GitHub API: Repositories
// ============================================================================

func (a *App) GetMyRepos(sort string) (string, error) {
	if sort == "" {
		sort = "updated"
	}

	// db 不可用时回退到直接 API 拉取
	if a.db == nil {
		return a.fetchMyReposFromAPI(sort)
	}

	// 缓存优先：先读 SQLite
	cached, cachedAt, err := loadCachedRepos(a.db, repoKindMine)
	if err == nil && len(cached) > 0 {
		// 检查是否需要后台同步
		lastSync, lastFullSync, _, _ := getSyncState(a.db, repoKindMine)
		if needsSync(lastSync) && tryLockSync(repoKindMine) {
			go a.syncMyRepos(sort, lastFullSync)
			return marshalCachedResponse(cached, cachedAt, true)
		}
		return marshalCachedResponse(cached, cachedAt, false)
	}

	// 无缓存：前台全量拉取
	if err := a.syncMyRepos(sort, 0); err != nil {
		return "", err
	}
	cached, cachedAt, _ = loadCachedRepos(a.db, repoKindMine)
	return marshalCachedResponse(cached, cachedAt, false)
}

// fetchMyReposFromAPI 直接从 GitHub API 拉取（无缓存模式）
func (a *App) fetchMyReposFromAPI(sort string) (string, error) {
	data, err := a.githubAPIPaged(fmt.Sprintf("/user/repos?sort=%s&type=all", sort), 100, 10)
	if err != nil {
		return "", err
	}
	items, err := parseRepoList(data)
	if err != nil {
		return "", err
	}
	result, _ := json.Marshal(items)
	return string(result), nil
}

// syncMyRepos 同步 my repos 到 SQLite。
// lastFullSync 用于判断走全量校正还是 since 增量：为 0 或超过 7 天则全量替换，
// 否则用 since=lastFullSync 拉取变更项做 UPSERT。
func (a *App) syncMyRepos(sort string, lastFullSync int64) error {
	defer unlockSync(repoKindMine)

	full := needsFullSync(lastFullSync)
	var data []byte
	var err error
	if full {
		data, err = a.githubAPIPaged(fmt.Sprintf("/user/repos?sort=%s&type=all", sort), 100, 10)
	} else {
		// GitHub /user/repos 的 since 参数基于 updated_at，需用 ISO8601 时间戳
		since := time.Unix(lastFullSync, 0).UTC().Format(time.RFC3339)
		data, err = a.githubAPIPaged(fmt.Sprintf("/user/repos?sort=%s&type=all&since=%s", sort, url.QueryEscape(since)), 100, 10)
	}
	if err != nil {
		return err
	}
	items, err := parseRepoList(data)
	if err != nil {
		return err
	}
	if full {
		return cacheRepos(a.db, repoKindMine, items, true)
	}
	return cacheReposIncremental(a.db, repoKindMine, items)
}

// parseRepoList 解析 GitHub API 返回的 repo 列表 JSON
func parseRepoList(data []byte) ([]RepoCacheItem, error) {
	var raw []struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		Language    string `json:"language"`
		Stars       int    `json:"stargazers_count"`
		Forks       int    `json:"forks_count"`
		OpenIssues  int    `json:"open_issues_count"`
		Private     bool   `json:"private"`
		HTMLURL     string `json:"html_url"`
		UpdatedAt   string `json:"updated_at"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	items := make([]RepoCacheItem, len(raw))
	for i, r := range raw {
		items[i] = RepoCacheItem{
			Name:        r.Name,
			Owner:       r.Owner.Login,
			FullName:    r.FullName,
			Description: r.Description,
			Language:    r.Language,
			Stars:       r.Stars,
			Forks:       r.Forks,
			OpenIssues:  r.OpenIssues,
			Private:     r.Private,
			HTMLURL:     r.HTMLURL,
			UpdatedAt:   r.UpdatedAt,
		}
	}
	return items, nil
}

func (a *App) SearchRepos(query string) (string, error) {
	encoded := url.QueryEscape(query)
	data, err := a.githubAPI("GET", fmt.Sprintf("/search/repositories?q=%s&per_page=100", encoded), nil)
	if err != nil {
		return "", err
	}

	var raw struct {
		TotalCount int `json:"total_count"`
		Items      []struct {
			Name  string `json:"name"`
			Owner struct {
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

// GetStarredRepos 返回当前用户 star 过的仓库列表（缓存优先 + 后台同步）。
// 返回值还会附加本地分组信息（groups: 该 repo 所属的分组 ID 列表），便于前端渲染。
func (a *App) GetStarredRepos() (string, error) {
	// db 不可用时回退到直接 API 拉取
	if a.db == nil {
		return a.fetchStarredReposFromAPI()
	}

	// 缓存优先
	cached, cachedAt, err := loadCachedRepos(a.db, repoKindStarred)
	if err == nil && len(cached) > 0 {
		lastSync, _, _, _ := getSyncState(a.db, repoKindStarred)
		if needsSync(lastSync) && tryLockSync(repoKindStarred) {
			go a.syncStarredRepos()
			return a.marshalStarredResponse(cached, cachedAt, true)
		}
		return a.marshalStarredResponse(cached, cachedAt, false)
	}

	// 无缓存：前台全量拉取
	if err := a.syncStarredRepos(); err != nil {
		return "", err
	}
	cached, cachedAt, _ = loadCachedRepos(a.db, repoKindStarred)
	return a.marshalStarredResponse(cached, cachedAt, false)
}

// fetchStarredReposFromAPI 直接从 API 拉取（无缓存模式）
func (a *App) fetchStarredReposFromAPI() (string, error) {
	data, err := a.githubAPIPaged("/user/starred?sort=pushed", 100, 20)
	if err != nil {
		return "", err
	}
	items, err := parseRepoList(data)
	if err != nil {
		return "", err
	}
	return a.marshalStarredResponse(items, 0, false)
}

// syncStarredRepos 全量同步 starred repos 到 SQLite。
// GitHub /user/starred 不支持 since 参数，始终走全量替换。
func (a *App) syncStarredRepos() error {
	defer unlockSync(repoKindStarred)
	data, err := a.githubAPIPaged("/user/starred?sort=pushed", 100, 20)
	if err != nil {
		return err
	}
	items, err := parseRepoList(data)
	if err != nil {
		return err
	}
	return cacheRepos(a.db, repoKindStarred, items, true)
}

// marshalStarredResponse 将仓库列表 + 分组信息 + cachedAt 序列化为 JSON
func (a *App) marshalStarredResponse(items []RepoCacheItem, cachedAt int64, syncing bool) (string, error) {
	// 构造 repo fullName → groups 反向索引
	a.settingsMu.RLock()
	repoToGroups := make(map[string][]string)
	for _, g := range a.settings.StarGroups {
		for _, r := range g.Repos {
			repoToGroups[r] = append(repoToGroups[r], g.ID)
		}
	}
	a.settingsMu.RUnlock()

	type StarredRepoOut struct {
		RepoCacheItem
		Groups []string `json:"groups"`
	}

	repos := make([]StarredRepoOut, len(items))
	for i, r := range items {
		groups := repoToGroups[r.FullName]
		if groups == nil {
			groups = []string{}
		}
		repos[i] = StarredRepoOut{
			RepoCacheItem: r,
			Groups:        groups,
		}
	}

	// 统一返回 { data, cachedAt, syncing } 格式
	resp := struct {
		Data     []StarredRepoOut `json:"data"`
		CachedAt int64            `json:"cachedAt"`
		Syncing  bool             `json:"syncing"`
	}{
		Data:     repos,
		CachedAt: cachedAt,
		Syncing:  syncing,
	}
	result, _ := json.Marshal(resp)
	return string(result), nil
}

// SyncRepos 供前端调用的手动强制同步 API（强制全量校正）。
// kind: "mine" | "starred" | "" (both)
func (a *App) SyncRepos(kind string) error {
	if a.db == nil {
		return fmt.Errorf("cache db not available")
	}
	kind = strings.TrimSpace(strings.ToLower(kind))
	// 传入 lastFullSync=0 强制 needsFullSync 返回 true，从而走全量分支
	if kind == "" || kind == "mine" {
		if tryLockSync(repoKindMine) {
			if err := a.syncMyRepos("updated", 0); err != nil {
				return err
			}
		}
	}
	if kind == "" || kind == "starred" {
		if tryLockSync(repoKindStarred) {
			if err := a.syncStarredRepos(); err != nil {
				return err
			}
		}
	}
	return nil
}

// StarRepo / UnstarRepo 转发到 GitHub API，同步用户 star 状态。
func (a *App) StarRepo(repoFullName string) error {
	repoFullName = strings.TrimSpace(repoFullName)
	if repoFullName == "" {
		return fmt.Errorf("repo name cannot be empty")
	}
	_, err := a.githubAPI("PUT", fmt.Sprintf("/user/starred/%s", url.PathEscape(repoFullName)), nil)
	return err
}

func (a *App) UnstarRepo(repoFullName string) error {
	repoFullName = strings.TrimSpace(repoFullName)
	if repoFullName == "" {
		return fmt.Errorf("repo name cannot be empty")
	}
	_, err := a.githubAPI("DELETE", fmt.Sprintf("/user/starred/%s", url.PathEscape(repoFullName)), nil)
	return err
}

// ============================================================================
// GitHub API: File content browsing
// ============================================================================

func (a *App) GetRepoContents(repo, path string) (string, error) {
	// URL escape path 中的特殊字符（如空格、中文），避免请求失败。
	// 注意：path 中的 "/" 不能被 escape，所以用 PathEscape 后再把 %2F 还原。
	escapedPath := url.PathEscape(path)
	escapedPath = strings.ReplaceAll(escapedPath, "%2F", "/")

	// path 为空时请求仓库根目录，endpoint 不带尾部斜杠（GitHub API 会 404）
	var endpoint string
	if escapedPath == "" {
		endpoint = fmt.Sprintf("/repos/%s/contents", url.PathEscape(repo))
	} else {
		endpoint = fmt.Sprintf("/repos/%s/contents/%s", url.PathEscape(repo), escapedPath)
	}
	data, err := a.githubAPI("GET", endpoint, nil)
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
