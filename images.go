package piai

import "context"

// GenerateImages generates images using the specified image model.
func GenerateImages(ctx context.Context, model ImagesModel, msgs []Message, opts ...ImageOptions) (AssistantImages, error) {
	provider, err := GetImagesProvider(model.API)
	if err != nil {
		return AssistantImages{}, err
	}

	var opt ImageOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	c := Context{Messages: msgs}
	result, err := provider.GenerateImages(model, c, opt)
	if err != nil {
		return AssistantImages{}, err
	}

	return *result, nil
}
