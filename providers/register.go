package providers

import (
	"pi-ai-go/core"
	"pi-ai-go/providers/anthropic"
	"pi-ai-go/providers/compat"
	"pi-ai-go/providers/deepseek"
	"pi-ai-go/providers/glm"
	"pi-ai-go/providers/google"
	"pi-ai-go/providers/kimi"
	"pi-ai-go/providers/openai"
	images "pi-ai-go/providers/openrouter"
	"pi-ai-go/providers/xiaomi"
)

// RegisterBuiltInProviders registers all built-in API providers.
func RegisterBuiltInProviders() {
	core.RegisterProvider(core.APIAnthropicMessages, anthropic.New(), "builtin")

	// OpenAI-compatible router: aggregates openai + 4 third-party providers
	// (xiaomi, glm, deepseek, kimi). Dispatch is by model.Provider.
	openaiCompat := compat.NewRouter().
		WithConfig(openai.NewCompat()).
		WithConfig(xiaomi.New()).
		WithConfig(glm.New()).
		WithConfig(deepseek.New()).
		WithConfig(kimi.New())
	core.RegisterProvider(core.APIOpenAICompletions, openaiCompat, "builtin")

	core.RegisterProvider(core.APIOpenAIResponses, openai.NewResponses(), "builtin")
	core.RegisterProvider(core.APIAzureOpenAIResponses, openai.NewAzure(), "builtin")
	core.RegisterProvider(core.APIOpenAICodexResponses, openai.NewCodex(), "builtin")
	core.RegisterProvider(core.APIGoogleGenerative, google.New(), "builtin")
	core.RegisterProvider(core.APIGoogleVertex, google.NewVertex(), "builtin")
	core.RegisterImagesProvider("openrouter-images", images.NewOpenRouter(), "builtin")
}

// UnregisterBuiltInProviders removes all built-in providers.
func UnregisterBuiltInProviders() {
	core.UnregisterProviders("builtin")
	core.UnregisterImagesProviders("builtin")
}

func init() {
	RegisterBuiltInProviders()
}
