// Package oauth provides OAuth authentication flows for AI providers.
package oauth

import "context"

// Credentials holds OAuth tokens.
type Credentials struct {
	Refresh  string `json:"refresh"`
	Access   string `json:"access"`
	Expires  int64  `json:"expires"` // Unix timestamp
	Extra    map[string]any `json:"extra,omitempty"`
}

// ProviderInterface defines the interface for an OAuth provider.
type ProviderInterface struct {
	ID           string
	Name         string
	Login        func(ctx context.Context, callbacks LoginCallbacks) (Credentials, error)
	RefreshToken func(ctx context.Context, credentials Credentials) (Credentials, error)
	GetAPIKey    func(ctx context.Context, credentials Credentials) (string, error)
}

// LoginCallbacks provides callbacks for the OAuth login flow.
type LoginCallbacks struct {
	OnAuth      func(url string)
	OnDeviceCode func(deviceCode string, verificationURI string)
	OnPrompt    func(message string)
	OnProgress  func(message string)
	Signal      <-chan struct{}
}
