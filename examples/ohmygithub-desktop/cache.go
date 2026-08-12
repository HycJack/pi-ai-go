package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite" // 注册 sqlite 驱动（纯 Go 实现，无需 CGO）
)

// ============================================================================
// 缓存常量与类型
// ============================================================================

const (
	repoKindMine    = "mine"
	repoKindStarred = "starred"
)

// syncInterval 后台增量同步的最小间隔。超过此时间再次访问时触发后台同步。
const syncInterval = 24 * time.Hour

// fullSyncInterval 全量校正的最小间隔。超过此时间后下一次同步走全量分支。
// my repos 用 since 参数做日常增量（仅拉更新的），但删除/转 private 等变更
// 无法通过 since 发现，因此定期执行一次全量替换校正。
const fullSyncInterval = 7 * 24 * time.Hour

// RepoCacheItem 仓库缓存项（对应 repos 表的一行）。
// JSON tag 与前端 Repo 类型对齐，前端可安全断言为 Repo。
type RepoCacheItem struct {
	Name        string `json:"name"`
	Owner       string `json:"owner"`
	FullName    string `json:"fullName"`
	Description string `json:"description"`
	Language    string `json:"language"`
	Stars       int    `json:"stars"`
	Forks       int    `json:"forks"`
	OpenIssues  int    `json:"openIssues"`
	Private     bool   `json:"private"`
	HTMLURL     string `json:"htmlUrl"`
	UpdatedAt   string `json:"updatedAt"`
}

// ============================================================================
// 同步锁：防止同一 kind 并发同步
// ============================================================================

var (
	syncMu    sync.Mutex
	syncLocks = make(map[string]bool)
)

// tryLockSync 尝试获取同步锁。成功返回 true，已被占用返回 false。
func tryLockSync(kind string) bool {
	syncMu.Lock()
	defer syncMu.Unlock()
	if syncLocks[kind] {
		return false
	}
	syncLocks[kind] = true
	return true
}

// unlockSync 释放同步锁。
func unlockSync(kind string) {
	syncMu.Lock()
	defer syncMu.Unlock()
	delete(syncLocks, kind)
}

// needsSync 判断是否需要后台同步（lastSync 为 0 或超过 syncInterval）。
func needsSync(lastSync int64) bool {
	if lastSync == 0 {
		return true
	}
	return time.Since(time.Unix(lastSync, 0)) > syncInterval
}

// needsFullSync 判断是否需要走全量校正分支（lastFullSync 为 0 或超过 fullSyncInterval）。
func needsFullSync(lastFullSync int64) bool {
	if lastFullSync == 0 {
		return true
	}
	return time.Since(time.Unix(lastFullSync, 0)) > fullSyncInterval
}

// ============================================================================
// 数据库初始化
// ============================================================================

// initDB 打开/创建 SQLite 数据库并建表。
func initDB(dbPath string) (*sql.DB, error) {
	if dir := filepath.Dir(dbPath); dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite 并发写入需要串行化，单连接避免锁冲突。
	db.SetMaxOpenConns(1)
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("exec pragma %q: %w", p, err)
		}
	}

	schema := `
	CREATE TABLE IF NOT EXISTS repos (
		full_name   TEXT NOT NULL,
		kind        TEXT NOT NULL,
		name        TEXT,
		owner       TEXT,
		description TEXT,
		language    TEXT,
		stars       INTEGER DEFAULT 0,
		forks       INTEGER DEFAULT 0,
		open_issues INTEGER DEFAULT 0,
		private     INTEGER DEFAULT 0,
		html_url    TEXT,
		updated_at  TEXT,
		cached_at   INTEGER,
		PRIMARY KEY (full_name, kind)
	);
	CREATE INDEX IF NOT EXISTS idx_repos_kind ON repos(kind);
	CREATE INDEX IF NOT EXISTS idx_repos_updated ON repos(kind, updated_at DESC);

	CREATE TABLE IF NOT EXISTS sync_state (
		kind            TEXT PRIMARY KEY,
		last_sync       INTEGER,
		last_full_sync  INTEGER DEFAULT 0,
		total_count     INTEGER DEFAULT 0
	);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	// 兼容旧库：sync_state 表若缺少 last_full_sync 列则补上。
	var hasFullSyncCol int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('sync_state') WHERE name='last_full_sync'`).Scan(&hasFullSyncCol); err == nil && hasFullSyncCol == 0 {
		if _, err := db.Exec(`ALTER TABLE sync_state ADD COLUMN last_full_sync INTEGER DEFAULT 0`); err != nil {
			db.Close()
			return nil, fmt.Errorf("alter sync_state: %w", err)
		}
	}

	return db, nil
}

// ============================================================================
// 缓存读写
// ============================================================================

// cacheRepos 全量替换某个 kind 的缓存（事务：先删后插），并更新 sync_state。
// full=true 时同步刷新 last_full_sync 时间戳。
func cacheRepos(db *sql.DB, kind string, items []RepoCacheItem, full bool) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM repos WHERE kind = ?", kind); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO repos
		(full_name, kind, name, owner, description, language, stars, forks, open_issues, private, html_url, updated_at, cached_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, r := range items {
		private := 0
		if r.Private {
			private = 1
		}
		if _, err := stmt.Exec(r.FullName, kind, r.Name, r.Owner, r.Description, r.Language, r.Stars, r.Forks, r.OpenIssues, private, r.HTMLURL, r.UpdatedAt, now); err != nil {
			return err
		}
	}

	if full {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO sync_state (kind, last_sync, last_full_sync, total_count) VALUES (?, ?, ?, ?)`, kind, now, now, len(items)); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO sync_state (kind, last_sync, total_count) VALUES (?, ?, COALESCE((SELECT total_count FROM sync_state WHERE kind=?), 0))`, kind, now, kind); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// cacheReposIncremental 增量合并缓存：对返回的 repo 列表做 UPSERT（不删除已有项），
// 适用于 GitHub /user/repos?since=... 这种只返回变更项的接口。
// full=false 仅刷新 last_sync；删除/转 private 等变更需依赖定期全量校正。
func cacheReposIncremental(db *sql.DB, kind string, items []RepoCacheItem) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO repos
		(full_name, kind, name, owner, description, language, stars, forks, open_issues, private, html_url, updated_at, cached_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(full_name, kind) DO UPDATE SET
			name=excluded.name, owner=excluded.owner, description=excluded.description,
			language=excluded.language, stars=excluded.stars, forks=excluded.forks,
			open_issues=excluded.open_issues, private=excluded.private, html_url=excluded.html_url,
			updated_at=excluded.updated_at, cached_at=excluded.cached_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, r := range items {
		private := 0
		if r.Private {
			private = 1
		}
		if _, err := stmt.Exec(r.FullName, kind, r.Name, r.Owner, r.Description, r.Language, r.Stars, r.Forks, r.OpenIssues, private, r.HTMLURL, r.UpdatedAt, now); err != nil {
			return err
		}
	}

	// 增量同步只更新 last_sync，保留 last_full_sync 和 total_count
	if _, err := tx.Exec(`INSERT OR REPLACE INTO sync_state (kind, last_sync, last_full_sync, total_count)
		VALUES (?, ?,
			COALESCE((SELECT last_full_sync FROM sync_state WHERE kind=?), 0),
			(SELECT COUNT(*) FROM repos WHERE kind=?))`,
		kind, now, kind, kind); err != nil {
		return err
	}

	return tx.Commit()
}

// loadCachedRepos 读取某个 kind 的全部缓存，返回列表和最大 cached_at（unix 秒）。
func loadCachedRepos(db *sql.DB, kind string) ([]RepoCacheItem, int64, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("db is nil")
	}
	rows, err := db.Query(`SELECT name, owner, full_name, description, language, stars, forks, open_issues, private, html_url, updated_at, cached_at
		FROM repos WHERE kind = ? ORDER BY updated_at DESC`, kind)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []RepoCacheItem
	var cachedAt int64
	for rows.Next() {
		var r RepoCacheItem
		var private int
		var rowCachedAt int64
		if err := rows.Scan(&r.Name, &r.Owner, &r.FullName, &r.Description, &r.Language, &r.Stars, &r.Forks, &r.OpenIssues, &private, &r.HTMLURL, &r.UpdatedAt, &rowCachedAt); err != nil {
			return nil, 0, err
		}
		r.Private = private != 0
		if rowCachedAt > cachedAt {
			cachedAt = rowCachedAt
		}
		items = append(items, r)
	}
	return items, cachedAt, rows.Err()
}

// getSyncState 读取同步状态：lastSync 最后一次同步（unix 秒），lastFullSync 最后一次
// 全量校正，totalCount 缓存条数。任一为 0 表示从未执行对应操作。
func getSyncState(db *sql.DB, kind string) (lastSync int64, lastFullSync int64, totalCount int, err error) {
	if db == nil {
		return 0, 0, 0, fmt.Errorf("db is nil")
	}
	var ls, lf sql.NullInt64
	var tc sql.NullInt64
	err = db.QueryRow("SELECT last_sync, last_full_sync, total_count FROM sync_state WHERE kind = ?", kind).Scan(&ls, &lf, &tc)
	if err == sql.ErrNoRows {
		return 0, 0, 0, nil
	}
	if err != nil {
		return 0, 0, 0, err
	}
	if ls.Valid {
		lastSync = ls.Int64
	}
	if lf.Valid {
		lastFullSync = lf.Int64
	}
	if tc.Valid {
		totalCount = int(tc.Int64)
	}
	return
}

// ============================================================================
// 响应序列化
// ============================================================================

// marshalCachedResponse 将缓存数据序列化为 { data, cachedAt, syncing } JSON。
// 用于 GetMyRepos 的返回值，前端按此格式解析。
func marshalCachedResponse(items []RepoCacheItem, cachedAt int64, syncing bool) (string, error) {
	if items == nil {
		items = []RepoCacheItem{}
	}
	resp := struct {
		Data     []RepoCacheItem `json:"data"`
		CachedAt int64           `json:"cachedAt"`
		Syncing  bool            `json:"syncing"`
	}{
		Data:     items,
		CachedAt: cachedAt,
		Syncing:  syncing,
	}
	result, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}
	return string(result), nil
}
