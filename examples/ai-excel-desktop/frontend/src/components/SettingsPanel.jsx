// 设置面板：配置 LLM provider / API Key / 模型
import { useState, useEffect, useCallback } from "react";
import { X, Plus, Trash2, RefreshCw, Check } from "lucide-react";
import { Api, toast } from "../lib/utils";

const PROVIDER_TYPES = [
  { type: "openai", name: "OpenAI", defaultUrl: "https://api.openai.com/v1" },
  { type: "anthropic", name: "Anthropic", defaultUrl: "https://api.anthropic.com/v1" },
  { type: "google", name: "Google", defaultUrl: "" },
  { type: "deepseek", name: "DeepSeek", defaultUrl: "https://api.deepseek.com/v1" },
  { type: "mistral", name: "Mistral", defaultUrl: "https://api.mistral.ai/v1" },
];

export function SettingsPanel({ onClose }) {
  const [settings, setSettings] = useState(null);
  const [models, setModels] = useState([]);
  const [loadingModels, setLoadingModels] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    Api.GetSettings().then((str) => {
      try {
        const s = JSON.parse(str);
        if (!s.providers || s.providers.length === 0) {
          s.providers = [{ name: "OpenAI", type: "openai", apiKey: "", baseUrl: "https://api.openai.com/v1" }];
        }
        setSettings(s);
        fetchModels(s);
      } catch (e) {
        toast("加载设置失败: " + e, "error");
      }
    }).catch((e) => toast("加载设置失败: " + e, "error"));
  }, []);

  const fetchModels = useCallback(async (s) => {
    const cp = s.providers?.[s.currentProviderIndex] || s.providers?.[0];
    if (!cp) return;
    setLoadingModels(true);
    try {
      const list = await Api.GetModels({
        provider: cp.type,
        baseUrl: cp.baseUrl,
        apiKey: cp.apiKey,
      });
      setModels(list || []);
    } catch (e) {
      toast("获取模型列表失败: " + e, "error");
      setModels([]);
    } finally {
      setLoadingModels(false);
    }
  }, []);

  if (!settings) {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
        <div className="surface-glow surface w-full max-w-md p-8 text-center text-sm text-foreground-muted">
          加载中...
        </div>
      </div>
    );
  }

  const cp = settings.providers[settings.currentProviderIndex] || settings.providers[0];

  const updateProvider = (field, value) => {
    setSettings((prev) => {
      const next = { ...prev };
      const idx = next.currentProviderIndex || 0;
      next.providers = [...next.providers];
      next.providers[idx] = { ...next.providers[idx], [field]: value };
      return next;
    });
  };

  const addProvider = () => {
    setSettings((prev) => {
      const next = { ...prev };
      next.providers = [...next.providers, { name: "新提供商", type: "openai", apiKey: "", baseUrl: "https://api.openai.com/v1" }];
      next.currentProviderIndex = next.providers.length - 1;
      return next;
    });
  };

  const removeProvider = (idx) => {
    setSettings((prev) => {
      if (prev.providers.length <= 1) return prev;
      const next = { ...prev };
      next.providers = prev.providers.filter((_, i) => i !== idx);
      if (next.currentProviderIndex >= next.providers.length) {
        next.currentProviderIndex = next.providers.length - 1;
      } else if (next.currentProviderIndex > idx) {
        next.currentProviderIndex -= 1;
      }
      return next;
    });
  };

  const selectProvider = (idx) => {
    setSettings((prev) => ({ ...prev, currentProviderIndex: idx }));
  };

  const onProviderTypeChange = (type) => {
    const cfg = PROVIDER_TYPES.find((p) => p.type === type);
    setSettings((prev) => {
      const next = { ...prev };
      const idx = next.currentProviderIndex || 0;
      next.providers = [...next.providers];
      next.providers[idx] = {
        ...next.providers[idx],
        type,
        name: cfg?.name || type,
        baseUrl: cfg?.defaultUrl || "",
      };
      return next;
    });
  };

  const save = async () => {
    setSaving(true);
    try {
      await Api.SaveSettings(JSON.stringify(settings));
      if (typeof window !== "undefined" && window.__invalidateSettingsCache) {
        window.__invalidateSettingsCache();
      }
      toast("设置已保存", "success");
      fetchModels(settings);
    } catch (e) {
      toast("保存失败: " + e, "error");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
      <div className="surface-glow surface flex max-h-[90vh] w-full max-w-lg flex-col overflow-hidden">
        {/* 头部 */}
        <div className="flex items-center justify-between border-b border-border/60 px-5 py-3.5">
          <h2 className="text-[15px] font-semibold tracking-tight">设置</h2>
          <button
            onClick={onClose}
            className="inline-flex h-8 w-8 items-center justify-center rounded-md text-foreground-muted transition-colors hover:bg-secondary hover:text-foreground"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* 内容 */}
        <div className="min-h-0 flex-1 overflow-y-auto p-5">
          {/* Provider 列表 */}
          <div className="mb-5">
            <div className="mb-2 flex items-center justify-between">
              <label className="text-[13px] font-medium text-foreground">LLM 提供商</label>
              <button
                onClick={addProvider}
                className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[11px] text-primary transition-colors hover:bg-primary/10"
              >
                <Plus className="h-3 w-3" />
                添加
              </button>
            </div>
            <div className="flex flex-wrap gap-1.5">
              {settings.providers.map((p, i) => (
                <button
                  key={i}
                  onClick={() => selectProvider(i)}
                  className={`inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1 text-[12px] transition-colors ${
                    i === settings.currentProviderIndex
                      ? "border-primary bg-primary/10 text-primary"
                      : "border-border bg-secondary/50 text-foreground-muted hover:text-foreground"
                  }`}
                >
                  {p.name || p.type}
                  {settings.providers.length > 1 && (
                    <Trash2
                      className="h-3 w-3 hover:text-danger"
                      onClick={(e) => { e.stopPropagation(); removeProvider(i); }}
                    />
                  )}
                </button>
              ))}
            </div>
          </div>

          {/* 当前 Provider 配置 */}
          <div className="space-y-3">
            <Field label="提供商类型">
              <select
                value={cp.type}
                onChange={(e) => onProviderTypeChange(e.target.value)}
                className="input"
              >
                {PROVIDER_TYPES.map((p) => (
                  <option key={p.type} value={p.type}>{p.name}</option>
                ))}
              </select>
            </Field>

            <Field label="名称">
              <input
                type="text"
                value={cp.name || ""}
                onChange={(e) => updateProvider("name", e.target.value)}
                className="input"
                placeholder="显示名称"
              />
            </Field>

            <Field label="API Key">
              <input
                type="password"
                value={cp.apiKey || ""}
                onChange={(e) => updateProvider("apiKey", e.target.value)}
                className="input"
                placeholder="sk-..."
              />
            </Field>

            <Field label="Base URL">
              <input
                type="text"
                value={cp.baseUrl || ""}
                onChange={(e) => updateProvider("baseUrl", e.target.value)}
                className="input"
                placeholder="https://api.openai.com/v1"
              />
            </Field>

            <Field label="模型">
              <div className="flex gap-2">
                <select
                  value={settings.model || ""}
                  onChange={(e) => setSettings((prev) => ({ ...prev, model: e.target.value }))}
                  className="input flex-1"
                >
                  {loadingModels ? (
                    <option>加载中...</option>
                  ) : models.length > 0 ? (
                    models.map((m) => (
                      <option key={m.id} value={m.id}>{m.name || m.id}</option>
                    ))
                  ) : (
                    <option value={settings.model || "gpt-4o-mini"}>{settings.model || "gpt-4o-mini"}</option>
                  )}
                </select>
                <button
                  onClick={() => fetchModels(settings)}
                  disabled={loadingModels}
                  className="btn-ghost inline-flex h-9 w-9 items-center justify-center rounded-md disabled:opacity-50"
                  title="刷新模型列表"
                >
                  <RefreshCw className={`h-3.5 w-3.5 ${loadingModels ? "animate-spin" : ""}`} />
                </button>
              </div>
            </Field>

            <div className="grid grid-cols-2 gap-3">
              <Field label="Max Tokens">
                <input
                  type="number"
                  value={settings.maxTokens || 4096}
                  onChange={(e) => setSettings((prev) => ({ ...prev, maxTokens: parseInt(e.target.value) || 4096 }))}
                  className="input"
                  min={256}
                />
              </Field>
              <Field label="Temperature">
                <input
                  type="number"
                  step="0.1"
                  value={settings.temperature ?? 1.0}
                  onChange={(e) => setSettings((prev) => ({ ...prev, temperature: parseFloat(e.target.value) || 1.0 }))}
                  className="input"
                  min={0}
                  max={2}
                />
              </Field>
            </div>
          </div>
        </div>

        {/* 底部 */}
        <div className="flex items-center justify-end gap-2 border-t border-border/60 px-5 py-3">
          <button
            onClick={onClose}
            className="btn-ghost inline-flex items-center gap-1.5 rounded-lg px-3.5 py-2 text-[13px] font-medium"
          >
            取消
          </button>
          <button
            onClick={save}
            disabled={saving}
            className="btn-primary inline-flex items-center gap-1.5 rounded-lg px-3.5 py-2 text-[13px] font-medium disabled:opacity-50"
          >
            {saving ? <RefreshCw className="h-3.5 w-3.5 animate-spin" /> : <Check className="h-3.5 w-3.5" />}
            保存
          </button>
        </div>
      </div>
    </div>
  );
}

function Field({ label, children }) {
  return (
    <div>
      <label className="mb-1 block text-[12px] font-medium text-foreground-muted">{label}</label>
      {children}
    </div>
  );
}
