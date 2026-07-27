export interface Settings {
  providers: ProviderSetting[];
  currentProviderIndex: number;
  model: string;
  maxTokens: number;
  temperature: number;
}

export interface ProviderSetting {
  name: string;
  type: string;
  apiKey: string;
  baseUrl: string;
}

export interface Conversation {
  id: string;
  title: string;
  prompt: string;
  code: string;
  timestamp: string;
  messages: Message[];
}

export interface Message {
  role: string;
  content: string;
}

export interface ConversationSummary {
  id: string;
  title: string;
  timestamp: string;
}

export interface CodeGenRequest {
  prompt: string;
  provider: string;
  apiKey: string;
  baseUrl: string;
  model: string;
  maxTokens: number;
  temperature: number;
  currentCode?: string;
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
};

export function getCurrentProvider(settings: Settings): ProviderSetting | undefined {
  if (settings.providers.length === 0) return undefined;
  const idx = settings.currentProviderIndex;
  if (idx < 0 || idx >= settings.providers.length) return settings.providers[0];
  return settings.providers[idx];
}
