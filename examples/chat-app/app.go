package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	piai "pi-ai-go"
	"pi-ai-go/core"
	"pi-ai-go/providers"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx       context.Context
	cancelFn  context.CancelFunc
	streamCtx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	providers.RegisterBuiltInProviders()
}

// CancelStream cancels the current streaming request
func (a *App) CancelStream() {
	if a.cancelFn != nil {
		a.cancelFn()
		a.cancelFn = nil
	}
}

func (a *App) Greet(name string) string {
	return "Hello " + name + ", Welcome to PI AI!"
}

type ModelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (a *App) GetModels(params map[string]interface{}) ([]ModelInfo, error) {
	providerStr, ok := params["provider"].(string)
	if !ok {
		return nil, nil
	}

	baseURL, _ := params["baseUrl"].(string)
	apiKey, _ := params["apiKey"].(string)

	if providerStr == "anthropic" {
		return a.getAnthropicModels(baseURL, apiKey)
	}

	return a.getOpenAIModels(baseURL, apiKey)
}

func (a *App) getOpenAIModels(baseURL, apiKey string) ([]ModelInfo, error) {
	url := baseURL
	if url == "" {
		url = "https://api.openai.com/v1"
	}
	url += "/models"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return a.getCachedModels(piai.ProviderOpenAI), nil
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	} else {
		envKey := core.ResolveAPIKey(piai.ProviderOpenAI, "")
		if envKey != "" {
			req.Header.Set("Authorization", "Bearer "+envKey)
		}
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error fetching models from API: %v\n", err)
		return a.getCachedModels(piai.ProviderOpenAI), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("API returned status: %d\n", resp.StatusCode)
		return a.getCachedModels(piai.ProviderOpenAI), nil
	}

	var result struct {
		Data []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Object     string `json:"object"`
			OwnedBy    string `json:"owned_by"`
			Permission []struct {
				ID                 string `json:"id"`
				Object             string `json:"object"`
				Created            int64  `json:"created"`
				AllowCreateEngine  bool   `json:"allow_create_engine"`
				AllowSampling      bool   `json:"allow_sampling"`
				AllowLogprobs      bool   `json:"allow_logprobs"`
				AllowSearchIndices bool   `json:"allow_search_indices"`
				AllowView          bool   `json:"allow_view"`
				AllowFineTuning    bool   `json:"allow_fine_tuning"`
				Organization       string `json:"organization"`
				Group              string `json:"group"`
				IsBlocking         bool   `json:"is_blocking"`
			} `json:"permission"`
			Root   string `json:"root"`
			Parent string `json:"parent"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Printf("Error parsing models response: %v\n", err)
		return a.getCachedModels(piai.ProviderOpenAI), nil
	}

	var modelInfos []ModelInfo
	for _, m := range result.Data {
		modelInfos = append(modelInfos, ModelInfo{
			ID:   m.ID,
			Name: m.Name,
		})
	}

	fmt.Printf("Fetched %d models from API\n", len(modelInfos))
	return modelInfos, nil
}

func (a *App) getAnthropicModels(baseURL, apiKey string) ([]ModelInfo, error) {
	url := baseURL
	if url == "" {
		url = "https://api.anthropic.com/v1"
	}
	url += "/models"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return a.getCachedModels(piai.ProviderAnthropic), nil
	}

	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	} else {
		envKey := core.ResolveAPIKey(piai.ProviderAnthropic, "")
		if envKey != "" {
			req.Header.Set("x-api-key", envKey)
		}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error fetching models from API: %v\n", err)
		return a.getCachedModels(piai.ProviderAnthropic), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("API returned status: %d\n", resp.StatusCode)
		return a.getCachedModels(piai.ProviderAnthropic), nil
	}

	var result struct {
		Data []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			ContextWindow int    `json:"context_window"`
			MaxTokens     int    `json:"max_tokens"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Printf("Error parsing models response: %v\n", err)
		return a.getCachedModels(piai.ProviderAnthropic), nil
	}

	var modelInfos []ModelInfo
	for _, m := range result.Data {
		modelInfos = append(modelInfos, ModelInfo{
			ID:   m.ID,
			Name: m.Name,
		})
	}

	fmt.Printf("Fetched %d models from API\n", len(modelInfos))
	return modelInfos, nil
}

func (a *App) getCachedModels(provider core.KnownProvider) []ModelInfo {
	models := piai.GetModels(provider)
	var result []ModelInfo
	for _, m := range models {
		result = append(result, ModelInfo{
			ID:   m.ID,
			Name: m.Name,
		})
	}
	return result
}

func (a *App) SendMessage(params map[string]interface{}) (string, error) {
	message, _ := params["message"].(string)
	providerStr, _ := params["provider"].(string)
	apiKey, _ := params["apiKey"].(string)
	baseURL, _ := params["baseUrl"].(string)
	modelID, _ := params["model"].(string)

	return a.callAIStream(message, providerStr, apiKey, baseURL, modelID)
}

func (a *App) StreamMessage(params map[string]interface{}) error {
	message, _ := params["message"].(string)
	providerStr, _ := params["provider"].(string)
	apiKey, _ := params["apiKey"].(string)
	baseURL, _ := params["baseUrl"].(string)
	modelID, _ := params["model"].(string)

	return a.callAIStreamWithEvents(message, providerStr, apiKey, baseURL, modelID)
}

// TestEvents sends test events to frontend for debugging
func (a *App) TestEvents() {
	runtime.EventsEmit(a.ctx, "stream-text-delta", "Test message 1")
	time.Sleep(500 * time.Millisecond)
	runtime.EventsEmit(a.ctx, "stream-text-delta", " Test message 2")
	time.Sleep(500 * time.Millisecond)
	runtime.EventsEmit(a.ctx, "stream-done", "")
}

func (a *App) callAIStreamWithEvents(message, providerStr, apiKey, baseURL, modelID string) error {
	var provider core.KnownProvider
	var api core.KnownAPI

	if providerStr == "anthropic" {
		provider = piai.ProviderAnthropic
		api = piai.APIAnthropicMessages
	} else {
		provider = piai.ProviderOpenAI
		api = piai.APIOpenAICompletions
	}

	var model core.Model
	var err error

	if modelID != "" {
		model, err = piai.GetModel(provider, modelID)
		if err != nil {
			model = core.Model{
				ID:            modelID,
				Provider:      provider,
				API:           api,
				ContextWindow: 8192,
			}
		}
	} else {
		models := piai.GetModels(provider)
		if len(models) > 0 {
			model = models[0]
		} else {
			defaultModel := "gpt-4o-mini"
			if provider == piai.ProviderAnthropic {
				defaultModel = "claude-3-5-haiku-20241022"
			}
			model = core.Model{
				ID:            defaultModel,
				Provider:      provider,
				API:           api,
				ContextWindow: 8192,
			}
		}
	}

	if baseURL != "" {
		model.BaseURL = baseURL
	}

	messages := []core.Message{
		core.UserMessage{Content: message},
	}

	// 创建一个可取消的 context 用于流式请求
	streamCtx, cancelFn := context.WithCancel(a.ctx)
	a.cancelFn = cancelFn

	opts := core.SimpleStreamOptions{
		StreamOptions: core.StreamOptions{
			APIKey: apiKey,
		},
	}

	stream, err := piai.StreamSimple(streamCtx, model, messages, opts)
	if err != nil {
		runtime.EventsEmit(a.ctx, "stream-error", fmt.Sprintf("Error: %v", err))
		a.cancelFn = nil
		return err
	}

	go func() {
		_, err = stream.ForEach(streamCtx, func(event core.AssistantMessageEvent) error {
			switch e := event.(type) {
			case core.EventThinkingStart:
				runtime.EventsEmit(a.ctx, "stream-thinking-start", "")
			case core.EventThinkingDelta:
				runtime.EventsEmit(a.ctx, "stream-thinking-delta", e.Delta)
			case core.EventThinkingEnd:
				runtime.EventsEmit(a.ctx, "stream-thinking-end", "")
			case core.EventToolCallStart:
				data := map[string]interface{}{"id": e.ID, "name": e.Name}
				jsonData, _ := json.Marshal(data)
				runtime.EventsEmit(a.ctx, "stream-tool-call-start", string(jsonData))
			case core.EventToolCallDelta:
				runtime.EventsEmit(a.ctx, "stream-tool-call-delta", e.ArgumentsDelta)
			case core.EventToolCallEnd:
				runtime.EventsEmit(a.ctx, "stream-tool-call-end", string(e.Arguments))
			case core.EventTextStart:
				runtime.EventsEmit(a.ctx, "stream-text-start", "")
			case core.EventTextDelta:
				runtime.EventsEmit(a.ctx, "stream-text-delta", e.Delta)
			case core.EventTextEnd:
				runtime.EventsEmit(a.ctx, "stream-text-end", "")
			case core.EventDone:
				runtime.EventsEmit(a.ctx, "stream-done", "")
				return nil
			}
			return nil
		})

		if err != nil {
			runtime.EventsEmit(a.ctx, "stream-error", fmt.Sprintf("Error: %v", err))
		}

		a.cancelFn = nil
	}()

	return nil
}

type ToolCallInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type AIResponse struct {
	Content   string         `json:"content"`
	Thinking  string         `json:"thinking"`
	ToolCalls []ToolCallInfo `json:"toolCalls"`
}

func (a *App) callAIStream(message, providerStr, apiKey, baseURL, modelID string) (string, error) {
	var provider core.KnownProvider
	var api core.KnownAPI

	if providerStr == "anthropic" {
		provider = piai.ProviderAnthropic
		api = piai.APIAnthropicMessages
	} else {
		provider = piai.ProviderOpenAI
		api = piai.APIOpenAICompletions
	}

	var model core.Model
	var err error

	if modelID != "" {
		model, err = piai.GetModel(provider, modelID)
		if err != nil {
			fmt.Printf("Model %s not found in registry, creating custom model\n", modelID)
			model = core.Model{
				ID:            modelID,
				Provider:      provider,
				API:           api,
				ContextWindow: 8192,
			}
		} else {
			fmt.Printf("Found model in registry: %s with API: %s\n", model.ID, model.API)
		}
	} else {
		models := piai.GetModels(provider)
		if len(models) > 0 {
			model = models[0]
			fmt.Printf("Using default model: %s with API: %s\n", model.ID, model.API)
		} else {
			defaultModel := "gpt-4o-mini"
			if provider == piai.ProviderAnthropic {
				defaultModel = "claude-3-5-haiku-20241022"
			}
			model = core.Model{
				ID:            defaultModel,
				Provider:      provider,
				API:           api,
				ContextWindow: 8192,
			}
			fmt.Printf("Using fallback model: %s with API: %s\n", model.ID, model.API)
		}
	}

	if baseURL != "" {
		model.BaseURL = baseURL
		fmt.Printf("Using custom base URL: %s\n", baseURL)
	}

	if apiKey != "" {
		fmt.Println("API Key provided")
	} else {
		fmt.Println("No API Key provided, will use environment variable")
	}

	messages := []core.Message{
		core.UserMessage{Content: message},
	}

	opts := core.SimpleStreamOptions{
		StreamOptions: core.StreamOptions{
			APIKey: apiKey,
		},
	}

	fmt.Printf("Calling StreamSimple with model: %s, API: %s, Provider: %s\n", model.ID, model.API, model.Provider)
	stream, err := piai.StreamSimple(a.ctx, model, messages, opts)
	if err != nil {
		fmt.Printf("Error calling AI: %v\n", err)
		return fmt.Sprintf(`{"content":"Error: %v","thinking":"","toolCalls":[]}`, err), nil
	}

	response := AIResponse{
		Content:   "",
		Thinking:  "",
		ToolCalls: []ToolCallInfo{},
	}

	var thinkingBuilder strings.Builder
	var currentToolCall *ToolCallInfo

	_, err = stream.ForEach(a.ctx, func(event core.AssistantMessageEvent) error {
		switch e := event.(type) {
		case core.EventThinkingStart:
			fmt.Println("Thinking started")
			thinkingBuilder.Reset()
		case core.EventThinkingDelta:
			thinkingBuilder.WriteString(e.Delta)
			fmt.Printf("Thinking: %s\n", e.Delta)
		case core.EventThinkingEnd:
			response.Thinking = thinkingBuilder.String()
			fmt.Printf("Thinking completed: %s\n", response.Thinking)
		case core.EventToolCallStart:
			fmt.Printf("Tool call started: %s (%s)\n", e.ID, e.Name)
			currentToolCall = &ToolCallInfo{
				ID:        e.ID,
				Name:      e.Name,
				Arguments: "",
			}
		case core.EventToolCallDelta:
			if currentToolCall != nil {
				currentToolCall.Arguments += e.ArgumentsDelta
				fmt.Printf("Tool call delta: %s\n", e.ArgumentsDelta)
			}
		case core.EventToolCallEnd:
			if currentToolCall != nil {
				currentToolCall.Arguments = string(e.Arguments)
				response.ToolCalls = append(response.ToolCalls, *currentToolCall)
				fmt.Printf("Tool call completed: %+v\n", currentToolCall)
				currentToolCall = nil
			}
		case core.EventTextStart:
			fmt.Println("Text started")
		case core.EventTextDelta:
			response.Content += e.Delta
			fmt.Printf("Text delta: %s\n", e.Delta)
		case core.EventTextEnd:
			fmt.Println("Text completed")
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Stream error: %v\n", err)
		return fmt.Sprintf(`{"content":"Error: %v","thinking":"","toolCalls":[]}`, err), nil
	}

	resultJSON, err := json.Marshal(response)
	if err != nil {
		fmt.Printf("Error marshaling response: %v\n", err)
		return fmt.Sprintf(`{"content":"Error marshaling response","thinking":"","toolCalls":[]}`), nil
	}

	return string(resultJSON), nil
}

func simulateAIResponse(message string) string {
	responses := []string{
		"That's a great question! Let me think about it...\n\nBased on my knowledge, here is what I can tell you about this topic. The subject is quite fascinating and has many interesting aspects worth exploring.",
		"I understand your request. Here is a detailed response:\n\n**Key Points:**\n1. First important aspect to consider\n2. Second important aspect related to your question\n3. Third important aspect that provides additional context\n\nI hope this helps clarify the topic for you!",
		"Great question! Here is my response:\n\nIn summary, the answer depends on various factors including context, available resources, and specific requirements. Each situation is unique, so it's important to consider all variables before making a decision.",
		"Thank you for asking! Here is what I know about this topic:\n\nThe subject is quite broad, but I can provide some general insights that might be helpful. Feel free to ask follow-up questions if you need more specific information.",
		"Interesting question! Let me break this down for you:\n\n- **Key concept 1:** Understanding the fundamentals\n- **Key concept 2:** Exploring practical applications\n- **Key concept 3:** Considering potential implications\n\nThis framework can help approach the topic systematically.",
		"Here's a mathematical example: The quadratic formula is $$x = \\frac{-b \\pm \\sqrt{b^2-4ac}}{2a}$$",
		"Here's a table example:\n\n| Feature | Description | Status |\n|---------|-------------|--------|\n| Markdown | Supports rich text | ✅ |\n| LaTeX | Math formula rendering | ✅ |\n| Tables | Tabular data display | ✅ |\n\nAll features are working correctly!",
	}
	return responses[len(message)%len(responses)]
}
