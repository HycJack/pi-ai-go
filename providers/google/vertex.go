package google

import (
	"fmt"
	"os"

	piai "pi-ai-go"
)

// VertexOptions holds Google Vertex AI-specific options.
type VertexOptions struct {
	ToolChoice any `json:"toolChoice,omitempty"`
	Thinking   *ThinkingConfig `json:"thinking,omitempty"`
	Project    string `json:"project,omitempty"`
	Location   string `json:"location,omitempty"`
}

// VertexProvider implements the Google Vertex AI API.
type VertexProvider struct{}

// NewVertex creates a new Google Vertex AI provider.
func NewVertex() *VertexProvider {
	return &VertexProvider{}
}

func (p *VertexProvider) Stream(model piai.Model, ctx piai.Context, opts piai.StreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	return streamVertex(model, ctx, opts, VertexOptions{})
}

func (p *VertexProvider) StreamSimple(model piai.Model, ctx piai.Context, opts piai.SimpleStreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	vertexOpts := VertexOptions{}
	if opts.Reasoning != "" {
		vertexOpts.Thinking = &ThinkingConfig{
			Enabled: true,
			Level:   mapThinkingLevel(opts.Reasoning),
		}
		if opts.ThinkingBudgets != nil {
			if budget, ok := opts.ThinkingBudgets[string(opts.Reasoning)]; ok {
				vertexOpts.Thinking.BudgetTokens = budget
			}
		}
	}
	return streamVertex(model, ctx, opts.StreamOptions, vertexOpts)
}

func streamVertex(model piai.Model, c piai.Context, opts piai.StreamOptions, vertexOpts VertexOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	apiKey := piai.ResolveAPIKey(model.Provider, opts.APIKey)

	project := vertexOpts.Project
	if project == "" {
		project = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	if project == "" {
		return nil, fmt.Errorf("google-vertex: no project specified")
	}

	location := vertexOpts.Location
	if location == "" {
		location = os.Getenv("GOOGLE_CLOUD_LOCATION")
	}
	if location == "" {
		location = "us-central1"
	}

	// Build Vertex AI-specific body
	body, err := buildGoogleBody(model, c, opts, Options{
		ToolChoice: vertexOpts.ToolChoice,
		Thinking:   vertexOpts.Thinking,
	})
	if err != nil {
		return nil, fmt.Errorf("google-vertex: failed to build request: %w", err)
	}

	if opts.OnPayload != nil {
		opts.OnPayload(body)
	}

	stream := piai.NewEventStream[piai.AssistantMessageEvent, piai.AssistantMessage]()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				stream.Error(fmt.Errorf("google-vertex: panic: %v", r))
			}
		}()

		baseURL := fmt.Sprintf("https://%s-aiplatform.googleapis.com", location)
		msg, err := doVertexStream(baseURL, apiKey, project, location, model, body, stream, opts)
		if err != nil {
			stream.Error(err)
			return
		}
		stream.End(msg)
	}()

	return stream, nil
}

func doVertexStream(baseURL, apiKey, project, location string, model piai.Model, body map[string]any, stream *piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], opts piai.StreamOptions) (piai.AssistantMessage, error) {
	// Vertex AI uses a different URL pattern
	url := fmt.Sprintf("%s/v1/projects/%s/locations/%s/publishers/google/models/%s:streamGenerateContent?alt=sse",
		baseURL, project, location, model.ID)

	// Use API key if available, otherwise use ADC
	if apiKey != "" {
		url += "&key=" + apiKey
	}

	return doGoogleStream(baseURL, apiKey, model, body, stream, opts)
}
