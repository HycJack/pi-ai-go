export interface ImageAttachment {
  /** Base64 data URL, e.g. "data:image/png;base64,..." */
  data: string;
  /** MIME type, e.g. "image/png" */
  mimeType: string;
  /** Optional display name */
  name?: string;
}

export interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: string;
  thinking?: string;
  toolCalls?: ToolCall[];
  images?: ImageAttachment[];
}

export interface ToolCall {
  id: string;
  name: string;
  arguments: string;
}

export interface Conversation {
  id: string;
  title: string;
  messages: Message[];
  timestamp: string;
}

export interface ProviderConfig {
  name: string;   // display name, e.g. "OpenAI"
  type: string;   // provider type, e.g. "openai", "anthropic"
  apiKey: string;
  baseUrl: string;
  apiKeys: string[]; // multiple keys for pooling
}

export interface AgentSettings {
  autoLearn: boolean;
  autoCompact: boolean;
  skillsDir: string;
}

export interface Settings {
  providers: ProviderConfig[];
  currentProviderIndex: number;
  model: string;
  maxTokens: number;
  temperature: number;
  reasoning: string;
  ttsEnabled: boolean;
  ttsVoice: string;
  agentMode: boolean;
  agentSettings: AgentSettings;
  locale?: string; // 'zh' | 'en', default 'zh'
}

export const DEFAULT_SETTINGS: Settings = {
  providers: [
    {
      name: 'OpenAI',
      type: 'openai',
      apiKey: '',
      apiKeys: [],
      baseUrl: 'https://api.openai.com/v1',
    },
  ],
  currentProviderIndex: 0,
  model: '',
  maxTokens: 4096,
  temperature: 1.0,
  reasoning: 'medium',
  ttsEnabled: false,
  ttsVoice: 'zh-CN',
  agentMode: true,
  agentSettings: {
    autoLearn: false,
    autoCompact: true,
    skillsDir: '',
  },
  locale: 'zh',
};

export const PROVIDER_TYPES: { type: string; name: string; baseUrl: string }[] = [
  { type: 'openai', name: 'OpenAI', baseUrl: 'https://api.openai.com/v1' },
  { type: 'openai-compatible', name: 'OpenAI Compatible', baseUrl: 'https://api.example.com/v1' },
  { type: 'anthropic', name: 'Anthropic', baseUrl: 'https://api.anthropic.com/v1' },
  { type: 'google', name: 'Google', baseUrl: 'https://generativelanguage.googleapis.com/v1beta' },
  { type: 'deepseek', name: 'DeepSeek', baseUrl: 'https://api.deepseek.com' },
  { type: 'mistral', name: 'Mistral', baseUrl: 'https://api.mistral.ai/v1' },
  { type: 'ollama', name: 'Ollama (Local)', baseUrl: 'http://localhost:11434' },
];

export function getProviderTypeName(type: string): string {
  return PROVIDER_TYPES.find((p) => p.type === type)?.name || type;
}

export function getCurrentProvider(settings: Settings): ProviderConfig | undefined {
  if (settings.providers.length === 0) return undefined;
  const idx = Math.min(settings.currentProviderIndex, settings.providers.length - 1);
  return settings.providers[idx];
}

export interface MemoryEntry {
  key: string;
  value: string;
  category?: string;
}
