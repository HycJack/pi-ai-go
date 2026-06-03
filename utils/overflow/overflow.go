// Package overflow provides context overflow detection for LLM responses.
//
// The patterns here cover 20+ provider error variants, distilled from
// oh-my-pi's `packages/ai/src/utils/overflow.ts`. The list is intentionally
// permissive — false positives are cheap (they trigger compaction), false
// negatives destroy the conversation.
package overflow

import (
	"regexp"
	"strings"
)

// overflowPatterns matches known provider error messages indicating
// context overflow. Organized in the same order as the oh-my-pi source.
var overflowPatterns = []*regexp.Regexp{
	// Anthropic
	regexp.MustCompile(`(?i)prompt is too long`),
	regexp.MustCompile(`(?i)request_too_large`),
	regexp.MustCompile(`(?i)request exceeds the maximum size`),
	// Generic HTTP 413 variants
	regexp.MustCompile(`(?i)payload too large`),
	regexp.MustCompile(`(?i)entity too large`),
	regexp.MustCompile(`(?i)\b413\b.*\b(request|payload|entity)\b.*\btoo large\b`),
	// Amazon Bedrock
	regexp.MustCompile(`(?i)input is too long for requested model`),
	// OpenAI (Completions & Responses)
	regexp.MustCompile(`(?i)exceeds? the context window`),
	regexp.MustCompile(`(?i)exceeds? (?:the )?(?:model'?s )?(?:maximum )?context length`),
	regexp.MustCompile(`(?i)exceeds? (?:the )?(?:model'?s )?maximum context length of [\d,]+ tokens?`),
	// Google (Gemini)
	regexp.MustCompile(`(?i)input token count.*exceeds the maximum`),
	// xAI (Grok)
	regexp.MustCompile(`(?i)maximum prompt length is \d+`),
	// Groq
	regexp.MustCompile(`(?i)reduce the length of the messages`),
	// OpenRouter
	regexp.MustCompile(`(?i)maximum context length is \d+ tokens`),
	regexp.MustCompile(`(?i)exceeds (?:the )?maximum allowed input length of [\d,]+ tokens?`),
	// llama.cpp
	regexp.MustCompile(`(?i)exceeds the available context size`),
	regexp.MustCompile(`(?i)requested tokens?.*exceed.*context (window|length|size)`),
	regexp.MustCompile(`(?i)context (window|length|size).*(exceeded|overflow|too small)`),
	regexp.MustCompile(`(?i)(prompt|input).*(too long|too large).*(context|n_ctx)`),
	regexp.MustCompile(`(?i)requested tokens?.*(exceeds?|greater than).*(n_ctx|context)`),
	// LM Studio
	regexp.MustCompile(`(?i)greater than the context length`),
	// GitHub Copilot
	regexp.MustCompile(`(?i)exceeds the limit of \d+`),
	// MiniMax
	regexp.MustCompile(`(?i)context window exceeds limit`),
	// Kimi For Coding
	regexp.MustCompile(`(?i)exceeded model token limit`),
	// Mistral
	regexp.MustCompile(`(?i)too large for model with \d+ maximum context length`),
	// z.ai
	regexp.MustCompile(`(?i)model_context_window_exceeded`),
	// Ollama
	regexp.MustCompile(`(?i)prompt too long; exceeded (?:max )?context length`),
	// Generic fallbacks
	regexp.MustCompile(`(?i)context[_ ]length[_ ]exceeded`),
	regexp.MustCompile(`(?i)too many tokens`),
	regexp.MustCompile(`(?i)token limit exceeded`),
	// Cerebras: 400/413 with no body (HTTP prefix optional)
	regexp.MustCompile(`(?i)^(?:HTTP\s+)?4(?:00|13)\s*(?:status code)?\s*\(no body\)`),
}

// nonOverflowPatterns excludes throttling / rate-limit errors that LOOK
// like overflow from auto-detection (they should be retried, not compacted).
var nonOverflowPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(throttling error|service unavailable):`),
	regexp.MustCompile(`(?i)rate limit`),
	regexp.MustCompile(`(?i)too many requests`),
}

// IsOverflow detects whether an error message or usage indicates context
// overflow. If contextWindow is 0, only error-based detection is used.
func IsOverflow(errMsg string, contextWindow int, usage int) bool {
	if errMsg != "" {
		lower := strings.ToLower(errMsg)
		// First exclude non-overflow patterns (rate limits, throttling).
		for _, p := range nonOverflowPatterns {
			if p.MatchString(lower) {
				return false
			}
		}
		for _, p := range overflowPatterns {
			if p.MatchString(lower) {
				return true
			}
		}
	}
	// Silent overflow: usage exceeds context window.
	if contextWindow > 0 && usage > contextWindow {
		return true
	}
	return false
}

// GetPatterns returns the overflow patterns for testing.
func GetPatterns() []*regexp.Regexp {
	return overflowPatterns
}
