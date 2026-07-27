package main

// This file is intentionally left minimal. All functionality has been
// split into focused modules:
//
//   types.go         — struct definitions
//   settings.go      — App lifecycle, settings load/save, context stats
//   conversations.go — conversation file storage
//   memory_api.go    — memory CRUD API
//   capture.go       — screen capture
//   models.go        — model list fetching
//   stream.go        — StreamMessage (non-agent chat)
//   agent.go         — AgentMessage, resolveModel, buildSystemPrompt, autolearn
//   messages.go      — message history construction & parsing
//   logger.go        — file logger
//
// The main.go file contains the Wails entry point (main function).
