/**
 * OnboardingApp.tsx — Main orchestration component for the generic AI Chat onboarding wizard
 *
 * This is the public API entry point. Consumers pass a config object and
 * receive the final result via onComplete callback.
 */

import { useState, useCallback, useRef } from 'react';
import type { OnboardingConfig, OnboardingResult, ModelConfig, WorkspaceConfig, ProviderOption, ModelInfo } from './types';
import { OnboardingStep, TOTAL_STEPS } from './types';
import { ProgressDots, Toast } from './ui';
import { WelcomeStep } from './steps/WelcomeStep';
import { LanguageStep } from './steps/LanguageStep';
import { ModelStep } from './steps/ModelStep';
import { ThemeStep } from './steps/ThemeStep';
import { WorkspaceStep } from './steps/WorkspaceStep';
import { CompleteStep } from './steps/CompleteStep';
import { GetModels } from '../../../wailsjs/go/main/App';

export { OnboardingStep, TOTAL_STEPS };
export type { OnboardingConfig, OnboardingResult, ModelConfig, WorkspaceConfig };

// ── Default config ──

const DEFAULT_LOCALES = [
  { value: 'zh', label: '简体中文' },
  { value: 'en', label: 'English' },
  // { value: 'ja', label: '日本語' },
  // { value: 'ko', label: '한국어' },
];

const DEFAULT_PROVIDERS: ProviderOption[] = [
  { value: 'openai', label: 'OpenAI', url: 'https://api.openai.com/v1', apiType: 'openai', local: false },
  { value: 'anthropic', label: 'Anthropic', url: 'https://api.anthropic.com/v1', apiType: 'anthropic', local: false },
  { value: 'google', label: 'Google AI', url: 'https://generativelanguage.googleapis.com/v1beta', apiType: 'google', local: false },
  { value: 'deepseek', label: 'DeepSeek', url: 'https://api.deepseek.com/v1', apiType: 'openai', local: false },
  { value: 'mistral', label: 'Mistral', url: 'https://api.mistral.ai/v1', apiType: 'openai', local: false },
  { value: 'ollama', label: 'Ollama (Local)', url: 'http://localhost:11434/v1', apiType: 'openai', local: true },
  { value: '_custom', label: 'Custom', url: '', apiType: 'openai', local: false, custom: true },
];

const DEFAULT_THEMES = [
  { id: 'light', label: 'Light' },
  { id: 'dark', label: 'Dark' },
  { id: 'auto', label: 'Auto' },
  { id: 'high-contrast', label: 'High Contrast' },
];

// ── Props ──

interface OnboardingAppProps {
  /** Onboarding configuration */
  config?: Partial<OnboardingConfig>;
  /** Called when all steps are completed */
  onComplete?: (result: OnboardingResult) => void | Promise<void>;
  /** Called when the user finishes the final step */
  onFinish?: () => void | Promise<void>;
  /** Async function to fetch models from a provider (defaults to wails GetModels) */
  fetchModels?: (provider: ProviderOption, apiKey: string) => Promise<ModelInfo[]>;
  /** System folder picker (inject for Electron/web) */
  selectFolder?: () => Promise<string | null>;
  /** Skip directly to the final tutorial/complete step */
  skipToComplete?: boolean;
}

// ── Default fetchModels: bridge to chat-app's wails GetModels API ──

const DEFAULT_FETCH_MODELS = async (provider: ProviderOption, apiKey: string): Promise<ModelInfo[]> => {
  const list = await GetModels({
    provider: provider.apiType,
    baseUrl: provider.url,
    apiKey,
  });
  if (!list) return [];
  return list.map((m: any) => ({
    id: m.id,
    name: m.name || m.id,
    reasoning: m.reasoning,
  }));
};

export function OnboardingApp({
  config: partialConfig,
  onComplete,
  onFinish,
  fetchModels = DEFAULT_FETCH_MODELS,
  selectFolder,
  skipToComplete = false,
}: OnboardingAppProps) {
  const config: OnboardingConfig = {
    locales: partialConfig?.locales ?? DEFAULT_LOCALES,
    providers: partialConfig?.providers ?? DEFAULT_PROVIDERS,
    themes: partialConfig?.themes ?? DEFAULT_THEMES,
    defaultWorkspaceDirname: partialConfig?.defaultWorkspaceDirname ?? 'ai-workspace',
    avatarSrc: partialConfig?.avatarSrc,
    preview: partialConfig?.preview ?? false,
    branding: partialConfig?.branding ?? {
      appName: 'Pi-AI Chat',
      tagline: '你的智能助手',
      welcomeTitle: '欢迎使用 Pi-AI Chat',
      welcomeSubtitle: '选择语言开始使用',
    },
  };

  const [step, setStep] = useState(skipToComplete ? OnboardingStep.Complete : OnboardingStep.Welcome);
  const [stepKey, setStepKey] = useState(0);
  const [toastMsg, setToastMsg] = useState('');
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Accumulated result
  const [locale, setLocale] = useState(config.locales[0]?.value ?? 'en');
  const [theme, setTheme] = useState(config.themes[0]?.id ?? 'light');
  const [modelConfig, setModelConfig] = useState<ModelConfig>({
    chatModel: '',
    providerName: '',
    providerUrl: '',
    apiType: 'openai',
    apiKey: '',
    addedModels: [],
  });
  const [workspace, setWorkspace] = useState<WorkspaceConfig>({
    path: '',
    useDefault: true,
  });

  const showError = useCallback((msg: string) => {
    setToastMsg(msg);
    if (toastTimer.current) clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setToastMsg(''), 3000);
  }, []);

  const goToStep = useCallback((index: OnboardingStep) => {
    if (index < OnboardingStep.Welcome || index > OnboardingStep.Complete) return;
    setStepKey((k) => k + 1);
    setStep(index);
  }, []);

  const buildResult = useCallback((): OnboardingResult => ({
    step,
    locale,
    identity: {
      userName: '',
      agentName: '',
      memoryEnabled: true,
    },
    model: modelConfig,
    theme,
    workspace,
  }), [step, locale, modelConfig, theme, workspace]);

  const handleWelcomeComplete = useCallback(async (result: any) => {
    if (result.locale) setLocale(result.locale);
    // Welcome step already collects locale, skip the redundant Language step
    goToStep(OnboardingStep.Model);
  }, [goToStep]);

  const handleLocaleChange = useCallback((newLocale: string) => {
    setLocale(newLocale);
  }, []);

  const handleModelChange = useCallback((model: ModelConfig) => {
    setModelConfig(model);
  }, []);

  const handleThemeChange = useCallback((newTheme: string) => {
    setTheme(newTheme);
  }, []);

  const handleWorkspaceChange = useCallback((ws: WorkspaceConfig) => {
    setWorkspace(ws);
  }, []);

  const handleFinish = useCallback(async () => {
    const result = buildResult();
    if (onComplete) {
      await onComplete(result);
    }
    if (onFinish) {
      await onFinish();
    }
  }, [buildResult, onComplete, onFinish]);

  const stepLabels = ['Welcome', 'Language', 'Model', 'Theme', 'Workspace', 'Complete'];

  return (
    <div className="gonboarding">
      <ProgressDots total={TOTAL_STEPS} current={step} labels={stepLabels} />

      {step === OnboardingStep.Welcome && (
        <WelcomeStep
          key={`step-0-${stepKey}`}
          config={config}
          avatarSrc={config.avatarSrc ?? ''}
          goToStep={goToStep}
          showError={showError}
          onComplete={handleWelcomeComplete}
        />
      )}

      {step === OnboardingStep.Language && (
        <LanguageStep
          key={`step-1-${stepKey}`}
          config={config}
          initialLocale={locale}
          goToStep={goToStep}
          showError={showError}
          onLocaleChange={handleLocaleChange}
        />
      )}

      {step === OnboardingStep.Model && (
        <ModelStep
          key={`step-2-${stepKey}`}
          config={config}
          modelConfig={modelConfig}
          goToStep={goToStep}
          showError={showError}
          onModelChange={handleModelChange}
          fetchModels={fetchModels}
        />
      )}

      {step === OnboardingStep.Theme && (
        <ThemeStep
          key={`step-3-${stepKey}`}
          config={config}
          activeTheme={theme}
          goToStep={goToStep}
          showError={showError}
          onThemeChange={handleThemeChange}
        />
      )}

      {step === OnboardingStep.Workspace && (
        <WorkspaceStep
          key={`step-4-${stepKey}`}
          config={config}
          workspace={workspace}
          goToStep={goToStep}
          showError={showError}
          onWorkspaceChange={handleWorkspaceChange}
          selectFolder={selectFolder}
        />
      )}

      {step === OnboardingStep.Complete && (
        <CompleteStep
          key={`step-5-${stepKey}`}
          config={config}
          result={buildResult()}
          goToStep={goToStep}
          showError={showError}
          onFinish={handleFinish}
        />
      )}

      <Toast message={toastMsg} />
    </div>
  );
}

export default OnboardingApp;
