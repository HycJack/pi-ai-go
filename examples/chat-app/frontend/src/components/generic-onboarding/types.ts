/**
 * types.ts — Core types for the generic AI Chat Onboarding wizard
 *
 * Designed to be framework-agnostic (data layer) with React bindings in ui.tsx.
 */

// ── Step identifiers ──

export enum OnboardingStep {
  Welcome = 0,
  Language = 1,
  Model = 2,
  Theme = 3,
  Workspace = 4,
  Complete = 5,
}

export const TOTAL_STEPS = 6;

// ── Locale ──

export interface LocaleOption {
  value: string;
  label: string;
}

// ── Model / Provider ──

export interface ProviderOption {
  value: string;
  label: string;
  url: string;
  apiType: string;
  /** If true, apiKey is optional */
  local?: boolean;
  /** If true, user must fill in custom fields */
  custom?: boolean;
}

export interface ModelInfo {
  id: string;
  name?: string;
  context?: number;
  maxOutput?: number;
  vision?: boolean;
  reasoning?: boolean;
}

export interface ModelConfig {
  chatModel: string;
  providerName: string;
  providerUrl: string;
  apiType: string;
  apiKey: string;
  addedModels: ModelInfo[];
  utilityModel?: string;
  utilityLargeModel?: string;
}

// ── Theme ──

export interface ThemeOption {
  id: string;
  label: string;
  description?: string;
}

// ── Workspace ──

export interface WorkspaceConfig {
  path: string;
  useDefault: boolean;
}

// ── User identity ──

export interface UserIdentity {
  userName: string;
  agentName: string;
  memoryEnabled: boolean;
}

// ── Complete onboarding result ──

export interface OnboardingResult {
  step: OnboardingStep;
  locale: string;
  identity: UserIdentity;
  model: ModelConfig;
  theme: string;
  workspace: WorkspaceConfig;
}

// ── Configuration for the onboarding wizard ──

export interface OnboardingConfig {
  /** Available locales */
  locales: LocaleOption[];
  /** Available LLM providers */
  providers: ProviderOption[];
  /** Available themes */
  themes: ThemeOption[];
  /** Default workspace directory name */
  defaultWorkspaceDirname: string;
  /** Agent avatar URL (optional) */
  avatarSrc?: string;
  /** Whether running in preview/dev mode (skips real API calls) */
  preview?: boolean;
  /** Welcome screen branding */
  branding?: {
    appName: string;
    tagline: string;
    welcomeTitle: string;
    welcomeSubtitle: string;
  };
}

// ── Step props passed to each step component ──

export interface StepProps {
  /** Navigate to a specific step */
  goToStep: (step: OnboardingStep) => void;
  /** Show an error toast */
  showError: (message: string) => void;
}

export interface WelcomeStepProps extends StepProps {
  config: OnboardingConfig;
  avatarSrc: string;
  onComplete: (result: OnboardingResult) => void | Promise<void>;
}

export interface LanguageStepProps extends StepProps {
  config: OnboardingConfig;
  initialLocale: string;
  onLocaleChange: (locale: string) => void;
}

export interface ModelStepProps extends StepProps {
  config: OnboardingConfig;
  modelConfig: ModelConfig;
  onModelChange: (model: ModelConfig) => void;
  /** Async function to fetch models from a provider */
  fetchModels?: (provider: ProviderOption, apiKey: string) => Promise<ModelInfo[]>;
}

export interface ThemeStepProps extends StepProps {
  config: OnboardingConfig;
  activeTheme: string;
  onThemeChange: (theme: string) => void;
}

export interface WorkspaceStepProps extends StepProps {
  config: OnboardingConfig;
  workspace: WorkspaceConfig;
  onWorkspaceChange: (workspace: WorkspaceConfig) => void;
  /** System folder picker (inject for Electron/web) */
  selectFolder?: () => Promise<string | null>;
}

export interface CompleteStepProps extends StepProps {
  config: OnboardingConfig;
  result: OnboardingResult;
  onFinish: () => void | Promise<void>;
}
