package piai

import (
	"testing"
)

func setupTestImageModels() {
	LoadImageModels(map[KnownProvider]map[string]ImagesModel{
		ProviderOpenRouter: {
			"flux-pro": {
				ID:       "flux-pro",
				Name:     "Flux Pro",
				API:      "openrouter-images",
				Provider: ProviderOpenRouter,
				Input:    []Modality{ModalityText},
				Output:   []Modality{ModalityImage},
				Cost:     Cost{Output: 0.05},
			},
		},
	})
}

func TestGetImageModel(t *testing.T) {
	setupTestImageModels()

	m, err := GetImageModel(ProviderOpenRouter, "flux-pro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.ID != "flux-pro" {
		t.Errorf("expected flux-pro, got %s", m.ID)
	}
}

func TestGetImageModelNotFound(t *testing.T) {
	setupTestImageModels()

	_, err := GetImageModel(ProviderOpenRouter, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent model")
	}
}

func TestGetImageProviders(t *testing.T) {
	setupTestImageModels()

	providers := GetImageProviders()
	if len(providers) == 0 {
		t.Error("expected non-empty providers")
	}
}

func TestGetImageModels(t *testing.T) {
	setupTestImageModels()

	models := GetImageModels(ProviderOpenRouter)
	if len(models) != 1 {
		t.Errorf("expected 1 model, got %d", len(models))
	}
}
