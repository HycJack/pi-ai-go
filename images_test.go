package piai

import (
	"context"
	"testing"
)

func TestGenerateImagesNoProvider(t *testing.T) {
	ClearImagesProviders()
	defer ClearImagesProviders()

	model := ImagesModel{
		ID:       "test",
		API:      "nonexistent-api",
		Provider: ProviderOpenRouter,
	}

	_, err := GenerateImages(context.Background(), model, []Message{})
	if err == nil {
		t.Error("expected error for unregistered provider")
	}
}
