package providers

import (
	"github.com/HycJack/pi-ai-go/core"
	"github.com/HycJack/pi-ai-go/providers/anthropic"
	"github.com/HycJack/pi-ai-go/providers/bedrock"
	"github.com/HycJack/pi-ai-go/providers/compat"
	"github.com/HycJack/pi-ai-go/providers/deepseek"
	"github.com/HycJack/pi-ai-go/providers/glm"
	"github.com/HycJack/pi-ai-go/providers/google"
	"github.com/HycJack/pi-ai-go/providers/kimi"
	"github.com/HycJack/pi-ai-go/providers/mistral"
	"github.com/HycJack/pi-ai-go/providers/openai"
	"github.com/HycJack/pi-ai-go/providers/openrouter"
	"github.com/HycJack/pi-ai-go/providers/xiaomi"
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
	core.RegisterProvider(core.APIMistralConversations, mistral.New(), "builtin")
	core.RegisterProvider(core.APIBedrockConverse, bedrock.New(), "builtin")
	core.RegisterImagesProvider("openrouter-images", openrouter.NewOpenRouter(), "builtin")
}

// UnregisterBuiltInProviders removes all built-in providers.
func UnregisterBuiltInProviders() {
	core.UnregisterProviders("builtin")
	core.UnregisterImagesProviders("builtin")
}

func init() {
	RegisterBuiltInProviders()
}
