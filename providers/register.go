package providers

import (
	piai "pi-ai-go"
	"pi-ai-go/providers/anthropic"
	"pi-ai-go/providers/bedrock"
	"pi-ai-go/providers/google"
	"pi-ai-go/providers/images"
	"pi-ai-go/providers/mistral"
	"pi-ai-go/providers/openai"
)

// RegisterBuiltInProviders registers all built-in API providers.
func RegisterBuiltInProviders() {
	// Anthropic
	piai.RegisterProvider(piai.APIAnthropicMessages, anthropic.New(), "builtin")

	// OpenAI Completions
	piai.RegisterProvider(piai.APIOpenAICompletions, openai.NewCompletions(), "builtin")

	// OpenAI Responses
	piai.RegisterProvider(piai.APIOpenAIResponses, openai.NewResponses(), "builtin")

	// Azure OpenAI
	piai.RegisterProvider(piai.APIAzureOpenAIResponses, openai.NewAzure(), "builtin")

	// OpenAI Codex
	piai.RegisterProvider(piai.APIOpenAICodexResponses, openai.NewCodex(), "builtin")

	// Google Gemini
	piai.RegisterProvider(piai.APIGoogleGenerative, google.New(), "builtin")

	// Google Vertex
	piai.RegisterProvider(piai.APIGoogleVertex, google.NewVertex(), "builtin")

	// Amazon Bedrock
	piai.RegisterProvider(piai.APIBedrockConverse, bedrock.New(), "builtin")

	// Mistral
	piai.RegisterProvider(piai.APIMistralConversations, mistral.New(), "builtin")

	// Image providers
	piai.RegisterImagesProvider("openrouter-images", images.NewOpenRouter(), "builtin")
}

// UnregisterBuiltInProviders removes all built-in providers.
func UnregisterBuiltInProviders() {
	piai.UnregisterProviders("builtin")
	piai.UnregisterImagesProviders("builtin")
}

func init() {
	RegisterBuiltInProviders()
}
