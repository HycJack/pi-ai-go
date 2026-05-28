package piai

import (
	"fmt"
	"sync"
)

var (
	imageModelsMap = make(map[KnownProvider]map[string]ImagesModel)
	imageModelsMu  sync.RWMutex
)

// LoadImageModels loads image models into the registry.
func LoadImageModels(models map[KnownProvider]map[string]ImagesModel) {
	imageModelsMu.Lock()
	defer imageModelsMu.Unlock()
	imageModelsMap = models
}

// GetImageModel looks up an image model by provider and model ID.
func GetImageModel(provider KnownProvider, modelID string) (ImagesModel, error) {
	imageModelsMu.RLock()
	defer imageModelsMu.RUnlock()

	providerModels, ok := imageModelsMap[provider]
	if !ok {
		return ImagesModel{}, fmt.Errorf("unknown image provider: %s", provider)
	}

	model, ok := providerModels[modelID]
	if !ok {
		return ImagesModel{}, fmt.Errorf("unknown image model: %s/%s", provider, modelID)
	}

	return model, nil
}

// GetImageProviders returns all registered image provider names.
func GetImageProviders() []KnownProvider {
	imageModelsMu.RLock()
	defer imageModelsMu.RUnlock()

	providers := make([]KnownProvider, 0, len(imageModelsMap))
	for p := range imageModelsMap {
		providers = append(providers, p)
	}
	return providers
}

// GetImageModels returns all image models for a given provider.
func GetImageModels(provider KnownProvider) []ImagesModel {
	imageModelsMu.RLock()
	defer imageModelsMu.RUnlock()

	providerModels, ok := imageModelsMap[provider]
	if !ok {
		return nil
	}

	models := make([]ImagesModel, 0, len(providerModels))
	for _, m := range providerModels {
		models = append(models, m)
	}
	return models
}
