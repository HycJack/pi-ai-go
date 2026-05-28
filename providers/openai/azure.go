package openai

import (
	"encoding/json"
	"fmt"
	"os"

	piai "pi-ai-go"
)

// AzureOptions holds Azure-specific options.
type AzureOptions struct {
	ReasoningEffort    string `json:"reasoningEffort,omitempty"`
	ReasoningSummary   string `json:"reasoningSummary,omitempty"`
	AzureAPIVersion    string `json:"azureApiVersion,omitempty"`
	AzureResourceName  string `json:"azureResourceName,omitempty"`
	AzureBaseURL       string `json:"azureBaseUrl,omitempty"`
	AzureDeploymentName string `json:"azureDeploymentName,omitempty"`
}

// AzureProvider implements the Azure OpenAI Responses API.
type AzureProvider struct{}

// NewAzure creates a new Azure OpenAI provider.
func NewAzure() *AzureProvider {
	return &AzureProvider{}
}

func (p *AzureProvider) Stream(model piai.Model, ctx piai.Context, opts piai.StreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	return streamAzure(model, ctx, opts, AzureOptions{})
}

func (p *AzureProvider) StreamSimple(model piai.Model, ctx piai.Context, opts piai.SimpleStreamOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	azureOpts := AzureOptions{}
	if opts.Reasoning != "" {
		azureOpts.ReasoningEffort = string(opts.Reasoning)
	}
	return streamAzure(model, ctx, opts.StreamOptions, azureOpts)
}

func streamAzure(model piai.Model, c piai.Context, opts piai.StreamOptions, azureOpts AzureOptions) (*piai.EventStream[piai.AssistantMessageEvent, piai.AssistantMessage], error) {
	apiKey := piai.ResolveAPIKey(model.Provider, opts.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("azure: no API key provided")
	}

	// Resolve Azure-specific configuration
	baseURL := resolveAzureBaseURL(model, azureOpts)
	deploymentName := resolveAzureDeploymentName(model, azureOpts)
	apiVersion := azureOpts.AzureAPIVersion
	if apiVersion == "" {
		apiVersion = "2025-03-01-preview"
	}

	// Modify the model to use Azure-specific settings
	azureModel := model
	azureModel.BaseURL = fmt.Sprintf("%s/openai/deployments/%s", baseURL, deploymentName)
	azureModel.Headers = mergeMaps(model.Headers, map[string]string{
		"api-key": apiKey,
	})

	return streamResponses(azureModel, c, opts, ResponsesOptions{
		ReasoningEffort:  azureOpts.ReasoningEffort,
		ReasoningSummary: azureOpts.ReasoningSummary,
	})
}

func resolveAzureBaseURL(model piai.Model, opts AzureOptions) string {
	if opts.AzureBaseURL != "" {
		return opts.AzureBaseURL
	}
	if model.BaseURL != "" {
		return model.BaseURL
	}
	if resourceName := opts.AzureResourceName; resourceName != "" {
		return fmt.Sprintf("https://%s.openai.azure.com", resourceName)
	}
	if resourceName := os.Getenv("AZURE_OPENAI_RESOURCE_NAME"); resourceName != "" {
		return fmt.Sprintf("https://%s.openai.azure.com", resourceName)
	}
	return ""
}

func resolveAzureDeploymentName(model piai.Model, opts AzureOptions) string {
	if opts.AzureDeploymentName != "" {
		return opts.AzureDeploymentName
	}
	// Try environment variable mapping
	if mapping := os.Getenv("AZURE_OPENAI_DEPLOYMENT_NAME_MAP"); mapping != "" {
		// Parse JSON mapping: {"model-id": "deployment-name"}
		var m map[string]string
		if err := jsonUnmarshal(mapping, &m); err == nil {
			if deployment, ok := m[model.ID]; ok {
				return deployment
			}
		}
	}
	return model.ID
}

func jsonUnmarshal(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}

func mergeMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
