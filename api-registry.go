package piai

import (
	"fmt"
	"sync"
)

// APIProvider is the interface for text generation API providers.
type APIProvider interface {
	// Stream starts a streaming completion request.
	Stream(model Model, context Context, opts StreamOptions) (*AssistantMessageEventStream, error)
	// StreamSimple starts a streaming completion with simplified options.
	StreamSimple(model Model, context Context, opts SimpleStreamOptions) (*AssistantMessageEventStream, error)
}

// AssistantMessageEventStream is a type alias for the event stream.
type AssistantMessageEventStream = EventStream[AssistantMessageEvent, AssistantMessage]

// EventStream is defined in utils/eventstream but we need a local alias for the provider interface.
// The actual implementation is in the eventstream package.

var (
	apiProviders   = make(map[KnownAPI]APIProvider)
	apiProviderSrc = make(map[string][]KnownAPI) // sourceID -> apis
	apiProvidersMu sync.RWMutex
)

// RegisterProvider registers an API provider for the given API type.
func RegisterProvider(api KnownAPI, provider APIProvider, sourceID ...string) {
	apiProvidersMu.Lock()
	defer apiProvidersMu.Unlock()
	apiProviders[api] = provider
	if len(sourceID) > 0 {
		apiProviderSrc[sourceID[0]] = append(apiProviderSrc[sourceID[0]], api)
	}
}

// GetProvider returns the API provider for the given API type.
func GetProvider(api KnownAPI) (APIProvider, error) {
	apiProvidersMu.RLock()
	defer apiProvidersMu.RUnlock()
	p, ok := apiProviders[api]
	if !ok {
		return nil, fmt.Errorf("no provider registered for API: %s", api)
	}
	return p, nil
}

// GetRegisteredProviders returns all registered API types.
func GetRegisteredProviders() []KnownAPI {
	apiProvidersMu.RLock()
	defer apiProvidersMu.RUnlock()
	apis := make([]KnownAPI, 0, len(apiProviders))
	for api := range apiProviders {
		apis = append(apis, api)
	}
	return apis
}

// UnregisterProviders removes all providers registered with the given source ID.
func UnregisterProviders(sourceID string) {
	apiProvidersMu.Lock()
	defer apiProvidersMu.Unlock()
	apis, ok := apiProviderSrc[sourceID]
	if !ok {
		return
	}
	for _, api := range apis {
		delete(apiProviders, api)
	}
	delete(apiProviderSrc, sourceID)
}

// ClearProviders removes all registered providers.
func ClearProviders() {
	apiProvidersMu.Lock()
	defer apiProvidersMu.Unlock()
	apiProviders = make(map[KnownAPI]APIProvider)
	apiProviderSrc = make(map[string][]KnownAPI)
}
