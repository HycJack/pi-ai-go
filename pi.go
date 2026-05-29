package piai

import "context"

// Stream starts a streaming completion request.
func Stream(ctx context.Context, model Model, msgs []Message, opts ...StreamOptions) (*EventStream[AssistantMessageEvent, AssistantMessage], error) {
	provider, err := GetProvider(model.API)
	if err != nil {
		return nil, err
	}

	var opt StreamOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	c := Context{
		Messages: msgs,
	}

	return provider.Stream(model, c, opt)
}

// Complete calls Stream and waits for the final result.
func Complete(ctx context.Context, model Model, msgs []Message, opts ...StreamOptions) (AssistantMessage, error) {
	s, err := Stream(ctx, model, msgs, opts...)
	if err != nil {
		return AssistantMessage{}, err
	}

	return s.Result()
}

// StreamSimple starts a streaming completion with simplified reasoning options.
func StreamSimple(ctx context.Context, model Model, msgs []Message, opts ...SimpleStreamOptions) (*EventStream[AssistantMessageEvent, AssistantMessage], error) {
	provider, err := GetProvider(model.API)
	if err != nil {
		return nil, err
	}

	var opt SimpleStreamOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	c := Context{
		Messages: msgs,
	}

	return provider.StreamSimple(model, c, opt)
}

// StreamSimpleWithContext starts a streaming completion with full context (including tools and system prompt).
func StreamSimpleWithContext(ctx context.Context, model Model, llmCtx Context, opts ...SimpleStreamOptions) (*EventStream[AssistantMessageEvent, AssistantMessage], error) {
	provider, err := GetProvider(model.API)
	if err != nil {
		return nil, err
	}

	var opt SimpleStreamOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	return provider.StreamSimple(model, llmCtx, opt)
}

// CompleteSimple calls StreamSimple and waits for the final result.
func CompleteSimple(ctx context.Context, model Model, msgs []Message, opts ...SimpleStreamOptions) (AssistantMessage, error) {
	s, err := StreamSimple(ctx, model, msgs, opts...)
	if err != nil {
		return AssistantMessage{}, err
	}

	return s.Result()
}
