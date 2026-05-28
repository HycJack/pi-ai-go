package openai

import (
	"fmt"

	piai "pi-ai-go"
)

// CodexOptions holds OpenAI Codex-specific options.
type CodexOptions struct {
	ReasoningEffort  string `json:"reasoningEffort,omitempty"`
	ReasoningSummary string `json:"reasoningSummary,omitempty"`
	ServiceTier      string `json:"serviceTier,omitempty"`
	TextVerbosity    string `json:"textVerbosity,omitempty"`
}

// CodexProvider implements the OpenAI Codex/ChatGPT Responses API.
type CodexProvider struct{}

// NewCodex creates a new OpenAI Codex provider.
func NewCodex() *CodexProvider {
	return &CodexProvider{}
}

func (p *CodexProvider) Stream(model piai.Model, ctx piai.Context, opts piai.StreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	return streamCodex(model, ctx, opts, CodexOptions{})
}

func (p *CodexProvider) StreamSimple(model piai.Model, ctx piai.Context, opts piai.SimpleStreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	codexOpts := CodexOptions{}
	if opts.Reasoning != "" {
		codexOpts.ReasoningEffort = string(clampReasoning(opts.Reasoning))
	}
	return streamCodex(model, ctx, opts.StreamOptions, codexOpts)
}

func streamCodex(model piai.Model, c piai.Context, opts piai.StreamOptions, codexOpts CodexOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	apiKey := piai.ResolveAPIKey(model.Provider, opts.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("openai-codex: no API key provided")
	}

	baseURL := piai.ResolveBaseURL(model, "https://api.openai.com/v1")

	// Build body with Codex-specific options
	body, err := buildResponsesBody(model, c, opts, ResponsesOptions{
		ReasoningEffort:  codexOpts.ReasoningEffort,
		ReasoningSummary: codexOpts.ReasoningSummary,
		ServiceTier:      codexOpts.ServiceTier,
	})
	if err != nil {
		return nil, fmt.Errorf("openai-codex: failed to build request: %w", err)
	}

	if codexOpts.TextVerbosity != "" {
		body["text"] = map[string]any{
			"verbosity": codexOpts.TextVerbosity,
		}
	}

	if opts.OnPayload != nil {
		opts.OnPayload(body)
	}

	stream := piai.NewEventStream[piai.AssistantMessageEvent, piai.AssistantMessage]()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				stream.Error(fmt.Errorf("openai-codex: panic: %v", r))
			}
		}()

		msg, err := doResponsesStream(baseURL, apiKey, model, body, stream, opts)
		if err != nil {
			stream.Error(err)
			return
		}
		stream.End(msg)
	}()

	return stream, nil
}
