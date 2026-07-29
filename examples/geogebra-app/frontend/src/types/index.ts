export interface ProviderConfig {
  name: string;
  type: string;
  apiKey: string;
  baseUrl: string;
}

export interface Settings {
  providers: ProviderConfig[];
  currentProviderIndex: number;
  model: string;
  maxTokens: number;
  temperature: number;
  systemPrompt?: string;
}

export const DEFAULT_SETTINGS: Settings = {
  providers: [
    {
      name: 'OpenAI',
      type: 'openai',
      apiKey: '',
      baseUrl: 'https://api.openai.com/v1',
    },
  ],
  currentProviderIndex: 0,
  model: 'gpt-4o-mini',
  maxTokens: 4096,
  temperature: 1.0,
  systemPrompt: '',
};

export const PROVIDER_TYPES: { type: string; name: string; baseUrl: string }[] = [
  { type: 'openai', name: 'OpenAI', baseUrl: 'https://api.openai.com/v1' },
  { type: 'openai-compatible', name: 'OpenAI Compatible', baseUrl: 'https://api.example.com/v1' },
  { type: 'anthropic', name: 'Anthropic', baseUrl: 'https://api.anthropic.com/v1' },
  { type: 'google', name: 'Google', baseUrl: 'https://generativelanguage.googleapis.com/v1beta' },
  { type: 'deepseek', name: 'DeepSeek', baseUrl: 'https://api.deepseek.com' },
  { type: 'ollama', name: 'Ollama (Local)', baseUrl: 'http://localhost:11434' },
];

export function getCurrentProvider(settings: Settings): ProviderConfig | undefined {
  if (settings.providers.length === 0) return undefined;
  const idx = Math.min(settings.currentProviderIndex, settings.providers.length - 1);
  return settings.providers[idx];
}

export interface GeogebraResult {
  text: string;
  ggbCode: string;
  html: string;
  svg: string;
}

export interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: string;
  html?: string;
}

export interface Conversation {
  id: string;
  title: string;
  messages: Message[];
  result?: GeogebraResult;
  prompt?: string;
  timestamp: string;
}
