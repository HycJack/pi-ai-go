# AGENTS.md

This file provides guidance for AI coding agents working in this repository.

## Quick Reference

```bash
go build ./...                    # Build all
go test ./...                     # Test all
go test -v -run TestName .        # Single test
go test ./providers/openai/...    # Package tests
```

Go 1.23+. Single external dep (`jsonschema/v6`). No Makefile or linter.

## How to Add a New Provider

1. Create `providers/<name>/<name>.go`
2. Implement `APIProvider` interface (two methods: `Stream`, `StreamSimple`)
3. Register in `providers/register.go` `RegisterBuiltInProviders()` with `piai.RegisterProvider()`
4. Add provider constant to `KnownProvider` in `types.go` if it's a new logical provider
5. Add API key env var mapping in `env-api-keys.go` `providerEnvVars`
6. Add `Model` entries to the model registry (generated data or manual)

### Provider Implementation Checklist

Each provider must:
- Build JSON body from `piai.Context` + `piai.StreamOptions`
- Spawn a goroutine for HTTP POST + SSE parsing
- Push events in order: `EventStart` → `EventTextDelta`/`EventThinkingDelta`/`EventToolCallStart`+`Delta`+`End` → `EventDone`
- Call `stream.End(msg)` on success, `stream.Error(err)` on failure
- Handle `opts.OnPayload` (request body callback) and `opts.OnResponse` (raw response callback)
- Set `msg.Usage` (input/output/cache tokens) and `msg.StopReason`
- Call `piai.CalculateCost(model, msg.Usage)` for cost tracking

### Provider File Structure

```
providers/<name>/
  <name>.go       # Main implementation (Stream, StreamSimple, SSE processing)
  <name>_test.go  # Unit tests for message conversion and request building
```

Shared OpenAI-family code lives in `providers/openai/shared.go`. Shared Google-family code in `providers/google/shared.go`.

## How to Add a New Event Type

1. Define struct in `types.go` with `eventTag()` method
2. Add to `AssistantMessageEvent` interface (already satisfied by the method)
3. Push from relevant providers in their SSE processing loops
4. Handle in consumers via type switch on `AssistantMessageEvent`

## How to Add a New ContentBlock Type

1. Define struct in `types.go` with `contentTag()` method
2. Handle in each provider's `convertAssistantContent` / message conversion functions
3. Handle in each provider's SSE parsing (if the API returns this type)

## Common Patterns

### Message Conversion

Every provider has a `convertMessages()` function that transforms `[]piai.Message` into provider-specific `[]map[string]any`. Pattern:

```go
case piai.UserMessage:
    // Convert Content (string or []ContentBlock) to provider format
case piai.AssistantMessage:
    // Convert Content blocks (TextContent, ThinkingContent, ToolCall) to provider format
case piai.ToolResultMessage:
    // Convert tool results to provider format
```

### SSE Processing

All providers use the same pattern:

```go
scanner := bufio.NewScanner(body)
scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

for scanner.Scan() {
    line := scanner.Text()
    if !strings.HasPrefix(line, "data: ") {
        continue
    }
    data := strings.TrimPrefix(line, "data: ")
    // Parse JSON, dispatch on event type
}
```

### Stop Reason Mapping

Each provider has a `mapStopReason()` function mapping provider-specific strings to `piai.StopReason` constants. Add new mappings as needed.

### Tool Call ID Normalization

Some providers have strict ID requirements (e.g., Mistral requires 9-char alphanumeric). Use `normalizeToolCallID()` or `generateShortID()` from `providers/transform.go`.

## EventStream Contract

- `Push()` returns `bool` — producer should stop if false
- `End()`/`Error()` must be called exactly once to close the stream
- `Stop()` is called by consumer on early exit (context cancel, callback error)
- Channel buffer is 64 — producers should handle backpressure via `Push()` return value
- `ForEach()` calls `Stop()` automatically on context cancel or callback error

## Testing

- Unit tests for message conversion and request body building (most common)
- Table-driven tests for mapping functions
- Registry tests use `ClearProviders()` + `defer ClearProviders()`
- EventStream tests use goroutines to simulate concurrent producer/consumer
- No mocking framework — tests exercise internal functions directly
- Integration tests are standalone programs in `test/` (not `_test.go` files)

## Things to Watch For

- `json.Unmarshal` for tool parameters must check errors (silent failure = nil params)
- Anthropic requires `signature` fields on thinking/text blocks in multi-turn conversations
- Mistral assistant messages with tool calls must have `tool_calls` at top level, not nested in `content`
- Vertex AI uses a different URL pattern (`/v1/projects/{project}/locations/{location}/publishers/google/models/...`) than Google AI (`/v1beta/models/...`)
- `http.NewRequest` should use `context.Context` for cancellation (currently not propagated — known issue)
- Each provider creates a new `http.Client` per request (no connection reuse)
- `opts.Signal` channel is defined but never checked by providers
