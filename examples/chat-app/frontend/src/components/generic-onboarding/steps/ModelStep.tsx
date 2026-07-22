/**
 * ModelStep.tsx — Step 2: Provider + Model configuration
 */

import { useState, useCallback } from 'react';
import type { ModelStepProps, ProviderOption, ModelInfo } from '../types';
import { StepContainer, Multiline, Actions, PrimaryButton, SecondaryButton } from '../ui';

export function ModelStep({
  config,
  modelConfig,
  goToStep,
  onModelChange,
  fetchModels,
  showError,
}: ModelStepProps) {
  const [provider, setProvider] = useState<string>(modelConfig.providerName);
  const [providerUrl, setProviderUrl] = useState(modelConfig.providerUrl);
  const [providerApi, setProviderApi] = useState(modelConfig.apiType);
  const [apiKey, setApiKey] = useState(modelConfig.apiKey);
  const [customName, setCustomName] = useState('');
  const [customUrl, setCustomUrl] = useState('');
  const [customApi, setCustomApi] = useState('openai-completions');

  const [fetchedModels, setFetchedModels] = useState<ModelInfo[]>([]);
  const [addedModels, setAddedModels] = useState<ModelInfo[]>(modelConfig.addedModels);
  const [selectedModel, setSelectedModel] = useState(modelConfig.chatModel);
  const [testing, setTesting] = useState(false);
  const [tested, setTested] = useState(false);

  const selectedPreset = config.providers.find((p) => p.value === provider);
  const isCustom = selectedPreset?.custom;
  const isLocal = selectedPreset?.local;

  const effectiveUrl = isCustom ? customUrl : providerUrl;
  const effectiveName = isCustom
    ? customName.trim().toLowerCase().replace(/\s+/g, '-') || provider
    : provider;
  const effectiveApi = isCustom ? customApi : providerApi;
  const effectiveKey = isLocal ? '' : apiKey;

  const canTest = !!(effectiveUrl && effectiveName && (isLocal || apiKey));

  const handleTest = useCallback(async () => {
    if (!canTest || !fetchModels) {
      setTested(true);
      return;
    }
    setTesting(true);
    try {
      // Build the provider option to pass to fetchModels, using the selected preset
      // and overriding url/apiKey for custom providers
      const preset = selectedPreset ?? config.providers[0];
      if (!preset) return;
      const providerOption: ProviderOption = isCustom
        ? { ...preset, url: customUrl, apiType: customApi as any, label: customName || preset.label }
        : preset;
      const models = await fetchModels(providerOption, effectiveKey);
      setFetchedModels(models);
      setTested(true);
    } catch (err: any) {
      showError(err.message ?? 'Connection failed');
    } finally {
      setTesting(false);
    }
  }, [canTest, fetchModels, selectedPreset, isCustom, customUrl, customApi, customName, effectiveKey, config.providers, showError]);

  const handleSelectPreset = useCallback((preset: ProviderOption) => {
    setProvider(preset.value);
    setProviderUrl(preset.url);
    setProviderApi(preset.apiType);
    setTested(false);
  }, []);

  const handleAddModel = useCallback((modelId: string) => {
    if (!modelId || addedModels.some((m) => m.id === modelId)) return;
    const next = [...addedModels, { id: modelId }];
    setAddedModels(next);
    if (!selectedModel) setSelectedModel(modelId);
  }, [addedModels, selectedModel]);

  const handleRemoveModel = useCallback((modelId: string) => {
    const next = addedModels.filter((m) => m.id !== modelId);
    setAddedModels(next);
    if (selectedModel === modelId) setSelectedModel(next[0]?.id ?? '');
  }, [addedModels, selectedModel]);

  const handleNext = useCallback(() => {
    if (!effectiveName || !selectedModel) return;
    onModelChange({
      chatModel: selectedModel,
      providerName: effectiveName,
      providerUrl: effectiveUrl,
      apiType: effectiveApi,
      apiKey: effectiveKey,
      addedModels,
    });
    goToStep(3 as any);
  }, [effectiveName, effectiveUrl, effectiveApi, effectiveKey, selectedModel, addedModels, onModelChange, goToStep]);

  return (
    <StepContainer>
      <h1 className="gonboarding-title">Configure AI Model</h1>
      <Multiline
        className="gonboarding-subtitle"
        text="Choose a provider and select your AI model."
      />

      {/* Provider selection */}
      <div className="gonboarding-provider-section">
        <div className="gonboarding-section-label">Provider</div>
        <div className="gonboarding-provider-grid">
          {config.providers.map((preset) => (
            <button
              key={preset.value}
              className={`gonboarding-provider-card${provider === preset.value ? ' selected' : ''}`}
              onClick={() => handleSelectPreset(preset)}
            >
              {preset.label || preset.value}
            </button>
          ))}
        </div>

        {isCustom && (
          <div className="gonboarding-custom-provider">
            <label className="gonboarding-field">
              <span className="gonboarding-field-label">Name</span>
              <input
                className="gonboarding-input"
                value={customName}
                onChange={(e) => setCustomName(e.target.value)}
                placeholder="my-provider"
              />
            </label>
            <label className="gonboarding-field">
              <span className="gonboarding-field-label">API URL</span>
              <input
                className="gonboarding-input"
                value={customUrl}
                onChange={(e) => setCustomUrl(e.target.value)}
                placeholder="https://api.example.com/v1"
              />
            </label>
            <label className="gonboarding-field">
              <span className="gonboarding-field-label">API Type</span>
              <select
                className="gonboarding-input"
                value={customApi}
                onChange={(e) => setCustomApi(e.target.value)}
              >
                <option value="openai-completions">OpenAI Compatible</option>
                <option value="anthropic">Anthropic</option>
                <option value="google">Google AI</option>
              </select>
            </label>
          </div>
        )}

        {!isLocal && (
          <label className="gonboarding-field">
            <span className="gonboarding-field-label">API Key</span>
            <input
              className="gonboarding-input"
              type="password"
              value={apiKey}
              onChange={(e) => { setApiKey(e.target.value); setTested(false); }}
              placeholder={isLocal ? 'Not required for local providers' : 'sk-...'}
            />
          </label>
        )}
      </div>

      {/* Test connection */}
      <button
        className="gonboarding-test-btn"
        onClick={handleTest}
        disabled={testing || !canTest}
      >
        {testing ? 'Testing...' : tested ? 'Connection OK ✓' : 'Test Connection'}
      </button>

      {/* Model list (shown after successful test) */}
      {tested && (
        <div className="gonboarding-model-section">
          <div className="gonboarding-section-label">Models</div>

          <div className="gonboarding-added-models">
            {addedModels.length === 0 ? (
              <div className="gonboarding-models-empty">
                Click a model below to add it, or test connection first.
              </div>
            ) : (
              <div className="gonboarding-model-list">
                {addedModels.map((model) => (
                  <div key={model.id} className="gonboarding-model-row">
                    <span className="gonboarding-model-name">
                      {model.name || model.id}
                    </span>
                    {selectedModel === model.id && (
                      <span className="gonboarding-model-badge">Main</span>
                    )}
                    <div className="gonboarding-model-actions">
                      {selectedModel !== model.id && (
                        <button onClick={() => setSelectedModel(model.id)}>
                          Set Main
                        </button>
                      )}
                      <button onClick={() => handleRemoveModel(model.id)}>
                        ✕
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Available models to add */}
          {fetchedModels.length > 0 && (
            <div className="gonboarding-available-models">
              <div className="gonboarding-section-label">Available Models</div>
              <div className="gonboarding-model-grid">
                {fetchedModels
                  .filter((m) => !addedModels.some((a) => a.id === m.id))
                  .map((model) => (
                    <button
                      key={model.id}
                      className="gonboarding-model-chip"
                      onClick={() => handleAddModel(model.id)}
                    >
                      {model.name || model.id}
                    </button>
                  ))}
              </div>
            </div>
          )}
        </div>
      )}

      <Actions>
        <SecondaryButton onClick={() => goToStep(1 as any)}>
          Back
        </SecondaryButton>
        <PrimaryButton onClick={handleNext} disabled={!selectedModel}>
          Next
        </PrimaryButton>
      </Actions>
    </StepContainer>
  );
}
