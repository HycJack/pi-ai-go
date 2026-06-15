// Example: Agent with skills support
//
// Usage:
//
//	go run . -query "your question"
//	go run . -skills ./skills -query "list files"
//
// Environment:
//
//	Set PROVIDER, MODEL, and API_KEY in .env or via flags
package main

import (
	"bufio"
	"context"
	"flag"
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

func main() {
	// Load .env file
	loadEnv()

	// Parse flags
	skillsDir := flag.String("skills", os.Getenv("SKILLS"), "Directory containing SKILL.md files")
	modelID := flag.String("model", os.Getenv("MODEL"), "Model ID")
	provider := flag.String("provider", os.Getenv("PROVIDER"), "Provider name")
	baseURL := flag.String("base-url", os.Getenv("BASE_URL"), "Base URL")
	apiKeyFlag := flag.String("api-key", "", "API key")
	query := flag.String("query", "", "Query to run (interactive if empty)")
	verbose := flag.Bool("v", false, "Verbose mode")
	flag.Parse()

	// Log configuration
	fmt.Fprintf(os.Stderr, "[config] Provider: %s\n", *provider)
	fmt.Fprintf(os.Stderr, "[config] Model: %s\n", *modelID)
	fmt.Fprintf(os.Stderr, "[config] Base URL: %s\n", *baseURL)
	fmt.Fprintf(os.Stderr, "[config] Skills dir: %s\n", *skillsDir)
	fmt.Fprintf(os.Stderr, "[config] Query: %s\n", *query)
	fmt.Fprintf(os.Stderr, "[config] Verbose: %v\n", *verbose)

	// Validate configuration
	if *modelID == "" {
		fmt.Fprintln(os.Stderr, "Error: MODEL not configured")
		os.Exit(1)
	}

	apiKey := *apiKeyFlag
	if apiKey == "" {
		apiKey = os.Getenv("API_KEY")
		if apiKey == "" && *provider != "" {
			apiKey = os.Getenv(strings.ToUpper(*provider) + "_API_KEY")
		}
	}
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: API key not configured")
		os.Exit(1)
	}

	// Resolve model
	model := resolveModel(*provider, *modelID, *baseURL)
	if model.ID == "" {
		fmt.Fprintln(os.Stderr, "Error: Failed to resolve model")
		os.Exit(1)
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Using model: %s (%s)\n", model.ID, model.Provider)
	}

	// Load skills if directory specified
	var skillsText string
	if *skillsDir != "" {
		skills, diags := session.LoadSkills(*skillsDir)
		for _, d := range diags {
			fmt.Fprintf(os.Stderr, "[skill] %s: %s\n", d.Path, d.Message)
		}
		if len(skills) > 0 {
			skillsText = session.FormatSkillsForSystemPrompt(skills)
			if *verbose {
				fmt.Fprintf(os.Stderr, "Loaded %d skill(s)\n", len(skills))
			}
		}
	}

	// Build system prompt
	systemPrompt := buildSystemPrompt(skillsText)

	// Configure agent
	config := agent.AgentLoopConfig{
		Model:        model,
		SystemPrompt: systemPrompt,
		Tools:        tools.All(),
	}
	config.SimpleStreamOptions.APIKey = apiKey

	// Run agent
	if *query != "" {
		runSingleQuery(config, *query, *verbose)
	} else {
		runInteractive(config, *verbose)
	}
}

func resolveModel(provider, modelID, baseURL string) core.Model {
	// Normalize provider to lowercase to ensure consistent matching
	provider = strings.ToLower(provider)

	// Try to get model from registry
	if provider != "" && modelID != "" {
		if model, err := llm.GetModel(core.KnownProvider(provider), modelID); err == nil && model.ID != "" {
			if baseURL != "" {
				model.BaseURL = baseURL
			}
			return model
		}
	}

	// Fallback to manual config
	// Determine API type based on provider
	api := core.KnownAPI("")
	switch core.KnownProvider(provider) {
	case core.ProviderOpenAI, core.ProviderDeepSeek, core.ProviderGroq, core.ProviderFireworks, core.ProviderTogether, core.ProviderCerebras:
		api = core.APIOpenAICompletions
	case core.ProviderAnthropic:
		api = core.APIAnthropicMessages
	case core.ProviderGoogle:
		api = core.APIGoogleGenerative
	case core.ProviderGoogleVertex:
		api = core.APIGoogleVertex
	case core.ProviderMistral:
		api = core.APIMistralConversations
	case core.ProviderAzureOpenAI:
		api = core.APIAzureOpenAIResponses
	case core.ProviderOpenRouter:
		api = core.OpenRouter
	case core.ProviderAmazonBedrock:
		api = core.APIBedrockConverse
	default:
		// Default to OpenAI compatible for unknown providers
		api = core.APIOpenAICompletions
	}

	return core.Model{
		ID:       modelID,
		Provider: core.KnownProvider(provider),
		API:      api,
		BaseURL:  baseURL,
	}
}

func buildSystemPrompt(skillsText string) string {
	base := "You are a helpful coding assistant. You have access to file system tools."
	if skillsText != "" {
		return base + "\n\n" + skillsText
	}
	return base
}

func runSingleQuery(config agent.AgentLoopConfig, query string, verbose bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	messages := []core.Message{
		core.UserMessage{
			Role:      "user",
			Content:   query,
			Timestamp: time.Now(),
		},
	}

	stream, detailed := agent.AgentLoopDetailed(ctx, messages, config)

	_, err := stream.ForEach(ctx, func(evt agent.AgentEvent) error {
		switch e := evt.(type) {
		case agent.EventMessageUpdate:
			switch ae := e.AssistantEvent.(type) {
			case core.EventTextDelta:
				fmt.Print(ae.Delta)
			case core.EventThinkingDelta:
				if verbose {
					fmt.Fprintf(os.Stderr, "[thinking] %s", ae.Delta)
				}
			case core.EventToolCallStart:
				fmt.Fprintf(os.Stderr, "\n[tool] %s", ae.Name)
			case core.EventToolCallDelta:
				if len(ae.ArgumentsDelta) > 0 {
					fmt.Fprintf(os.Stderr, "%s", ae.ArgumentsDelta)
				}
			case core.EventToolCallEnd:
				fmt.Fprintf(os.Stderr, "\n")
			case core.EventError:
				fmt.Fprintf(os.Stderr, "\nError: %v\n", ae.Error)
			}
		case agent.EventMessageEnd:
			printAssistantMessage(e.Message)
			if e.Message.ErrorMessage != "" {
				fmt.Fprintf(os.Stderr, "\nError: %s\n", e.Message.ErrorMessage)
			}
			if len(e.Message.Content) == 0 {
				fmt.Fprintf(os.Stderr, "\n[Warning] Assistant message has no content\n")
			}
		case agent.EventToolExecStart:
			fmt.Fprintf(os.Stderr, "\n[exec] %s\n", e.ToolName)
			if verbose && len(e.Args) > 0 {
				fmt.Fprintf(os.Stderr, "  args: %s\n", string(e.Args))
			}
		case agent.EventToolExecEnd:
			status := "ok"
			if e.IsError {
				status = "error"
			}
			fmt.Fprintf(os.Stderr, "[exec done] %s (%s)\n", e.ToolName, status)
		case agent.EventTurnEnd:
			if verbose {
				fmt.Fprintf(os.Stderr, "[turn end] ToolResults: %d\n", len(e.ToolResults))
			}
		case agent.EventAgentEnd:
			if verbose {
				fmt.Fprintf(os.Stderr, "[agent end]\n")
			}
		}
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	printSummary(detailed)
}

func runInteractive(config agent.AgentLoopConfig, verbose bool) {
	scanner := bufio.NewScanner(os.Stdin)
	messages := make([]core.Message, 0)

	fmt.Fprintln(os.Stderr, "Interactive mode (type :quit to exit)")

	for {
		fmt.Fprint(os.Stderr, "\nquery> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if strings.ToLower(input) == ":quit" || strings.ToLower(input) == ":exit" {
			break
		}

		// Add user message
		messages = append(messages, core.UserMessage{
			Role:      "user",
			Content:   input,
			Timestamp: time.Now(),
		})

		// Configure follow-up
		config.GetFollowUpMessages = func() []core.Message {
			return nil
		}

		// Run agent
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		stream, detailed := agent.AgentLoopDetailed(ctx, messages, config)

		_, err := stream.ForEach(ctx, func(evt agent.AgentEvent) error {
			switch e := evt.(type) {
			case agent.EventMessageUpdate:
				switch ae := e.AssistantEvent.(type) {
				case core.EventTextDelta:
					fmt.Print(ae.Delta)
				case core.EventThinkingDelta:
					if verbose {
						fmt.Fprintf(os.Stderr, "[thinking] %s", ae.Delta)
					}
				case core.EventToolCallStart:
					fmt.Fprintf(os.Stderr, "\n[tool] %s", ae.Name)
				case core.EventToolCallDelta:
					if len(ae.ArgumentsDelta) > 0 {
						fmt.Fprintf(os.Stderr, "%s", ae.ArgumentsDelta)
					}
				case core.EventToolCallEnd:
					fmt.Fprintf(os.Stderr, "\n")
				case core.EventError:
					fmt.Fprintf(os.Stderr, "\nError: %v\n", ae.Error)
				}
			case agent.EventMessageEnd:
				printAssistantMessage(e.Message)
				if e.Message.ErrorMessage != "" {
					fmt.Fprintf(os.Stderr, "\nError: %s\n", e.Message.ErrorMessage)
				}
			case agent.EventAgentEnd:
				messages = e.Messages
			case agent.EventToolExecStart:
				fmt.Fprintf(os.Stderr, "\n[exec] %s\n", e.ToolName)
				if verbose && len(e.Args) > 0 {
					fmt.Fprintf(os.Stderr, "  args: %s\n", string(e.Args))
				}
			case agent.EventToolExecEnd:
				status := "ok"
				if e.IsError {
					status = "error"
				}
				fmt.Fprintf(os.Stderr, "[exec done] %s (%s)\n", e.ToolName, status)
			case agent.EventTurnEnd:
				if verbose {
					fmt.Fprintf(os.Stderr, "[turn end]\n")
				}
			}
			return nil
		})

		cancel()

		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
			continue
		}

		fmt.Println()
		printSummary(detailed)
	}
}

func printAssistantMessage(msg core.AssistantMessage) {
	for _, block := range msg.Content {
		switch b := block.(type) {
		case core.TextContent:
			fmt.Print(b.Text)
		case core.ThinkingContent:
			fmt.Fprintf(os.Stderr, "[thinking] %s", b.Thinking)
		}
	}
}

func printSummary(detailed func() (agent.AgentLoopDetailedResult, error)) {
	fmt.Fprintf(os.Stderr, "\n[Summary] Getting detailed result...\n")
	res, err := detailed()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Summary] Error: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "[Summary] Steps: %d, Tools: %d, Cost: $%.4f\n",
		res.Summary.StepCount,
		res.Summary.ToolCallCount,
		res.Summary.TotalCost,
	)
}

func loadEnv() {
	file, err := os.Open("C:\\Users\\huangyicao\\Downloads\\hyperframes-test\\pi-ai-go\\examples\\agent-with-skills\\.env")
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

		// Remove quotes
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}

		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}
