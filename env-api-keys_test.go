package piai

import (
	"os"
	"testing"
)

func TestGetEnvAPIKey(t *testing.T) {
	// Set a test env var
	os.Setenv("TEST_API_KEY", "test-value-123")
	defer os.Unsetenv("TEST_API_KEY")

	// Temporarily add to the map
	original := providerEnvVars["test-provider"]
	providerEnvVars["test-provider"] = []string{"TEST_API_KEY"}
	defer func() {
		if original != nil {
			providerEnvVars["test-provider"] = original
		} else {
			delete(providerEnvVars, "test-provider")
		}
	}()

	key := GetEnvAPIKey("test-provider")
	if key != "test-value-123" {
		t.Errorf("expected 'test-value-123', got '%s'", key)
	}
}

func TestGetEnvAPIKeyNotFound(t *testing.T) {
	key := GetEnvAPIKey("nonexistent-provider")
	if key != "" {
		t.Errorf("expected empty string, got '%s'", key)
	}
}

func TestFindEnvKeys(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	found := FindEnvKeys(ProviderAnthropic)
	if len(found) == 0 {
		t.Error("expected to find at least one env key")
	}

	foundAnthropic := false
	for _, k := range found {
		if k == "ANTHROPIC_API_KEY" {
			foundAnthropic = true
			break
		}
	}
	if !foundAnthropic {
		t.Error("expected to find ANTHROPIC_API_KEY")
	}
}

func TestResolveAPIKey(t *testing.T) {
	// With explicit key
	key := ResolveAPIKey(ProviderAnthropic, "explicit-key")
	if key != "explicit-key" {
		t.Errorf("expected 'explicit-key', got '%s'", key)
	}
}

func TestResolveBaseURL(t *testing.T) {
	tests := []struct {
		model    Model
		fallback string
		want     string
	}{
		{
			model:    Model{BaseURL: "https://custom.api.com/v1/"},
			fallback: "https://default.api.com",
			want:     "https://custom.api.com/v1",
		},
		{
			model:    Model{},
			fallback: "https://default.api.com/",
			want:     "https://default.api.com",
		},
	}

	for _, tt := range tests {
		got := ResolveBaseURL(tt.model, tt.fallback)
		if got != tt.want {
			t.Errorf("ResolveBaseURL(%+v, %s) = %s, want %s", tt.model, tt.fallback, got, tt.want)
		}
	}
}
