package piai

import (
	"testing"
)

type mockImagesProvider struct{}

func (m *mockImagesProvider) GenerateImages(model ImagesModel, context Context, opts ImageOptions) (*AssistantImages, error) {
	return &AssistantImages{
		API:       model.API,
		Provider:  model.Provider,
		Model:     model.ID,
		StopReason: StopStop,
	}, nil
}

func TestRegisterAndGetImagesProvider(t *testing.T) {
	ClearImagesProviders()
	defer ClearImagesProviders()

	p := &mockImagesProvider{}
	RegisterImagesProvider("test-images", p, "test")

	got, err := GetImagesProvider("test-images")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != p {
		t.Error("expected same provider")
	}
}

func TestGetImagesProviderNotFound(t *testing.T) {
	ClearImagesProviders()
	defer ClearImagesProviders()

	_, err := GetImagesProvider("nonexistent")
	if err == nil {
		t.Error("expected error for unregistered provider")
	}
}

func TestGetRegisteredImagesProviders(t *testing.T) {
	ClearImagesProviders()
	defer ClearImagesProviders()

	RegisterImagesProvider("test1", &mockImagesProvider{}, "test")
	RegisterImagesProvider("test2", &mockImagesProvider{}, "test")

	providers := GetRegisteredImagesProviders()
	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}
}

func TestClearImagesProviders(t *testing.T) {
	RegisterImagesProvider("test", &mockImagesProvider{}, "test")
	ClearImagesProviders()

	providers := GetRegisteredImagesProviders()
	if len(providers) != 0 {
		t.Errorf("expected 0 providers, got %d", len(providers))
	}
}
