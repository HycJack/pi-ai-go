package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"pi-ai-go/providers"

	"chat-app/contextmgr"
	"chat-app/keypool"
	"chat-app/memory"
)

// NewApp creates a new App instance with default settings.
func NewApp() *App {
	dataDir := getDataDir()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		LogError("[app] failed to create data dir %s: %v", dataDir, err)
	}
	return &App{
		dataDir:              dataDir,
		settingsPath:         filepath.Join(dataDir, "settings.json"),
		conversationSettings: make(map[string]ConversationSettings),
		tokenStats:           make(map[string]*contextmgr.TokenStats),
		settings: AppSettings{
			Providers: []ProviderSetting{
				{
					Name:    "OpenAI",
					Type:    "openai",
					APIKey:  "",
					ApiKeys: []string{},
					BaseURL: "https://api.openai.com/v1",
				},
			},
			CurrentProvider: 0,
			Model:           "gpt-4o-mini",
			MaxTokens:       4096,
			Temperature:     1.0,
			Reasoning:       "medium",
			WorkingDir:      "",
			AgentSettings: AgentSettings{
				AutoLearn:   false,
				AutoCompact: true,
				SkillsDir:   filepath.Join(dataDir, "skills"),
			},
		},
	}
}

// startup is called by Wails when the app starts. It initializes the
// logger, registers providers, loads settings, and opens the memory store.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Initialize the logger early so subsequent steps are logged.
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		InitLogger(homeDir)
	}
	LogInfo("=== App starting, data dir: %s ===", a.dataDir)

	providers.RegisterBuiltInProviders()
	a.loadSettings()
	a.initKeyPool()
	mem, err := memory.New(filepath.Join(a.dataDir, "memory.json"))
	if err != nil {
		LogError("[memory] init error: %v", err)
	} else {
		a.mem = mem
		LogInfo("[memory] loaded %d entries", mem.Size())
	}
	skillsDir := a.settings.SkillsDir
	if skillsDir == "" {
		skillsDir = filepath.Join(a.dataDir, "skills")
	}
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		LogError("[app] failed to create skills dir %s: %v", skillsDir, err)
	}
	LogInfo("[app] initialized, data dir: %s", a.dataDir)
}

// initKeyPool initializes the API key pool from the current provider.
func (a *App) initKeyPool() {
	cp := a.settings.Current()
	if cp == nil {
		a.keyPool = keypool.New(nil, keypool.DefaultSettings())
		return
	}
	keys := cp.ApiKeys
	if len(keys) == 0 && cp.APIKey != "" {
		keys = []string{cp.APIKey}
	}
	a.keyPool = keypool.New(keys, keypool.DefaultSettings())
}

// selectAPIKey returns the next available API key from the pool.
func (a *App) selectAPIKey() string {
	key, err := a.keyPool.Next()
	if err != nil {
		return ""
	}
	return key
}

// markKeySuccess marks the current key as successful.
func (a *App) markKeySuccess() {
	a.keyPool.MarkSuccess()
}

// markKeyFailed marks the current key as failed.
func (a *App) markKeyFailed(err error) {
	a.keyPool.MarkFailed(keypool.CategorizeError(err))
}

// ─── Settings load/save ───

func (a *App) loadSettings() {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	data, err := os.ReadFile(a.settingsPath)
	if err != nil {
		return
	}
	var s AppSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return
	}
	a.settings = s
}

func (a *App) saveSettings() {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	data, err := json.MarshalIndent(a.settings, "", "  ")
	if err != nil {
		LogError("[settings] failed to marshal for SaveSettings: %v", err)
		return
	}
	if err := os.WriteFile(a.settingsPath, data, 0644); err != nil {
		LogError("[settings] failed to write settings file %s: %v", a.settingsPath, err)
	}
}

// ─── Settings API ───

func (a *App) GetSettings() (string, error) {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	data, err := json.Marshal(a.settings)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) SaveSettings(str string) error {
	var s AppSettings
	if err := json.Unmarshal([]byte(str), &s); err != nil {
		return err
	}
	// Validate: ensure currentProviderIndex is within bounds.
	if len(s.Providers) == 0 {
		s.CurrentProvider = 0
	} else if s.CurrentProvider < 0 || s.CurrentProvider >= len(s.Providers) {
		s.CurrentProvider = 0
	}
	a.settingsMu.Lock()
	oldModel := a.settings.Model
	newModel := s.Model
	a.settings = s
	a.settingsMu.Unlock()
	a.initKeyPool()
	if a.settings.SkillsDir != "" {
		_ = os.MkdirAll(a.settings.SkillsDir, 0755)
	}
	if oldModel != newModel || newModel == "" {
		modelID := newModel
		if modelID == "" {
			modelID = "gpt-4o-mini"
		}
		a.ctxSettings = contextmgr.DefaultSettings(modelID)
		// Clear per-conversation stats on model change.
		a.tokenStats = make(map[string]*contextmgr.TokenStats)
	}
	a.saveSettings()
	return nil
}

// SetAutoLearnEnabled toggles the auto-learn feature.
func (a *App) SetAutoLearnEnabled(enabled bool) error {
	a.settingsMu.Lock()
	a.settings.AutoLearn = enabled
	a.settingsMu.Unlock()
	a.saveSettings()
	return nil
}

// CancelStream cancels any in-flight stream/agent request.
func (a *App) CancelStream() {
	if a.cancelFn != nil {
		a.cancelFn()
		a.cancelFn = nil
	}
}

// getOrCreateTokenStats returns the TokenStats for the given conversation ID,
// creating one if it doesn't exist.
func (a *App) getOrCreateTokenStats(convID, modelID string) *contextmgr.TokenStats {
	if modelID == "" {
		a.settingsMu.RLock()
		modelID = a.settings.Model
		a.settingsMu.RUnlock()
		if modelID == "" {
			modelID = "gpt-4o-mini"
		}
	}
	if a.ctxSettings.MaxContextTokens == 0 {
		a.ctxSettings = contextmgr.DefaultSettings(modelID)
	}
	if convID == "" {
		convID = "_default"
	}
	ts, ok := a.tokenStats[convID]
	if !ok {
		ts = contextmgr.NewTokenStats(a.ctxSettings)
		a.tokenStats[convID] = ts
	}
	return ts
}

// ─── Context stats ───

func (a *App) GetContextStats() string {
	ts := a.getOrCreateTokenStats(a.currentConvID, "")
	stats := ts.Get()
	return contextmgr.FormatStats(stats)
}

func (a *App) GetCompactionStatus() string {
	ts := a.getOrCreateTokenStats(a.currentConvID, "")
	s := ts.Get()
	return contextmgr.FormatStats(s)
}

// getDataDir returns the app data directory (~/.pi-chat-app).
func getDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".pi-chat-app"
	}
	return filepath.Join(home, ".pi-chat-app")
}

// ─── WriteLog (frontend → backend log bridge) ───

// WriteLog is a Wails-exposed method that lets the frontend write log
// lines to the same daily-rotated log file.
func (a *App) WriteLog(level string, message string) error {
	if StdLogger == nil {
		return nil
	}
	switch upperLevel(level) {
	case "DEBUG":
		StdLogger.Debug("%s", message)
	case "INFO":
		StdLogger.Info("%s", message)
	case "WARN":
		StdLogger.Warn("%s", message)
	case "ERROR":
		StdLogger.Error("%s", message)
	default:
		StdLogger.Info("%s", message)
	}
	return nil
}

// upperLevel is a tiny helper to avoid importing strings just for this.
func upperLevel(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 32
		}
	}
	return string(b)
}
