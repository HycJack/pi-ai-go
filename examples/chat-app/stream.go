package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"pi-ai-go/core"
	"pi-ai-go/llm"

	"chat-app/contextmgr"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// StreamMessage handles a non-agent streaming chat request. It builds the
// message history, calls the LLM, and emits stream events to the frontend.
func (a *App) StreamMessage(params map[string]interface{}) error {
	message, _ := params["message"].(string)
	providerStr, _ := params["provider"].(string)
	apiKey, _ := params["apiKey"].(string)
	baseURL, _ := params["baseUrl"].(string)
	modelID, _ := params["model"].(string)

	LogInfo("[stream] request: provider=%s model=%s baseURL=%s msgLen=%d", providerStr, modelID, baseURL, len(message))

	cp := a.settings.Current()
	if providerStr == "" && cp != nil {
		providerStr = cp.Type
	}
	if apiKey == "" && cp != nil {
		apiKey = cp.APIKey
	}
	if baseURL == "" && cp != nil {
		baseURL = cp.BaseURL
	}
	if modelID == "" {
		modelID = a.settings.Model
	}

	model := a.resolveModel(providerStr, modelID, baseURL)
	if apiKey == "" {
		apiKey = a.selectAPIKey()
	}
	if apiKey == "" {
		apiKey = core.ResolveAPIKey(model.Provider, "")
	}

	messages := buildMessages(params, message)
	streamCtx, cancelFn := context.WithCancel(a.ctx)
	a.cancelFn = cancelFn

	// Use per-conversation token stats.
	tokenStats := a.getOrCreateTokenStats(a.currentConvID, modelID)
	tokenStats.Recompute(messages)
	totalTokens := tokenStats.Tokens()
	hardLimit := a.ctxSettings.HardLimit()
	if totalTokens > hardLimit && len(messages) > 5 {
		messages = contextmgr.Truncate(messages, int(math.Max(float64(len(messages))*0.5, 5)))
		tokenStats.Recompute(messages)
	}

	maxTokens, _ := params["maxTokens"].(float64)
	temperature, _ := params["temperature"].(float64)
	reasoning, _ := params["reasoning"].(string)

	opts := core.SimpleStreamOptions{
		StreamOptions: core.StreamOptions{
			APIKey: apiKey,
		},
	}
	if maxTokens > 0 {
		t := int(maxTokens)
		opts.MaxTokens = &t
	}
	if temperature > 0 {
		t := temperature
		opts.Temperature = &t
	}
	if reasoning != "" {
		opts.Reasoning = core.ThinkingLevel(reasoning)
	}

	stream, err := llm.StreamSimple(streamCtx, model, messages, opts)
	if err != nil {
		LogError("[stream] StreamSimple error: %v", err)
		runtime.EventsEmit(a.ctx, "stream-error", fmt.Sprintf("Error: %v", err))
		return err
	}

	LogInfo("[stream] stream started, messages=%d", len(messages))

	go func() {
		defer func() {
			a.cancelFn = nil
		}()
		textLen := 0
		_, forEachErr := stream.ForEach(streamCtx, func(event core.AssistantMessageEvent) error {
			switch e := event.(type) {
			case core.EventThinkingDelta:
				runtime.EventsEmit(a.ctx, "stream-thinking-delta", e.Delta)
			case core.EventToolCallStart:
				data, _ := json.Marshal(map[string]interface{}{"id": e.ID, "name": e.Name})
				runtime.EventsEmit(a.ctx, "stream-tool-call-start", string(data))
			case core.EventToolCallDelta:
				runtime.EventsEmit(a.ctx, "stream-tool-call-delta", e.ArgumentsDelta)
			case core.EventToolCallEnd:
				argsStr := string(e.Arguments)
				safeArgs, _ := json.Marshal(argsStr)
				runtime.EventsEmit(a.ctx, "stream-tool-call-end", string(safeArgs))
			case core.EventTextDelta:
				textLen += len(e.Delta)
				runtime.EventsEmit(a.ctx, "stream-text-delta", e.Delta)
			case core.EventDone:
				a.markKeySuccess()
				LogInfo("[stream] done, total text length=%d", textLen)
				if textLen == 0 {
					LogWarn("[stream] done but no text was received — model may have returned empty content")
				}
				runtime.EventsEmit(a.ctx, "stream-done", "")
				return nil
			}
			return nil
		})
		if forEachErr != nil {
			LogError("[stream] ForEach error: %v (textLen=%d)", forEachErr, textLen)
			a.markKeyFailed(forEachErr)
			runtime.EventsEmit(a.ctx, "stream-error", fmt.Sprintf("Error: %v", forEachErr))
			runtime.EventsEmit(a.ctx, "stream-done", "")
		}
	}()
	return nil
}
