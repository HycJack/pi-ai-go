package providers

import piai "pi-ai-go"

// BuildBaseOptions extracts base StreamOptions from SimpleStreamOptions.
func BuildBaseOptions(model piai.Model, opts piai.SimpleStreamOptions, apiKey string) piai.StreamOptions {
	base := opts.StreamOptions
	if base.APIKey == "" {
		base.APIKey = apiKey
	}
	return base
}

// ClampReasoning clamps "xhigh" to "high" for providers that don't support it.
func ClampReasoning(effort piai.ThinkingLevel) piai.ThinkingLevel {
	if effort == piai.ThinkingXHigh {
		return piai.ThinkingHigh
	}
	return effort
}

// AdjustMaxTokensForThinking calculates thinking budget and total max tokens.
func AdjustMaxTokensForThinking(baseMaxTokens int, modelMaxTokens int, reasoningLevel piai.ThinkingLevel, customBudgets map[string]int) (thinkingBudget int, totalMaxTokens int) {
	if baseMaxTokens <= 0 {
		baseMaxTokens = modelMaxTokens
	}
	if baseMaxTokens <= 0 {
		baseMaxTokens = 4096
	}

	// Check for custom budget
	if customBudgets != nil {
		if budget, ok := customBudgets[string(reasoningLevel)]; ok {
			thinkingBudget = budget
		}
	}

	// Default thinking budgets by level
	if thinkingBudget == 0 {
		switch reasoningLevel {
		case piai.ThinkingMinimal:
			thinkingBudget = baseMaxTokens / 4
		case piai.ThinkingLow:
			thinkingBudget = baseMaxTokens / 2
		case piai.ThinkingMedium:
			thinkingBudget = baseMaxTokens
		case piai.ThinkingHigh:
			thinkingBudget = baseMaxTokens * 2
		case piai.ThinkingXHigh:
			thinkingBudget = baseMaxTokens * 4
		}
	}

	totalMaxTokens = baseMaxTokens + thinkingBudget

	// Cap at model max
	if modelMaxTokens > 0 && totalMaxTokens > modelMaxTokens {
		totalMaxTokens = modelMaxTokens
		thinkingBudget = totalMaxTokens - baseMaxTokens
	}

	return thinkingBudget, totalMaxTokens
}

// ResolveAPIKey resolves the API key from options or environment.
func ResolveAPIKey(model piai.Model, optsKey string) string {
	return piai.ResolveAPIKey(model.Provider, optsKey)
}
