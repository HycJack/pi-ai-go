package piai

import (
	"fmt"
	"sync"
)

// ImagesAPIProvider is the interface for image generation API providers.
type ImagesAPIProvider interface {
	GenerateImages(model ImagesModel, context Context, opts ImageOptions) (*AssistantImages, error)
}

var (
	imagesProviders   = make(map[KnownAPI]ImagesAPIProvider)
	imagesProviderSrc = make(map[string][]KnownAPI)
	imagesProvidersMu sync.RWMutex
)

// RegisterImagesProvider registers an image API provider.
func RegisterImagesProvider(api KnownAPI, provider ImagesAPIProvider, sourceID ...string) {
	imagesProvidersMu.Lock()
	defer imagesProvidersMu.Unlock()
	imagesProviders[api] = provider
	if len(sourceID) > 0 {
		imagesProviderSrc[sourceID[0]] = append(imagesProviderSrc[sourceID[0]], api)
	}
}

// GetImagesProvider returns the image API provider for the given API type.
func GetImagesProvider(api KnownAPI) (ImagesAPIProvider, error) {
	imagesProvidersMu.RLock()
	defer imagesProvidersMu.RUnlock()
	p, ok := imagesProviders[api]
	if !ok {
		return nil, fmt.Errorf("no images provider registered for API: %s", api)
	}
	return p, nil
}

// GetRegisteredImagesProviders returns all registered image API types.
func GetRegisteredImagesProviders() []KnownAPI {
	imagesProvidersMu.RLock()
	defer imagesProvidersMu.RUnlock()
	apis := make([]KnownAPI, 0, len(imagesProviders))
	for api := range imagesProviders {
		apis = append(apis, api)
	}
	return apis
}

// UnregisterImagesProviders removes all image providers registered with the given source ID.
func UnregisterImagesProviders(sourceID string) {
	imagesProvidersMu.Lock()
	defer imagesProvidersMu.Unlock()
	apis, ok := imagesProviderSrc[sourceID]
	if !ok {
		return
	}
	for _, api := range apis {
		delete(imagesProviders, api)
	}
	delete(imagesProviderSrc, sourceID)
}

// ClearImagesProviders removes all registered image providers.
func ClearImagesProviders() {
	imagesProvidersMu.Lock()
	defer imagesProvidersMu.Unlock()
	imagesProviders = make(map[KnownAPI]ImagesAPIProvider)
	imagesProviderSrc = make(map[string][]KnownAPI)
}
