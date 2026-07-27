package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDataDir(t *testing.T) {
	dir := getDataDir()
	if dir == "" {
		t.Fatal("getDataDir() returned empty string")
	}
	home, err := os.UserHomeDir()
	if err == nil {
		expected := filepath.Join(home, ".pi-chat-app")
		if dir != expected {
			t.Errorf("getDataDir() = %q, want %q", dir, expected)
		}
	}
}

func TestUpperLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"debug", "DEBUG"},
		{"info", "INFO"},
		{"warn", "WARN"},
		{"error", "ERROR"},
		{"DEBUG", "DEBUG"},
		{"Info", "INFO"},
		{"", ""},
		{"mixedCASE", "MIXEDCASE"},
	}
	for _, tc := range tests {
		got := upperLevel(tc.input)
		if got != tc.want {
			t.Errorf("upperLevel(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestNewAppDefaults(t *testing.T) {
	// NewApp creates sensible defaults — notably providers[0] should be "openai".
	// We can't easily test NewApp() without a real home dir since it calls
	// os.MkdirAll, but we can at least assert the defaults are well-formed.
	app := NewApp()
	if app == nil {
		t.Fatal("NewApp() returned nil")
	}
	if app.dataDir == "" {
		t.Error("NewApp().dataDir should not be empty")
	}
	if app.settingsPath == "" {
		t.Error("NewApp().settingsPath should not be empty")
	}
	if len(app.settings.Providers) == 0 {
		t.Error("NewApp() should have at least one provider")
	}
	if app.settings.Providers[0].Type != "openai" {
		t.Errorf("expected default provider type 'openai', got %q", app.settings.Providers[0].Type)
	}
	if app.settings.Model != "gpt-4o-mini" {
		t.Errorf("expected default model 'gpt-4o-mini', got %q", app.settings.Model)
	}
	if app.tokenStats == nil {
		t.Error("NewApp().tokenStats should be initialized")
	}
}

func TestSaveSettingsProviderBounds(t *testing.T) {
	app := NewApp()
	// SaveSettings with empty providers should reset CurrentProvider to 0.
	jsonStr := `{"providers":[],"currentProvider":5,"model":"gpt-4o"}`
	err := app.SaveSettings(jsonStr)
	if err != nil {
		t.Fatalf("SaveSettings() error: %v", err)
	}
	app.settingsMu.RLock()
	if app.settings.CurrentProvider != 0 {
		t.Errorf("expected CurrentProvider=0 after reset, got %d", app.settings.CurrentProvider)
	}
	app.settingsMu.RUnlock()
}
