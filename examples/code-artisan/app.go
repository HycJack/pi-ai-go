package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"code-artisan/internal/env"

	"pi-ai-go/core"
	"pi-ai-go/providers"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// NewApp creates a new App instance with default settings.
func NewApp() *App {
	dataDir := getDataDir()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		println("[app] failed to create data dir:", err.Error())
	}
	return &App{
		dataDir:          dataDir,
		settingsPath:     filepath.Join(dataDir, "settings.json"),
		envManager:       env.NewManager(),
		pythonRunner:     NewRunner(),
		artifactProgress: make(chan env.Progress, 64),
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

	// Initialize logger
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		InitLogger(homeDir)
	}
	LogInfo("=== Code Artisan starting, data dir: %s ===", a.dataDir)

	providers.RegisterBuiltInProviders()
	a.loadSettings()

	// Ensure embedded Python runtime is extracted
	if err := a.envManager.EnsureReady(a.reportProgress); err != nil {
		LogWarn("Embedded runtime not ready: %v", err)
	} else {
		LogInfo("Embedded Python runtime ready")
	}
}

func (a *App) reportProgress(p env.Progress) {
	LogInfo("[runtime progress] %s: %s (%d%%)", p.Stage, p.Message, p.Percent)
}

// ─── Python Runtime Status API ───

// GetPythonStatus returns the current runtime status as JSON.
func (a *App) GetPythonStatus() (string, error) {
	status, err := a.envManager.Status()
	if err != nil {
		return "", fmt.Errorf("get status: %w", err)
	}
	data, err := json.Marshal(status)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// RebuildPythonRuntime triggers re-extraction of the runtime archive.
func (a *App) RebuildPythonRuntime() (string, error) {
	if err := a.envManager.Rebuild(a.reportProgress); err != nil {
		return "", fmt.Errorf("rebuild: %w", err)
	}
	return a.GetPythonStatus()
}

// ─── Settings ───

// loadSettings loads settings from the JSON file.
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

// saveSettings persists settings to the JSON file.
func (a *App) saveSettings() {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	data, err := json.MarshalIndent(a.settings, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(a.settingsPath, data, 0644)
}

// GetSettings returns the current settings as JSON.
func (a *App) GetSettings() (string, error) {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	data, err := json.Marshal(a.settings)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveSettings updates settings from a JSON string.
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

// ─── Python Script Execution ───

// RunPythonScriptBackground runs a Python script using the embedded runtime.
func (a *App) RunPythonScriptBackground(jsonStr string) error {
	var req RunScriptRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		return fmt.Errorf("parse error: %v", err)
	}

	return a.pythonRunner.Run(req.Code, RunRequest{
		OnStart: func() {
			runtime.EventsEmit(a.ctx, "run-started", "")
		},
		OnFinish: func() {
			runtime.EventsEmit(a.ctx, "run-finished", "")
		},
		OnError: func(err error) {
			runtime.EventsEmit(a.ctx, "run-error", err.Error())
		},
	})
}

// ─── Helpers ───

func getDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".code-artisan"
	}
	return filepath.Join(home, ".code-artisan")
}

func (a *App) resolveModel(providerStr, modelID, baseURL string) core.Model {
	providerStr = toLower(providerStr)
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
	case "openai-compatible":
		provider = core.ProviderOpenAI
		api = core.APIOpenAICompletions
	default:
		provider = core.ProviderOpenAI
		api = core.APIOpenAICompletions
	}
	if modelID == "" {
		modelID = "auto"
	}
	model, _ := getModel(provider, modelID)
	if model.ID == "" {
		model = core.Model{
			ID:            modelID,
			Provider:      provider,
			API:           api,
			ContextWindow: 8192,
		}
	}
	if baseURL != "" {
		model.BaseURL = baseURL
	}
	return model
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

func getModel(provider core.KnownProvider, modelID string) (core.Model, error) {
	if modelID == "auto" || modelID == "" {
		return core.Model{
			ID:            "gpt-4o-mini",
			Provider:      provider,
			API:           core.APIOpenAICompletions,
			ContextWindow: 128000,
		}, nil
	}
	return core.Model{
		ID:            modelID,
		Provider:      provider,
		API:           core.APIOpenAICompletions,
		ContextWindow: 128000,
	}, nil
}
