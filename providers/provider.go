// Package providers defines the provider interface and shared utilities.
package providers

import piai "pi-ai-go"

// Provider is the interface that all API providers must implement.
type Provider interface {
	// Stream starts a streaming completion request.
	Stream(model piai.Model, context piai.Context, opts piai.StreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error)
	// StreamSimple starts a streaming completion with simplified options.
	StreamSimple(model piai.Model, context piai.Context, opts piai.SimpleStreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error)
}
