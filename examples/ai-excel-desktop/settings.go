package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"pi-ai-go/providers"
)

// NewApp 创建一个带默认设置的 App 实例。
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

// startup 在 Wails 启动时调用：初始化日志、注册 provider、加载设置。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		InitLogger(homeDir)
	}
	LogInfo("=== AI Excel App starting, data dir: %s ===", a.dataDir)

	providers.RegisterBuiltInProviders()
	a.loadSettings()
}

// getDataDir 返回应用数据目录 (~/.ai-excel-desktop)。
func getDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".ai-excel-desktop"
	}
	return filepath.Join(home, ".ai-excel-desktop")
}

// ─── 设置加载/保存 ───

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

// GetSettings 返回当前设置的 JSON 字符串。
func (a *App) GetSettings() (string, error) {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	data, err := json.Marshal(a.settings)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveSettings 接收 JSON 字符串并保存设置。
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

// CancelStream 取消正在进行的 agent 请求。
func (a *App) CancelStream() {
	if a.cancelFn != nil {
		a.cancelFn()
		a.cancelFn = nil
	}
}

// WriteLog 是 Wails 暴露给前端的方法，把前端日志写入同一个日志文件。
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
