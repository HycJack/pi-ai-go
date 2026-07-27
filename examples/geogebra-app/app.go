package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"pi-ai-go/core"
	"pi-ai-go/llm"
	"pi-ai-go/providers"
)

// This file contains the core app setup.
// Functionality is split into:
//   types.go       — struct definitions
//   settings.go    — App lifecycle, settings load/save
//   geogebra.go    — GeogebraMessage (generate+stream)
//   models.go      — model list fetching
//   logger.go      — file logger

// NewApp creates a new App instance with default settings.
func NewApp() *App {
	dataDir := getDataDir()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		LogError("[app] failed to create data dir %s: %v", dataDir, err)
	}
	return &App{
		dataDir: dataDir,
		settings: AppSettings{
			Providers: []ProviderSetting{
				{
					Name:    "OpenAI",
					Type:    "openai",
					APIKey:  "",
					BaseURL: "https://api.openai.com/v1",
				},
			},
			CurrentProvider: 0,
			Model:           "gpt-4o-mini",
			MaxTokens:       4096,
			Temperature:     1.0,
		},
	}
}

// startup is called by Wails when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		InitLogger(homeDir)
	}
	LogInfo("=== GeoGebra App starting, data dir: %s ===", a.dataDir)

	providers.RegisterBuiltInProviders()
	a.loadSettings()
}

// getDataDir returns the app data directory.
func getDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".geogebra-app"
	}
	return filepath.Join(home, ".geogebra-app")
}

// ─── Settings ───

func (a *App) loadSettings() {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	data, err := os.ReadFile(filepath.Join(a.dataDir, "settings.json"))
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
		LogError("[settings] failed to marshal: %v", err)
		return
	}
	if err := os.WriteFile(filepath.Join(a.dataDir, "settings.json"), data, 0644); err != nil {
		LogError("[settings] failed to write: %v", err)
	}
}

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
	if len(s.Providers) == 0 {
		s.CurrentProvider = 0
	} else if s.CurrentProvider < 0 || s.CurrentProvider >= len(s.Providers) {
		s.CurrentProvider = 0
	}
	a.settingsMu.Lock()
	a.settings = s
	a.settingsMu.Unlock()
	a.saveSettings()
	return nil
}

// CancelStream cancels any in-flight request.
func (a *App) CancelStream() {
	if a.cancelFn != nil {
		a.cancelFn()
		a.cancelFn = nil
	}
}

// resolveModel maps provider string + model ID to a core.Model.
func (a *App) resolveModel(providerStr, modelID, baseURL string) core.Model {
	providerStr = strings.ToLower(providerStr)
	var provider core.KnownProvider
	var api core.KnownAPI
	switch providerStr {
	case "anthropic":
		provider = core.ProviderAnthropic
		api = core.APIAnthropicMessages
	case "google":
		provider = core.ProviderGoogle
		api = core.APIGoogleGenerative
	case "deepseek":
		provider = core.ProviderDeepSeek
		api = core.APIOpenAICompletions
	case "mistral":
		provider = core.ProviderMistral
		api = core.APIMistralConversations
	default:
		provider = core.ProviderOpenAI
		api = core.APIOpenAICompletions
	}
	if modelID == "" {
		modelID = "auto"
	}
	model, err := llm.GetModel(provider, modelID)
	if err != nil {
		if provider != core.ProviderOpenAI && providerStr != "openai" {
			if m, err2 := llm.GetModel(core.ProviderOpenAI, modelID); err2 == nil {
				model = m
				err = nil
			}
		}
		if err != nil {
			LogWarn("[model] %v, using fallback model %s/%s", err, provider, modelID)
			model = core.Model{
				ID:            modelID,
				Provider:      provider,
				API:           api,
				ContextWindow: 8192,
			}
		}
	}
	if baseURL != "" {
		model.BaseURL = baseURL
	}
	LogDebug("[model] resolved: id=%s provider=%s api=%s baseURL=%s ctxWindow=%d",
		model.ID, model.Provider, model.API, model.BaseURL, model.ContextWindow)
	return model
}

// ListDirectory returns a JSON array of file entries.
func (a *App) ListDirectory(dir string) (string, error) {
	if dir == "" {
		dir = a.dataDir
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "[]", err
	}
	type entry struct {
		Name  string `json:"name"`
		IsDir bool   `json:"isDir"`
		Size  int64  `json:"size,omitempty"`
	}
	out := make([]entry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		sz := int64(0)
		if err == nil {
			sz = info.Size()
		}
		out = append(out, entry{Name: e.Name(), IsDir: e.IsDir(), Size: sz})
	}
	data, _ := json.Marshal(out)
	return string(data), nil
}

// WriteLog is a Wails-exposed method for frontend log bridge.
func (a *App) WriteLog(level string, message string) error {
	if StdLogger == nil {
		return nil
	}
	switch strings.ToUpper(level) {
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
