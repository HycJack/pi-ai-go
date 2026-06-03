package providers

import (
	"testing"

	piai "pi-ai-go"
)

func TestRegisterBuiltInProviders(t *testing.T) {
	// The init() function should have registered all built-in providers
	apis := piai.GetRegisteredProviders()

	expectedAPIs := []piai.KnownAPI{
		piai.APIAnthropicMessages,
		piai.APIOpenAICompletions,
		piai.APIOpenAIResponses,
		piai.APIAzureOpenAIResponses,
		piai.APIOpenAICodexResponses,
		piai.APIGoogleGenerative,
		piai.APIGoogleVertex,
		// Bedrock and Mistral are not currently built in; they remain
		// declared as KnownAPI for forward compatibility.
	}

	for _, expected := range expectedAPIs {
		found := false
		for _, api := range apis {
			if api == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected API '%s' to be registered", expected)
		}
	}
}

func TestUnregisterBuiltInProviders(t *testing.T) {
	UnregisterBuiltInProviders()
	defer RegisterBuiltInProviders()

	apis := piai.GetRegisteredProviders()
	if len(apis) != 0 {
		t.Errorf("expected 0 providers after unregister, got %d", len(apis))
	}
}

func TestImagesProviderRegistered(t *testing.T) {
	apis := piai.GetRegisteredImagesProviders()

	found := false
	for _, api := range apis {
		if api == "openrouter-images" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected openrouter-images to be registered")
	}
}
