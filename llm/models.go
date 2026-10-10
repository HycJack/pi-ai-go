package llm

import (
	"fmt"
	"strings"
	"sync"

	"github.com/HycJack/pi-ai-go/core"
)

var (
	modelsMap = make(map[core.KnownProvider]map[string]core.Model)
	modelsMu  sync.RWMutex
)

// LoadModels loads models into the registry. Called during init with generated data.
func LoadModels(models map[core.KnownProvider]map[string]core.Model) {
	modelsMu.Lock()
	defer modelsMu.Unlock()
	modelsMap = models
}

// GetModel looks up a model by provider and model ID.
func GetModel(provider core.KnownProvider, modelID string) (core.Model, error) {
	modelsMu.RLock()
	defer modelsMu.RUnlock()

	providerModels, ok := modelsMap[provider]
	if !ok {
		return core.Model{}, fmt.Errorf("unknown provider: %s", provider)
	}

	model, ok := providerModels[modelID]
	if !ok {
		return core.Model{}, fmt.Errorf("unknown model: %s/%s", provider, modelID)
	}

	return model, nil
}

// GetProviders returns all registered provider names.
func GetProviders() []core.KnownProvider {
	modelsMu.RLock()
	defer modelsMu.RUnlock()

	providers := make([]core.KnownProvider, 0, len(modelsMap))
	for p := range modelsMap {
		providers = append(providers, p)
	}
	return providers
}

// GetModels returns all models for a given provider.
func GetModels(provider core.KnownProvider) []core.Model {
	modelsMu.RLock()
	defer modelsMu.RUnlock()

	providerModels, ok := modelsMap[provider]
	if !ok {
		return nil
	}

	models := make([]core.Model, 0, len(providerModels))
	for _, m := range providerModels {
		models = append(models, m)
	}
	return models
}

// ListModels returns all models for a given provider. It is an alias of
// GetModels retained for symmetry with the model-catalog query API.
func ListModels(provider core.KnownProvider) []core.Model {
	return GetModels(provider)
}

// LookupModelExact resolves a model ref of the form "provider/id" with no
// interpretation or fallback. The provider and id are both required and must
// be non-empty. Returns an error for a malformed ref, an unknown provider, or
// an unknown model.
// || 严格解析 "provider/id" 形式的模型引用，不做任何解释或回退。
// || provider 与 id 都必填且非空；格式错误、未知 provider、未知模型均返回错误。
func LookupModelExact(ref string) (core.Model, error) {
	provider, id, err := splitModelRef(ref)
	if err != nil {
		return core.Model{}, err
	}
	return GetModel(provider, id)
}

// LookupModel resolves a model reference. It accepts either the full
// "provider/id" form or a bare id searched across all registered providers. A
// bare id is resolved only when it is unambiguous; an id that maps to more
// than one provider is an error. Returns an error when nothing matches.
// || 解析模型引用：接受完整的 "provider/id" 形式，或跨所有 provider 搜索的裸 id。
// || 裸 id 仅在无歧义时解析；命中多个 provider 时报错。无匹配时返回错误。
func LookupModel(ref string) (core.Model, error) {
	before, _, hasProvider := strings.Cut(ref, "/")
	if hasProvider {
		return LookupModelExact(ref)
	}
	// No slash: the whole ref is a bare id to search across providers.
	id := before
	modelsMu.RLock()
	defer modelsMu.RUnlock()
	var found core.Model
	var foundProvider core.KnownProvider
	for p, pm := range modelsMap {
		m, ok := pm[id]
		if !ok {
			continue
		}
		if foundProvider != "" {
			return core.Model{}, fmt.Errorf("ambiguous model id %q (providers %s and %s)", id, foundProvider, p)
		}
		found, foundProvider = m, p
	}
	if foundProvider == "" {
		return core.Model{}, fmt.Errorf("unknown model: %s", id)
	}
	return found, nil
}

// splitModelRef splits "provider/id" into its two non-empty parts.
func splitModelRef(ref string) (core.KnownProvider, string, error) {
	provider, id, ok := strings.Cut(ref, "/")
	if !ok || provider == "" || id == "" {
		return "", "", fmt.Errorf("malformed model ref %q: want provider/id", ref)
	}
	return core.KnownProvider(provider), id, nil
}

// GetSupportedThinkingLevels returns the thinking levels supported by a model.
func GetSupportedThinkingLevels(model core.Model) []core.ThinkingLevel {
	if !model.Reasoning {
		return nil
	}

	if model.ThinkingLevelMap != nil {
		levels := make([]core.ThinkingLevel, 0, len(model.ThinkingLevelMap))
		for level := range model.ThinkingLevelMap {
			levels = append(levels, core.ThinkingLevel(level))
		}
		return levels
	}

	return []core.ThinkingLevel{core.ThinkingLow, core.ThinkingMedium, core.ThinkingHigh}
}

// ClampThinkingLevel finds the closest supported thinking level.
func ClampThinkingLevel(model core.Model, level core.ThinkingLevel) core.ThinkingLevel {
	supported := GetSupportedThinkingLevels(model)
	if len(supported) == 0 {
		return core.ThinkingMedium
	}

	for _, s := range supported {
		if s == level {
			return level
		}
	}

	levelOrder := []core.ThinkingLevel{core.ThinkingMinimal, core.ThinkingLow, core.ThinkingMedium, core.ThinkingHigh, core.ThinkingXHigh}
	targetIdx := -1
	for i, l := range levelOrder {
		if l == level {
			targetIdx = i
			break
		}
	}

	if targetIdx < 0 {
		targetIdx = 2
	}

	bestIdx := -1
	bestDist := len(levelOrder) + 1
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
func ModelsAreEqual(a, b core.Model) bool {
	return a.ID == b.ID && a.Provider == b.Provider
}

// --- Image Model Registry ---

var (
	imageModelsMap = make(map[core.KnownProvider]map[string]core.ImagesModel)
	imageModelsMu  sync.RWMutex
)

// LoadImageModels loads image models into the registry.
func LoadImageModels(models map[core.KnownProvider]map[string]core.ImagesModel) {
	imageModelsMu.Lock()
	defer imageModelsMu.Unlock()
	imageModelsMap = models
}

// GetImageModel looks up an image model by provider and model ID.
func GetImageModel(provider core.KnownProvider, modelID string) (core.ImagesModel, error) {
	imageModelsMu.RLock()
	defer imageModelsMu.RUnlock()

	providerModels, ok := imageModelsMap[provider]
	if !ok {
		return core.ImagesModel{}, fmt.Errorf("unknown image provider: %s", provider)
	}

	model, ok := providerModels[modelID]
	if !ok {
		return core.ImagesModel{}, fmt.Errorf("unknown image model: %s/%s", provider, modelID)
	}

	return model, nil
}

// GetImageProviders returns all registered image provider names.
func GetImageProviders() []core.KnownProvider {
	imageModelsMu.RLock()
	defer imageModelsMu.RUnlock()

	providers := make([]core.KnownProvider, 0, len(imageModelsMap))
	for p := range imageModelsMap {
		providers = append(providers, p)
	}
	return providers
}

// GetImageModels returns all image models for a given provider.
func GetImageModels(provider core.KnownProvider) []core.ImagesModel {
	imageModelsMu.RLock()
	defer imageModelsMu.RUnlock()

	providerModels, ok := imageModelsMap[provider]
	if !ok {
		return nil
	}

	models := make([]core.ImagesModel, 0, len(providerModels))
	for _, m := range providerModels {
		models = append(models, m)
	}
	return models
}

// ModelCapabilities summarizes the capabilities derivable from a Model.
// || 从 Model 派生出的能力摘要
type ModelCapabilities struct {
	Text           bool // 支持文本输入
	Vision         bool // 支持图像输入
	Audio          bool // 支持音频输入
	Reasoning      bool // 支持推理（thinking）
}

// ToCapabilities derives a ModelCapabilities summary from a model's modalities
// and reasoning flag.
// || 从模型的模态与推理标志派生能力摘要
func ToCapabilities(model core.Model) ModelCapabilities {
	var caps ModelCapabilities
	for _, mod := range model.Input {
		switch mod {
		case core.ModalityText:
			caps.Text = true
		case core.ModalityImage:
			caps.Vision = true
		case core.ModalityAudio:
			caps.Audio = true
		}
	}
	caps.Reasoning = model.Reasoning
	return caps
}
