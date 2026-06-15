import { useState, useEffect } from 'react';
import { X, Save, RefreshCw, ChevronDown } from 'lucide-react';
import { GetModels } from '../../wailsjs/go/main/App';

interface Settings {
  provider: 'openai' | 'anthropic';
  apiKey: string;
  baseUrl: string;
  model: string;
  customModel: string;
  ttsEnabled: boolean;
  ttsVoice: string;
}

interface SettingsPanelProps {
  isOpen: boolean;
  onClose: () => void;
  currentSettings: Settings;
  onSave: (settings: Settings) => void;
}

interface ModelInfo {
  id: string;
  name: string;
}

export default function SettingsPanel({ isOpen, onClose, currentSettings, onSave }: SettingsPanelProps) {
  const [settings, setSettings] = useState<Settings>(currentSettings);
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [showCustomModel, setShowCustomModel] = useState(false);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    setSettings(currentSettings);
    setSaved(false);
  }, [currentSettings, isOpen]);

  const fetchModels = async () => {
    setIsLoading(true);
    try {
      const modelList = await GetModels({ 
        provider: settings.provider,
        baseUrl: settings.baseUrl,
        apiKey: settings.apiKey
      });
      setModels(modelList);
      if (modelList.length > 0 && !settings.model) {
        setSettings(prev => ({ ...prev, model: modelList[0].id }));
      }
    } catch (error) {
      console.error('Failed to fetch models:', error);
    }
    setIsLoading(false);
  };

  const handleProviderChange = (provider: 'openai' | 'anthropic') => {
    setSettings(prev => ({
      ...prev,
      provider,
      model: '',
      baseUrl: provider === 'openai' ? 'https://api.openai.com/v1' : 'https://api.anthropic.com/v1'
    }));
    setShowCustomModel(false);
  };

  const handleModelChange = (model: string) => {
    setSettings(prev => ({ ...prev, model }));
    setShowCustomModel(model === 'custom');
    if (model !== 'custom') {
      setSettings(prev => ({ ...prev, customModel: '' }));
    }
  };

  const handleSave = () => {
    const finalSettings = {
      ...settings,
      model: settings.model === 'custom' ? settings.customModel : settings.model
    };
    onSave(finalSettings);
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-slate-800 rounded-xl shadow-2xl w-full max-w-md mx-4 overflow-hidden">
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-700">
          <h2 className="text-lg font-semibold text-white">Settings</h2>
          <button
            onClick={onClose}
            className="p-2 hover:bg-slate-700 rounded-lg transition-colors"
          >
            <X className="w-5 h-5 text-slate-400" />
          </button>
        </div>

        <div className="p-6 space-y-6">
          <div>
            <label className="block text-sm font-medium text-slate-300 mb-2">Provider</label>
            <div className="flex gap-2">
              <button
                onClick={() => handleProviderChange('openai')}
                className={`flex-1 py-2.5 px-4 rounded-lg text-sm font-medium transition-colors ${
                  settings.provider === 'openai'
                    ? 'bg-blue-600 text-white'
                    : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
                }`}
              >
                OpenAI
              </button>
              <button
                onClick={() => handleProviderChange('anthropic')}
                className={`flex-1 py-2.5 px-4 rounded-lg text-sm font-medium transition-colors ${
                  settings.provider === 'anthropic'
                    ? 'bg-blue-600 text-white'
                    : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
                }`}
              >
                Anthropic
              </button>
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-300 mb-2">API Key</label>
            <input
              type="password"
              value={settings.apiKey}
              onChange={(e) => setSettings(prev => ({ ...prev, apiKey: e.target.value }))}
              placeholder="Enter your API key"
              className="w-full px-4 py-2.5 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
            />
            <p className="text-xs text-slate-500 mt-1">
              {settings.provider === 'openai' ? 'OPENAI_API_KEY' : 'ANTHROPIC_API_KEY'}
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-300 mb-2">Base URL</label>
            <input
              type="text"
              value={settings.baseUrl}
              onChange={(e) => setSettings(prev => ({ ...prev, baseUrl: e.target.value }))}
              placeholder="Enter base URL"
              className="w-full px-4 py-2.5 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-300 mb-2">Model</label>
            <div className="relative flex gap-2">
              <select
                value={settings.model}
                onChange={(e) => handleModelChange(e.target.value)}
                disabled={isLoading}
                className="flex-1 px-4 py-2.5 bg-slate-700 border border-slate-600 rounded-lg text-white appearance-none focus:outline-none focus:border-blue-500 cursor-pointer"
              >
                <option value="" disabled>Select a model</option>
                {models.map((model) => (
                  <option key={model.id} value={model.id}>
                    {model.name} ({model.id})
                  </option>
                ))}
                <option value="custom">Custom Model...</option>
              </select>
              <button
                onClick={fetchModels}
                disabled={isLoading}
                className="px-3 py-2.5 bg-slate-700 border border-slate-600 rounded-lg hover:bg-slate-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                title="Refresh models"
              >
                <RefreshCw className={`w-5 h-5 text-slate-400 ${isLoading ? 'animate-spin' : ''}`} />
              </button>
            </div>
            {showCustomModel && (
              <input
                type="text"
                value={settings.customModel}
                onChange={(e) => setSettings(prev => ({ ...prev, customModel: e.target.value }))}
                placeholder="Enter custom model ID"
                className="mt-2 w-full px-4 py-2.5 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
              />
            )}
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-300 mb-2">Text-to-Speech (TTS)</label>
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm text-slate-400">Enable TTS</span>
              <button
                onClick={() => setSettings(prev => ({ ...prev, ttsEnabled: !prev.ttsEnabled }))}
                className={`relative w-12 h-6 rounded-full transition-colors ${
                  settings.ttsEnabled ? 'bg-blue-600' : 'bg-slate-600'
                }`}
              >
                <span
                  className={`absolute top-1 w-4 h-4 rounded-full bg-white transition-transform ${
                    settings.ttsEnabled ? 'translate-x-7' : 'translate-x-1'
                  }`}
                />
              </button>
            </div>
            {settings.ttsEnabled && (
              <select
                value={settings.ttsVoice}
                onChange={(e) => setSettings(prev => ({ ...prev, ttsVoice: e.target.value }))}
                className="w-full px-4 py-2.5 bg-slate-700 border border-slate-600 rounded-lg text-white appearance-none focus:outline-none focus:border-blue-500 cursor-pointer"
              >
                <option value="zh-CN">Chinese (Mandarin)</option>
                <option value="en-US">English (US)</option>
                <option value="en-GB">English (UK)</option>
                <option value="ja-JP">Japanese</option>
                <option value="ko-KR">Korean</option>
              </select>
            )}
          </div>
        </div>

        <div className="flex gap-3 px-6 py-4 border-t border-slate-700 bg-slate-800/50">
          <button
            onClick={onClose}
            className="flex-1 py-2.5 px-4 bg-slate-700 text-slate-300 rounded-lg font-medium hover:bg-slate-600 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            className={`flex-1 py-2.5 px-4 rounded-lg font-medium transition-colors flex items-center justify-center gap-2 ${
              saved
                ? 'bg-green-600 text-white'
                : 'bg-blue-600 text-white hover:bg-blue-500'
            }`}
          >
            <Save className="w-4 h-4" />
            {saved ? 'Saved!' : 'Save'}
          </button>
        </div>
      </div>
    </div>
  );
}
