package piai

import (
	"fmt"
	"sync"
)

var (
	modelsMap = make(map[KnownProvider]map[string]Model)
	modelsMu  sync.RWMutex
)

// LoadModels loads models into the registry. Called during init with generated data.
func LoadModels(models map[KnownProvider]map[string]Model) {
	modelsMu.Lock()
	defer modelsMu.Unlock()
	modelsMap = models
}

// GetModel looks up a model by provider and model ID.
func GetModel(provider KnownProvider, modelID string) (Model, error) {
	modelsMu.RLock()
	defer modelsMu.RUnlock()

	providerModels, ok := modelsMap[provider]
	if !ok {
		return Model{}, fmt.Errorf("unknown provider: %s", provider)
	}

	model, ok := providerModels[modelID]
	if !ok {
		return Model{}, fmt.Errorf("unknown model: %s/%s", provider, modelID)
	}

	return model, nil
}

// GetProviders returns all registered provider names.
func GetProviders() []KnownProvider {
	modelsMu.RLock()
	defer modelsMu.RUnlock()

	providers := make([]KnownProvider, 0, len(modelsMap))
	for p := range modelsMap {
		providers = append(providers, p)
	}
	return providers
}

// GetModels returns all models for a given provider.
func GetModels(provider KnownProvider) []Model {
	modelsMu.RLock()
	defer modelsMu.RUnlock()

	providerModels, ok := modelsMap[provider]
	if !ok {
		return nil
	}

	models := make([]Model, 0, len(providerModels))
	for _, m := range providerModels {
		models = append(models, m)
	}
	return models
}

// CalculateCost computes the cost of a request from per-million-token rates.
func CalculateCost(model Model, usage Usage) CostBreakdown {
	inputCost := float64(usage.Input) * model.Cost.Input / 1_000_000
	outputCost := float64(usage.Output) * model.Cost.Output / 1_000_000
	cacheReadCost := float64(usage.CacheRead) * model.Cost.CacheRead / 1_000_000
	cacheWriteCost := float64(usage.CacheWrite) * model.Cost.CacheWrite / 1_000_000

	return CostBreakdown{
		Input:      inputCost,
		Output:     outputCost,
		CacheRead:  cacheReadCost,
		CacheWrite: cacheWriteCost,
		Total:      inputCost + outputCost + cacheReadCost + cacheWriteCost,
	}
}

// GetSupportedThinkingLevels returns the thinking levels supported by a model.
func GetSupportedThinkingLevels(model Model) []ThinkingLevel {
	if !model.Reasoning {
		return nil
	}

	if model.ThinkingLevelMap != nil {
		levels := make([]ThinkingLevel, 0, len(model.ThinkingLevelMap))
		for level := range model.ThinkingLevelMap {
			levels = append(levels, ThinkingLevel(level))
		}
		return levels
	}

	// Default levels for reasoning models
	return []ThinkingLevel{ThinkingLow, ThinkingMedium, ThinkingHigh}
}

// ClampThinkingLevel finds the closest supported thinking level.
func ClampThinkingLevel(model Model, level ThinkingLevel) ThinkingLevel {
	supported := GetSupportedThinkingLevels(model)
	if len(supported) == 0 {
		return ThinkingMedium
	}

	// Check if exact match
	for _, s := range supported {
		if s == level {
			return level
		}
	}

	// Find closest
	levelOrder := []ThinkingLevel{ThinkingMinimal, ThinkingLow, ThinkingMedium, ThinkingHigh, ThinkingXHigh}
	targetIdx := -1
	for i, l := range levelOrder {
		if l == level {
			targetIdx = i
			break
		}
	}

	bestIdx := -1
	bestDist := 100
	for _, s := range supported {
		for i, l := range levelOrder {
			if l == s {
				dist := abs(i - targetIdx)
				if dist < bestDist {
					bestDist = dist
					bestIdx = i
				}
			}
		}
	}

	if bestIdx >= 0 {
		return levelOrder[bestIdx]
	}

	return supported[0]
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ModelsAreEqual compares two models by id and provider.
func ModelsAreEqual(a, b Model) bool {
	return a.ID == b.ID && a.Provider == b.Provider
}
