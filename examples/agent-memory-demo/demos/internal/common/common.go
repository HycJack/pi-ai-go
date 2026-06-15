// Package common provides shared utilities for the agent-memory-demo programs.
package common

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"pi-ai-go/agent"
	"pi-ai-go/agent/session"
	"pi-ai-go/agent/tools"
	"pi-ai-go/core"
	"pi-ai-go/llm"
	_ "pi-ai-go/providers"
)

// LoadEnv loads .env file into the process environment.
func LoadEnv() {
	file, err := os.Open(".env")
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

// GetAPIKey returns the API key from flag, env, or provider-specific env.
func GetAPIKey(provider string) string {
	if key := os.Getenv("API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv(strings.ToUpper(provider) + "_API_KEY"); key != "" {
		return key
	}
	return ""
}

// ResolveModel resolves a model by provider and model ID, with fallback.
func ResolveModel(provider, modelID string) core.Model {
	provider = strings.ToLower(provider)
	if m, err := llm.GetModel(core.KnownProvider(provider), modelID); err == nil && m.ID != "" {
		return m
	}
	return core.Model{
		ID:       modelID,
		Provider: core.KnownProvider(provider),
		API:      core.APIOpenAICompletions,
	}
}

// PrintHeader prints a formatted header.
func PrintHeader(title string) {
	fmt.Fprintln(os.Stderr, "\n" + strings.Repeat("═", 60))
	fmt.Fprintf(os.Stderr, "  %s\n", title)
	fmt.Fprintln(os.Stderr, strings.Repeat("═", 60))
}

// PrintStep prints a step marker.
func PrintStep(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n▶ "+format+"\n", args...)
}

// PrintResult prints a result.
func PrintResult(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "  ✓ "+format+"\n", args...)
}

// NewSession creates a new session with the given storage backend.
func NewSession(storage session.SessionStorage, description string) (*session.Session, error) {
	sess, err := session.NewSession(storage)
	if err != nil {
		return nil, err
	}
	id := fmt.Sprintf("demo-%d", time.Now().Unix())
	_ = sess.Append(session.SessionTreeEntry{
		ID:          session.GenerateID(),
		Type:        session.EntrySessionInfo,
		Timestamp:   time.Now(),
		SessionID:   id,
		Description: description,
	})
	return sess, nil
}

// BuildAgentConfig builds a standard agent config.
func BuildAgentConfig(model core.Model, apiKey string, policy *agent.ContextPolicy, onCompact func(agent.CompactionEvent)) agent.AgentLoopConfig {
	cfg := agent.AgentLoopConfig{
		Model:        model,
		SystemPrompt: "你是一个有帮助的助手。请用中文回答。",
		Tools:        tools.All(),
		ContextPolicy: policy,
	}
	if onCompact != nil {
		cfg.OnCompaction = onCompact
	}
	cfg.SimpleStreamOptions.APIKey = apiKey
	return cfg
}

// AppendMessage appends a message to the session.
func AppendMessage(sess *session.Session, msg core.Message) {
	_ = sess.Append(session.SessionTreeEntry{
		ID:        session.GenerateID(),
		Type:      session.EntryMessage,
		Timestamp: time.Now(),
		Message:   msg,
	})
}

// PrintSessionStats prints session statistics.
func PrintSessionStats(sess *session.Session) {
	entries := sess.Entries()
	var user, assistant, tool, compactions, branches int
	for _, e := range entries {
		switch e.Type {
		case session.EntryMessage:
			switch e.Message.(type) {
			case core.UserMessage:
				user++
			case core.AssistantMessage:
				assistant++
			case core.ToolResultMessage:
				tool++
			}
		case session.EntryCompaction:
			compactions++
		case session.EntryBranchSummary:
			branches++
		}
	}
	fmt.Fprintf(os.Stderr, "  📊 会话统计: 用户=%d 助手=%d 工具=%d 压缩=%d 分支=%d 总计=%d\n",
		user, assistant, tool, compactions, branches, len(entries))
}

// RunSingleQuery runs a single query against the agent.
func RunSingleQuery(ctx context.Context, config agent.AgentLoopConfig, prompt string) (core.AssistantMessage, error) {
	messages := []core.Message{
		core.UserMessage{Role: "user", Content: prompt, Timestamp: time.Now()},
	}
	stream, _ := agent.AgentLoopDetailed(ctx, messages, config)
	var lastAssistant core.AssistantMessage
	_, _ = stream.ForEach(ctx, func(evt agent.AgentEvent) error {
		switch e := evt.(type) {
		case agent.EventMessageUpdate:
			switch ae := e.AssistantEvent.(type) {
			case core.EventTextDelta:
				fmt.Print(ae.Delta)
			case core.EventToolCallStart:
				fmt.Fprintf(os.Stderr, "\n  🔧 %s", ae.Name)
			}
		case agent.EventMessageEnd:
			lastAssistant = e.Message
		case agent.EventToolExecEnd:
			status := "✅"
			if e.IsError {
				status = "❌"
			}
			fmt.Fprintf(os.Stderr, "  %s %s\n", status, e.ToolName)
		}
		return nil
	})
	fmt.Println()
	return lastAssistant, nil
}
