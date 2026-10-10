// Package llm provides the LLM calling layer: public streaming/completion API,
// model management, and environment variable resolution.
package llm

import (
	"context"

	"github.com/HycJack/pi-ai-go/core"
)

// Stream starts a streaming completion request.
func Stream(ctx context.Context, model core.Model, msgs []core.Message, opts ...core.SimpleStreamOptions) (*core.EventStream[core.AssistantMessageEvent, core.AssistantMessage], error) {
	provider, err := core.GetProvider(model.API)
	if err != nil {
		return nil, err
	}

	var opt core.SimpleStreamOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	c, err := core.NormalizeContext(core.Context{Messages: msgs})
	if err != nil {
		return nil, err
	}
	return provider.StreamSimple(ctx, model, c, opt)
}

// Complete calls Stream and waits for the final result.
func Complete(ctx context.Context, model core.Model, msgs []core.Message, opts ...core.SimpleStreamOptions) (core.AssistantMessage, error) {
	s, err := Stream(ctx, model, msgs, opts...)
	if err != nil {
		return core.AssistantMessage{}, err
	}
	return s.Result()
}

// StreamSimple starts a streaming completion with simplified reasoning options.
func StreamSimple(ctx context.Context, model core.Model, msgs []core.Message, opts ...core.SimpleStreamOptions) (*core.EventStream[core.AssistantMessageEvent, core.AssistantMessage], error) {
	provider, err := core.GetProvider(model.API)
	if err != nil {
		return nil, err
	}

	var opt core.SimpleStreamOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	c, err := core.NormalizeContext(core.Context{Messages: msgs})
	if err != nil {
		return nil, err
	}
	return provider.StreamSimple(ctx, model, c, opt)
}

// StreamSimpleWithContext starts a streaming completion with full context (including tools and system prompt).
func StreamSimpleWithContext(ctx context.Context, model core.Model, llmCtx core.Context, opts ...core.SimpleStreamOptions) (*core.EventStream[core.AssistantMessageEvent, core.AssistantMessage], error) {
	provider, err := core.GetProvider(model.API)
	if err != nil {
		return nil, err
	}

	var opt core.SimpleStreamOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	norm, err := core.NormalizeContext(llmCtx)
	if err != nil {
		return nil, err
	}
	return provider.StreamSimple(ctx, model, norm, opt)
}

// CompleteSimple calls StreamSimple and waits for the final result.
func CompleteSimple(ctx context.Context, model core.Model, msgs []core.Message, opts ...core.SimpleStreamOptions) (core.AssistantMessage, error) {
	s, err := StreamSimple(ctx, model, msgs, opts...)
	if err != nil {
		return core.AssistantMessage{}, err
	}
	return s.Result()
}

// GenerateImages generates images using the specified image model.
func GenerateImages(ctx context.Context, model core.ImagesModel, msgs []core.Message, opts ...core.ImageOptions) (core.AssistantImages, error) {
	provider, err := core.GetImagesProvider(model.API)
	if err != nil {
		return core.AssistantImages{}, err
	}

	var opt core.ImageOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	c := core.Context{Messages: msgs}
	result, err := provider.GenerateImages(model, c, opt)
	if err != nil {
		return core.AssistantImages{}, err
	}
	return *result, nil
}
