# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
# Build all packages
go build ./...

# Run all tests
go test ./...

# Run tests for a specific package
go test ./providers/anthropic/...
go test -v -run TestEventStream .

# Verbose with coverage
go test -v -cover ./...
```

No Makefile, linter, or CI is configured. Go 1.23+ required. Only external dependency is `github.com/santhosh-tekuri/jsonschema/v6`.

## Architecture

### Provider Registry Pattern

Two-layer dispatch:

1. **API Registry** (`api-registry.go`): Maps `KnownAPI` (9 values) → `APIProvider` implementations. `Stream()` calls `GetProvider(model.API)` to dispatch.
2. **Provider packages** (`providers/*/`): Each implements `APIProvider` interface with `Stream()` and `StreamSimple()`.

Registration happens in `providers/register.go` via `init()`. Importing `_ "pi-ai-go/providers"` registers all providers. Providers register with `sourceID = "builtin"` for bulk cleanup via `UnregisterProviders()`.

**Key distinction**: `KnownProvider` (22 values) = logical provider (DeepSeek, Groq, etc.). `KnownAPI` (9 values) = API protocol. Multiple providers share one API type. The `Compat` struct on `Model` handles per-provider variations.

### Streaming Flow

```
Stream() → GetProvider(model.API) → provider.Stream() → goroutine:
  → HTTP POST (stream: true)
  → SSE parse (bufio.Scanner)
  → Push typed events into EventStream[AssistantMessageEvent, AssistantMessage]
  → stream.End(finalMsg) or stream.Error(err)
```

`Complete()` = `Stream()` + `stream.Result()` (blocks until done).

### EventStream[T, R]

Core generic streaming primitive in `eventstream.go`:
- 64-element buffered channel
- **Non-blocking Push** with `select` + `stop` channel — returns `bool` (false = consumer stopped)
- `Stop()` signals producer to stop (called by `ForEach` on context cancel or callback error)
- `End()`/`Error()` release mutex before channel write to avoid deadlock
- `ForEach()` iterates events with context cancellation support

### Dual Option Levels

- `StreamOptions`: Low-level (temperature, max tokens, API key, headers, callbacks)
- `SimpleStreamOptions`: Embeds `StreamOptions` + `Reasoning ThinkingLevel` + `ThinkingBudgets`

Each provider maps reasoning levels to its native format in `StreamSimple()`.

### Message Transformation

`providers/transform.go` handles cross-provider compatibility:
- Image content removal for non-vision models
- Thinking block preservation (same-model) or conversion to text (cross-model)
- Tool call ID normalization via deterministic short IDs

### Content Types Use Marker Interfaces

`ContentBlock`, `Message`, and `AssistantMessageEvent` use empty methods (`contentTag()`, `messageTag()`, `eventTag()`) as sealed union types. Use type switches to handle variants.

## Key Files

- `types.go` — All type definitions (Model, Message variants, ContentBlock variants, 12 event types, enums)
- `pi.go` — Public API entry points (Stream, Complete, StreamSimple, CompleteSimple)
- `eventstream.go` — Generic EventStream implementation
- `api-registry.go` — Provider dispatch registry
- `env-api-keys.go` — API key resolution from environment variables
- `providers/register.go` — Built-in provider registration
- `providers/transform.go` — Cross-provider message transformation

## Provider Implementation Pattern

Each provider follows the same structure:
1. Implement `APIProvider` interface (`Stream`, `StreamSimple`)
2. Build provider-specific JSON body from `Context` + `StreamOptions`
3. Spawn goroutine for HTTP POST + SSE parsing
4. Push typed events (`EventStart`, `EventTextDelta`, `EventToolCallStart/Delta/End`, `EventDone`, `EventError`)
5. Call `stream.End(msg)` or `stream.Error(err)`

All providers use `net/http` directly — no SDK dependencies.

## Environment Variables

API keys are resolved via `ResolveAPIKey()` in `env-api-keys.go`. Each provider has a convention like `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, etc. See `.env.example` for the full list. Integration tests in `test/` use a `.env` file.
