package oauth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

var (
	providers = make(map[string]*ProviderInterface)
	mu        sync.RWMutex
)

// Register registers an OAuth provider.
func Register(provider *ProviderInterface) {
	mu.Lock()
	defer mu.Unlock()
	providers[provider.ID] = provider
}

// Unregister removes an OAuth provider.
func Unregister(id string) {
	mu.Lock()
	defer mu.Unlock()
	delete(providers, id)
}

// Reset removes all OAuth providers.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	providers = make(map[string]*ProviderInterface)
}

// Get returns an OAuth provider by ID.
func Get(id string) (*ProviderInterface, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := providers[id]
	if !ok {
		return nil, fmt.Errorf("oauth provider not found: %s", id)
	}
	return p, nil
}

// List returns all registered OAuth provider IDs.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	ids := make([]string, 0, len(providers))
	for id := range providers {
		ids = append(ids, id)
	}
	return ids
}

// GetAPIKey returns a valid API key, refreshing the token if expired.
func GetAPIKey(ctx context.Context, providerID string, credentials Credentials) (string, error) {
	p, err := Get(providerID)
	if err != nil {
		return "", err
	}

	// Check if token is expired (with 5 minute buffer)
	if credentials.Expires > 0 && time.Now().Unix() > credentials.Expires-300 {
		if p.RefreshToken != nil {
			credentials, err = p.RefreshToken(ctx, credentials)
			if err != nil {
				return "", fmt.Errorf("failed to refresh token: %w", err)
			}
		}
	}

	if p.GetAPIKey != nil {
		return p.GetAPIKey(ctx, credentials)
	}

	return credentials.Access, nil
}

func init() {
	// Register built-in OAuth providers
	Register(&ProviderInterface{
		ID:   "anthropic",
		Name: "Anthropic",
		Login: func(ctx context.Context, callbacks LoginCallbacks) (Credentials, error) {
			return loginAnthropic(ctx, callbacks)
		},
		RefreshToken: func(ctx context.Context, credentials Credentials) (Credentials, error) {
			return refreshAnthropic(ctx, credentials)
		},
		GetAPIKey: func(ctx context.Context, credentials Credentials) (string, error) {
			return credentials.Access, nil
		},
	})

	Register(&ProviderInterface{
		ID:   "github-copilot",
		Name: "GitHub Copilot",
		Login: func(ctx context.Context, callbacks LoginCallbacks) (Credentials, error) {
			return loginGitHubCopilot(ctx, callbacks)
		},
		RefreshToken: func(ctx context.Context, credentials Credentials) (Credentials, error) {
			return refreshGitHubCopilot(ctx, credentials)
		},
		GetAPIKey: func(ctx context.Context, credentials Credentials) (string, error) {
			return credentials.Access, nil
		},
	})

	Register(&ProviderInterface{
		ID:   "openai-codex",
		Name: "OpenAI Codex",
		Login: func(ctx context.Context, callbacks LoginCallbacks) (Credentials, error) {
			return loginOpenAICodex(ctx, callbacks)
		},
		RefreshToken: func(ctx context.Context, credentials Credentials) (Credentials, error) {
			return refreshOpenAICodex(ctx, credentials)
		},
		GetAPIKey: func(ctx context.Context, credentials Credentials) (string, error) {
			return credentials.Access, nil
		},
	})
}
