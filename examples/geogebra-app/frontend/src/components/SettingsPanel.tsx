import { useState, useEffect } from 'react';
import type { Settings, ProviderConfig } from '../types';
import { PROVIDER_TYPES } from '../types';
import { GetModels } from '../../wailsjs/go/main/App';

interface SettingsPanelProps {
  isOpen: boolean;
  onClose: () => void;
  settings: Settings;
  onSave: (settings: Settings) => void;
}

type Tab = 'model' | 'prompt';

const PROMPT_TEMPLATES = [
  {
    id: 'default',
    name: '默认数学助手',
    content: '你是一个专业的GeoGebra数学教学助手。请用GeoGebra命令语言回答数学问题，每行一条命令。\n\n支持的视图模式：\n- 函数视图 (1): 2D函数图像\n- 平面几何 (2): 2D几何图形\n- 立体几何 (5): 3D几何图形\n\n常用命令示例：\n- 绘制函数: f(x) = x^2\n- 创建点: A = (1, 2)\n- 绘制线段: segment(A, B)\n- 绘制圆: circle(A, 3)\n- 绘制垂线: perpendicular(line1, A)\n- 显示标签: showLabels(true)\n\n请确保生成的命令可以在GeoGebra中正确执行。'
  },
  {
    id: 'teacher',
    name: '数学老师模式',
    content: '你是一位耐心的数学老师，使用GeoGebra帮助学生理解几何概念。\n\n任务：\n1. 分析用户的数学问题\n2. 生成GeoGebra命令来构建图形\n3. 详细解释每个命令的作用\n4. 给出解题步骤和思路\n\n输出格式：\n- 先输出GeoGebra命令（每行一条）\n- 然后用中文详细解释解题过程\n\n视图模式说明：\n- 函数视图 (1): 用于函数图像\n- 平面几何 (2): 用于2D几何\n- 立体几何 (5): 用于3D几何\n\n请确保生成的命令可以在GeoGebra中正确执行。'
  },
  {
    id: 'exam',
    name: '考试解题模式',
    content: '你是一位严格的数学阅卷老师，按照考试评分标准解答几何题目。\n\n要求：\n1. 严格按照题目要求绘制图形\n2. 使用标准的几何作图方法\nn3. 标注所有必要的点、线、角\n4. 保留作图痕迹（辅助线用虚线）\n\n输出格式：\n- 只输出GeoGebra命令，每行一条\n- 使用标准的几何命令\n- 显示所有关键点的标签\n\n视图模式：根据题目类型自动选择\n\n请确保生成的命令可以在GeoGebra中正确执行。'
  }
];

export default function SettingsPanel({ isOpen, onClose, settings, onSave }: SettingsPanelProps) {
  const [local, setLocal] = useState<Settings>({ ...settings });
  const [models, setModels] = useState<{ id: string; name: string }[]>([]);
  const [activeTab, setActiveTab] = useState<Tab>('model');
  const [selectedPromptTemplate, setSelectedPromptTemplate] = useState<string>('');

  useEffect(() => {
    if (isOpen) {
      setLocal({ ...settings });
      setActiveTab('model');
      setSelectedPromptTemplate('');
    }
  }, [isOpen, settings]);

  useEffect(() => {
    if (!isOpen) return;
    const cp = local.providers[local.currentProviderIndex];
    if (!cp) return;
    GetModels({ provider: cp.type, baseUrl: cp.baseUrl, apiKey: cp.apiKey })
      .then((list) => {
        if (list && list.length > 0) setModels(list);
      })
      .catch(() => {});
  }, [isOpen, local.currentProviderIndex, local.providers]);

  if (!isOpen) return null;

  const cp = local.providers[local.currentProviderIndex] || local.providers[0];

  const updateProvider = (patch: Partial<ProviderConfig>) => {
    const providers = [...local.providers];
    if (providers.length === 0) return;
    providers[local.currentProviderIndex] = { ...providers[local.currentProviderIndex], ...patch };
    setLocal({ ...local, providers });
  };

  const handleSave = () => {
    onSave(local);
    onClose();
  };

  const handlePromptTemplateSelect = (templateId: string) => {
    const template = PROMPT_TEMPLATES.find(t => t.id === templateId);
    if (template) {
      setLocal({ ...local, systemPrompt: template.content });
      setSelectedPromptTemplate(templateId);
    }
  };

  return (
    <div className="settings-overlay" onClick={onClose}>
      <div className="settings-panel settings-panel-lg" onClick={(e) => e.stopPropagation()}>
        <h2 className="settings-title">设置</h2>
        
        {/* Tabs */}
        <div className="settings-tabs">
          <button
            className={`settings-tab ${activeTab === 'model' ? 'active' : ''}`}
            onClick={() => setActiveTab('model')}
          >
            模型
          </button>
          <button
            className={`settings-tab ${activeTab === 'prompt' ? 'active' : ''}`}
            onClick={() => setActiveTab('prompt')}
          >
            提示词
          </button>
        </div>

        <div className="settings-group">
          {activeTab === 'model' && (
            <>
              <div className="settings-field">
                <label>提供商类型</label>
                <select
                  value={cp?.type || ''}
                  onChange={(e) => {
                    const t = PROVIDER_TYPES.find((p) => p.type === e.target.value);
                    if (t) updateProvider({ name: t.name, type: t.type, baseUrl: t.baseUrl });
                  }}
                >
                  {PROVIDER_TYPES.map((p) => (
                    <option key={p.type} value={p.type}>{p.name}</option>
                  ))}
                </select>
              </div>
              <div className="settings-field">
                <label>API Key</label>
                <input
                  type="password"
                  value={cp?.apiKey || ''}
                  onChange={(e) => updateProvider({ apiKey: e.target.value })}
                  placeholder="输入 API Key"
                />
              </div>
              <div className="settings-field">
                <label>Base URL</label>
                <input
                  value={cp?.baseUrl || ''}
                  onChange={(e) => updateProvider({ baseUrl: e.target.value })}
                  placeholder="https://api.openai.com/v1"
                />
              </div>
              <div className="settings-field">
                <label>模型</label>
                <select
                  value={local.model}
                  onChange={(e) => setLocal({ ...local, model: e.target.value })}
                >
                  {models.map((m) => (
                    <option key={m.id} value={m.id}>{m.name}</option>
                  ))}
                  {models.length === 0 && (
                    <option value={local.model}>{local.model || 'gpt-4o-mini'}</option>
                  )}
                </select>
              </div>
              <div className="settings-field">
                <label>Temperature ({local.temperature})</label>
                <input
                  type="range"
                  min="0"
                  max="2"
                  step="0.1"
                  value={local.temperature}
                  onChange={(e) => setLocal({ ...local, temperature: parseFloat(e.target.value) })}
                />
              </div>
              <div className="settings-field">
                <label>Max Tokens</label>
                <input
                  type="number"
                  value={local.maxTokens}
                  onChange={(e) => setLocal({ ...local, maxTokens: parseInt(e.target.value) || 4096 })}
                />
              </div>
            </>
          )}

          {activeTab === 'prompt' && (
            <>
              <div className="settings-field">
                <label>预设提示词模板</label>
                <div className="settings-prompt-templates">
                  {PROMPT_TEMPLATES.map((template) => (
                    <button
                      key={template.id}
                      onClick={() => handlePromptTemplateSelect(template.id)}
                      className={`settings-prompt-template ${
                        selectedPromptTemplate === template.id || 
                        (selectedPromptTemplate === '' && template.id === 'default')
                          ? 'active' : ''
                      }`}
                    >
                      <div className="settings-prompt-template-name">{template.name}</div>
                      <div className="settings-prompt-template-desc">{template.content.slice(0, 60)}...</div>
                    </button>
                  ))}
                </div>
              </div>
              <div className="settings-field">
                <label>自定义提示词</label>
                <textarea
                  value={local.systemPrompt || ''}
                  onChange={(e) => setLocal({ ...local, systemPrompt: e.target.value })}
                  placeholder="输入系统提示词，定义AI助手的行为"
                  className="settings-textarea"
                />
              </div>
            </>
          )}
        </div>
        <div className="settings-actions">
          <button className="btn-cancel" onClick={onClose}>取消</button>
          <button className="btn-save" onClick={handleSave}>保存</button>
        </div>
      </div>
    </div>
  );
}
