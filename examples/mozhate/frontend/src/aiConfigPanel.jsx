import React, { useState, useEffect } from 'react';
import {
    X,
    Sparkles,
    Key,
    Globe,
    Sliders,
    MessageSquare,
    Bot,
    Server,
    Check,
    AlertCircle
} from 'lucide-react';
import { getAIConfig, saveAIConfig } from './wailsApi';

const DEFAULT_PROMPT_BASE = `Generate a list of Chinese words or phrases related to the topic: "%s".
Return the result as a simple JSON array of strings.
Keep it between %d words/phrases.
Example: ["勤学苦练", "积极向上", "自强不息"]`;

function AIConfigPanel({ visible, onClose }) {
    const [config, setConfig] = useState({
        provider: 'gemini',
        model: 'gemini-2.0-flash',
        apiKey: '',
        endpoint: '',
        promptBase: DEFAULT_PROMPT_BASE,
        maxWords: 15,
        useTracing: true,
    });
    const [saving, setSaving] = useState(false);
    const [saved, setSaved] = useState(false);
    const [error, setError] = useState('');
    const [testing, setTesting] = useState(false);

    useEffect(() => {
        if (visible) {
            getAIConfig().then(cfg => {
                if (cfg) {
                    setConfig(prev => ({
                        ...prev,
                        ...cfg,
                        promptBase: cfg.promptBase || DEFAULT_PROMPT_BASE,
                    }));
                }
            });
        }
    }, [visible]);

    const update = (key, value) => {
        setConfig(prev => ({ ...prev, [key]: value }));
        setSaved(false);
        setError('');
    };

    const handleSave = async () => {
        if (!config.apiKey.trim()) {
            setError('请输入 API 密钥');
            return;
        }
        setSaving(true);
        setError('');
        try {
            const ok = await saveAIConfig(config);
            if (ok) {
                setSaved(true);
                setTimeout(() => setSaved(false), 3000);
            } else {
                setError('保存失败');
            }
        } catch (e) {
            setError('保存失败: ' + e.message);
        } finally {
            setSaving(false);
        }
    };

    const providerOptions = [
        { value: 'gemini', label: 'Google Gemini', desc: 'gemini-2.0-flash / gemini-1.5-pro' },
        { value: 'openai', label: 'OpenAI', desc: 'gpt-4o / gpt-4o-mini / gpt-4' },
        { value: 'openai-compatible', label: 'OpenAI 兼容', desc: 'DeepSeek, Moonshot, Qwen 等' },
    ];

    const modelSuggestions = {
        gemini: ['gemini-2.0-flash', 'gemini-2.0-flash-lite', 'gemini-1.5-pro', 'gemini-1.5-flash'],
        openai: ['gpt-4o', 'gpt-4o-mini', 'gpt-4-turbo', 'gpt-3.5-turbo'],
        'openai-compatible': ['deepseek-chat', 'moonshot-v1-8k', 'qwen-turbo'],
    };

    if (!visible) return null;

    const models = modelSuggestions[config.provider] || modelSuggestions.gemini;

    return (
        <div className="modal-overlay" onClick={onClose}>
            <div className="modal-content ai-config-modal" onClick={e => e.stopPropagation()}>
                {/* Header */}
                <div className="modal-header">
                    <div className="modal-header-left">
                        <div className="modal-icon">
                            <Sparkles size={22} />
                        </div>
                        <div>
                            <h2>AI 配置</h2>
                            <p className="modal-subtitle">配置 AI 内容生成服务</p>
                        </div>
                    </div>
                    <button className="modal-close" onClick={onClose}>
                        <X size={20} />
                    </button>
                </div>

                <div className="modal-body">
                    {/* Provider */}
                    <div className="section">
                        <div className="section-label"><Bot size={14} /> AI 服务商</div>
                        <div className="provider-grid">
                            {providerOptions.map(p => (
                                <button
                                    key={p.value}
                                    className={`provider-card ${config.provider === p.value ? 'active' : ''}`}
                                    onClick={() => {
                                        update('provider', p.value);
                                        if (p.value === 'gemini') update('model', 'gemini-2.0-flash');
                                        else if (p.value === 'openai') update('model', 'gpt-4o-mini');
                                        else update('model', '');
                                    }}
                                >
                                    <div className="provider-name">{p.label}</div>
                                    <div className="provider-desc">{p.desc}</div>
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* API Key */}
                    <div className="section">
                        <div className="section-label"><Key size={14} /> API 密钥</div>
                        <input
                            type="password"
                            value={config.apiKey}
                            onChange={e => update('apiKey', e.target.value)}
                            placeholder={config.provider === 'gemini' ? '输入 Gemini API Key...' : '输入 API Key...'}
                            className="input"
                        />
                        <p className="hint">
                            {config.provider === 'gemini'
                                ? '从 Google AI Studio 获取：https://aistudio.google.com'
                                : '密钥仅保存在本地，不会上传'}
                        </p>
                    </div>

                    {/* Endpoint (for OpenAI compatible) */}
                    {config.provider === 'openai-compatible' && (
                        <div className="section">
                            <div className="section-label"><Server size={14} /> 自定义端点</div>
                            <input
                                type="text"
                                value={config.endpoint}
                                onChange={e => update('endpoint', e.target.value)}
                                placeholder="https://api.deepseek.com/v1"
                                className="input"
                            />
                            <p className="hint">OpenAI 兼容 API 的基础地址</p>
                        </div>
                    )}

                    {/* Model */}
                    <div className="section">
                        <div className="section-label"><Globe size={14} /> 模型</div>
                        <div className="model-grid">
                            {models.map(m => (
                                <button
                                    key={m}
                                    className={`model-chip ${config.model === m ? 'active' : ''}`}
                                    onClick={() => update('model', m)}
                                >
                                    {m}
                                </button>
                            ))}
                        </div>
                        {config.provider === 'openai-compatible' && (
                            <input
                                type="text"
                                value={config.model}
                                onChange={e => update('model', e.target.value)}
                                placeholder="输入自定义模型名称..."
                                className="input"
                                style={{ marginTop: 8 }}
                            />
                        )}
                    </div>

                    {/* Prompt Template */}
                    <div className="section">
                        <div className="section-label"><MessageSquare size={14} /> 提示词模板</div>
                        <textarea
                            rows={5}
                            value={config.promptBase}
                            onChange={e => update('promptBase', e.target.value)}
                            className="textarea mono-text"
                            placeholder={DEFAULT_PROMPT_BASE}
                        />
                        <p className="hint">
                            使用 %s 表示用户输入的主题，%d 表示最大词语数量
                        </p>
                    </div>

                    {/* Max words */}
                    <div className="section">
                        <div className="section-label"><Sliders size={14} /> 生成数量</div>
                        <div className="slider-group">
                            <div className="slider-header">
                                <span className="slider-label">每批生成词语数</span>
                                <span className="slider-value">{config.maxWords}</span>
                            </div>
                            <input
                                type="range"
                                min={5}
                                max={40}
                                value={config.maxWords}
                                onChange={e => update('maxWords', parseInt(e.target.value))}
                                className="range-input"
                            />
                        </div>
                    </div>

                    {/* Tracing toggle */}
                    <div className="section">
                        <div className="toggle-row">
                            <div>
                                <span className="toggle-label">自动添加描红标记</span>
                                <p className="hint" style={{ marginTop: 2 }}>
                                    在复杂字的后面自动加 * 号
                                </p>
                            </div>
                            <label className="toggle">
                                <input
                                    type="checkbox"
                                    checked={config.useTracing}
                                    onChange={e => update('useTracing', e.target.checked)}
                                />
                                <span className="toggle-slider" />
                            </label>
                        </div>
                    </div>

                    {error && (
                        <div className="error-box">
                            <AlertCircle size={14} />
                            <span>{error}</span>
                        </div>
                    )}

                    {saved && (
                        <div className="success-box">
                            <Check size={14} />
                            <span>配置已保存</span>
                        </div>
                    )}
                </div>

                <div className="modal-footer">
                    <button className="btn btn-secondary" onClick={onClose}>
                        取消
                    </button>
                    <button
                        className="btn btn-primary"
                        onClick={handleSave}
                        disabled={saving}
                    >
                        {saving ? '保存中...' : '保存配置'}
                    </button>
                </div>
            </div>
        </div>
    );
}

export default AIConfigPanel;
