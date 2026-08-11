// AI 文档工作台 - 主应用（接入 pi-ai-go agent 流程）
import { useState, useCallback, useEffect, useRef } from "react";
import {
  FolderOpen,
  Database,
  Download,
  FileSpreadsheet,
  FileText,
  Presentation,
  FileType,
  Trash2,
  BarChart3,
  Table2,
  Sparkles,
  Loader2,
  PanelLeft,
  Rows3,
  Columns3,
  Hash,
  Type,
  Settings,
} from "lucide-react";
import { Api, toast, sampleData } from "./lib/utils";
import { EventsOn, EventsOff } from "../wailsjs/runtime/runtime";
import { DataTable } from "./components/DataTable";
import { AISidebar } from "./components/AISidebar";
import { SettingsPanel } from "./components/SettingsPanel";
import { WordEditor } from "./components/WordEditor";

let idCounter = 0;
const genId = () => `m${++idCounter}`;

export default function App() {
  const [fileInfo, setFileInfo] = useState(null);
  const [headers, setHeaders] = useState([]);
  const [rows, setRows] = useState([]);
  const [types, setTypes] = useState({});
  const [sheets, setSheets] = useState([]);
  const [currentSheet, setCurrentSheet] = useState("");
  const [loading, setLoading] = useState(false);
  const [gridApi, setGridApi] = useState(null);

  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [messages, setMessages] = useState([]);
  const [aiLoading, setAiLoading] = useState(false);

  // 当前正在流式输出的 assistant 消息 id
  const currentStreamRef = useRef(null);
  const sendingRef = useRef(false);

  const dataLoaded = headers.length > 0;
  const docType = getDocType(fileInfo?.name);
  const isSpreadsheet = docType === "excel" || docType === "csv";
  const isWordDoc = docType === "word";

  // ===== 监听 agent 事件流 =====
  useEffect(() => {
    const cleanups = [];
    // 通用事件注册器
    const handler = (eventName, cb) => {
      const fn = (data) => cb(data);
      EventsOn(eventName, fn);
      return () => EventsOff(eventName);
    };

    // 文本增量
    cleanups.push(handler("agent-text-delta", (delta) => {
      const id = currentStreamRef.current;
      if (!id) return;
      setMessages((prev) => prev.map((m) => {
        if (m.id !== id) return m;
        return { ...m, content: (m.content || "") + delta };
      }));
    }));

    // 思考增量
    cleanups.push(handler("agent-thinking-delta", (delta) => {
      const id = currentStreamRef.current;
      if (!id) return;
      setMessages((prev) => prev.map((m) => {
        if (m.id !== id) return m;
        return { ...m, thinking: (m.thinking || "") + delta };
      }));
    }));

    // 工具调用开始
    cleanups.push(handler("agent-tool-call-start", (data) => {
      let tc;
      try { tc = JSON.parse(data); } catch { return; }
      const id = currentStreamRef.current;
      if (!id) return;
      setMessages((prev) => prev.map((m) => {
        if (m.id !== id) return m;
        const steps = [...(m.steps || [])];
        steps.push({ type: "tool_call", toolName: tc.name, toolCallId: tc.id, content: "", status: "running" });
        return { ...m, steps };
      }));
    }));

    // 工具调用参数增量
    cleanups.push(handler("agent-tool-call-delta", (delta) => {
      const id = currentStreamRef.current;
      if (!id) return;
      setMessages((prev) => prev.map((m) => {
        if (m.id !== id) return m;
        const steps = [...(m.steps || [])];
        for (let i = steps.length - 1; i >= 0; i--) {
          if (steps[i].type === "tool_call" && steps[i].status === "running") {
            steps[i] = { ...steps[i], content: (steps[i].content || "") + delta };
            break;
          }
        }
        return { ...m, steps };
      }));
    }));

    // 工具调用结束
    cleanups.push(handler("agent-tool-call-end", (args) => {
      let argsStr = args;
      try { argsStr = JSON.parse(args); } catch { /* keep raw */ }
      const id = currentStreamRef.current;
      if (!id) return;
      setMessages((prev) => prev.map((m) => {
        if (m.id !== id) return m;
        const steps = [...(m.steps || [])];
        for (let i = steps.length - 1; i >= 0; i--) {
          if (steps[i].type === "tool_call" && steps[i].status === "running") {
            steps[i] = { ...steps[i], content: typeof argsStr === "string" ? argsStr : JSON.stringify(argsStr, null, 2) };
            break;
          }
        }
        return { ...m, steps };
      }));
    }));

    // 工具执行结果
    cleanups.push(handler("agent-tool-result", (result) => {
      const id = currentStreamRef.current;
      if (!id) return;
      setMessages((prev) => prev.map((m) => {
        if (m.id !== id) return m;
        const steps = [...(m.steps || [])];
        for (let i = steps.length - 1; i >= 0; i--) {
          if (steps[i].type === "tool_call" && steps[i].status === "running") {
            steps[i] = { ...steps[i], status: "done" };
            break;
          }
        }
        steps.push({ type: "tool_result", content: result });
        return { ...m, steps };
      }));
    }));

    // 工具执行结束
    cleanups.push(handler("agent-tool-exec-end", (data) => {
      let parsed;
      try { parsed = JSON.parse(data); } catch { parsed = { success: true }; }
      const id = currentStreamRef.current;
      if (!id) return;
      setMessages((prev) => prev.map((m) => {
        if (m.id !== id) return m;
        const steps = [...(m.steps || [])];
        for (let i = steps.length - 1; i >= 0; i--) {
          if (steps[i].type === "tool_call" && steps[i].status === "running") {
            steps[i] = { ...steps[i], status: parsed.success === false ? "error" : "done" };
            break;
          }
        }
        return { ...m, steps };
      }));
    }));

    // 图表配置事件（由 generate_chart 工具触发）
    // 每次触发都追加一条 chartBlock，支持一条消息中生成多张图
    cleanups.push(handler("chart-config", (cfgJson) => {
      try {
        const cfg = typeof cfgJson === "string" ? JSON.parse(cfgJson) : cfgJson;
        let cd = rows;
        if (!cd || cd.length === 0) {
          console.warn("[chart-config] rows is empty, skipping");
          toast("图表数据为空，跳过", "error");
          return;
        }
        // 过滤掉 undefined/null 的行
        cd = cd.filter(Boolean);
        if (cfg.sample > 0 && cd.length > cfg.sample) cd = sampleData(cd, cfg.sample);
        if (cd.length === 0) {
          toast("采样后数据为空", "error");
          return;
        }
        // 把图表配置追加到当前流式消息的 chartBlocks 数组
        const id = currentStreamRef.current;
        if (id) {
          setMessages((prev) => prev.map((m) => {
            if (m.id !== id) return m;
            const blocks = [...(m.chartBlocks || []), { config: cfg, data: cd }];
            return { ...m, chartBlocks: blocks };
          }));
          console.log("[chart-config] appended block type=%s rows=%d", cfg.type, cd.length);
        } else {
          console.warn("[chart-config] no active stream id");
        }
        toast(`已生成 ${cfg.type} 图表`, "success");
      } catch (e) {
        console.error("[chart-config] error:", e);
        toast("图表配置解析失败: " + (e?.message || e), "error");
      }
    }));

    // agent 完成
    cleanups.push(handler("agent-done", () => {
      const id = currentStreamRef.current;
      if (id) {
        setMessages((prev) => prev.map((m) => {
          if (m.id !== id) return m;
          if (!m.content) {
            return { ...m, content: "(模型未返回任何内容，请检查设置或重试)" };
          }
          return { ...m, streaming: false };
        }));
      }
      setAiLoading(false);
      currentStreamRef.current = null;
      sendingRef.current = null;
    }));

    // agent 错误
    cleanups.push(handler("agent-error", (error) => {
      const id = currentStreamRef.current;
      if (id) {
        setMessages((prev) => prev.map((m) => {
          if (m.id !== id) return m;
          if (!m.content) return { ...m, content: `Error: ${error}` };
          return { ...m, content: m.content + `\n\n[错误] ${error}` };
        }));
      }
      setAiLoading(false);
      currentStreamRef.current = null;
      sendingRef.current = null;
      toast("Agent 错误: " + error, "error");
    }));

    return () => cleanups.forEach((fn) => fn());
  }, [rows]);

  // ===== 文件操作 =====
  const openFile = useCallback(async () => {
    try {
      const fi = await Api.OpenFileDialog();
      if (!fi) return;
      setFileInfo(fi);
      setSheets(fi.sheets || []);
      setCurrentSheet(fi.selected || "");
      await loadSheet(fi.selected);
    } catch (e) {
      toast("打开失败: " + e, "error");
    }
  }, []);

  const loadSheet = useCallback(async (name) => {
    if (!name) return;
    setLoading(true);
    try {
      toast("正在解析数据...", "info");
      const data = await Api.LoadSheet(name);
      if (!data?.headers) {
        toast("工作表为空", "error");
        return;
      }
      setHeaders(data.headers);
      setTypes(data.types || {});
      const objRows = data.rows.map((r) => {
        const o = {};
        data.headers.forEach((h, i) => (o[h] = r[i]));
        return o;
      });
      setRows(objRows);
      toast(`已加载 ${data.total.toLocaleString()} 行`, "success");
    } catch (e) {
      toast("加载失败: " + e, "error");
    } finally {
      setLoading(false);
    }
  }, []);

  const generateSample = useCallback(() => {
    const count = 50000;
    const months = ["1月","2月","3月","4月","5月","6月","7月","8月","9月","10月","11月","12月"];
    const newHeaders = ["序号","月份","销售额","利润","客户数","满意度","地区","负责人"];
    const newRows = [];
    for (let i = 0; i < count; i++) {
      const base = 10000 + (i % 12) * 2000;
      const o = {};
      o["序号"] = i + 1;
      o["月份"] = months[i % 12];
      o["销售额"] = Math.floor(base + Math.random() * 5000);
      o["利润"] = Math.floor(base * 0.3 + Math.random() * 2000);
      o["客户数"] = Math.floor(100 + (i % 12) * 20 + Math.random() * 50);
      o["满意度"] = parseFloat((4 + Math.random()).toFixed(2));
      o["地区"] = ["华北","华东","华南","西部"][Math.floor(i / 4) % 4];
      o["负责人"] = ["张经理","李经理","王经理","刘经理"][i % 4];
      newRows.push(o);
    }
    const newTypes = {};
    newHeaders.forEach((h) => {
      const vals = newRows.slice(0, 1000).map((r) => r[h]).filter((v) => v !== null && v !== "");
      const num = vals.filter((v) => typeof v === "number" || !isNaN(parseFloat(v))).length;
      newTypes[h] = num > vals.length * 0.7 ? "numeric" : "categorical";
    });
    setFileInfo({ name: "示例数据.csv" });
    setHeaders(newHeaders);
    setRows(newRows);
    setTypes(newTypes);
    setSheets([]);
    setCurrentSheet("");
    toast(`已生成 ${count.toLocaleString()} 行示例数据`, "success");
  }, []);

  // ===== AI 对话（agent 流程）=====
  const sendPrompt = useCallback(async (text) => {
    if (!dataLoaded) { toast("请先加载数据", "error"); return; }
    if (sendingRef.current) { toast("正在处理中...", "info"); return; }

    // 构建历史消息（只取最近 10 轮，避免上下文过长）
    const historyMessages = messages
      .filter((m) => m.role === "user" || (m.role === "assistant" && m.content))
      .slice(-20)
      .map((m) => ({ role: m.role, content: m.content }));

    // 先把用户消息和占位的 assistant 消息加入列表
    const userMsgId = genId();
    const assistantMsgId = genId();
    setMessages((prev) => [
      ...prev,
      { id: userMsgId, role: "user", content: text },
      { id: assistantMsgId, role: "assistant", content: "", streaming: true, steps: [] },
    ]);
    setAiLoading(true);
    sendingRef.current = text;
    currentStreamRef.current = assistantMsgId;

    try {
      const settings = await loadSettings();
      const req = {
        message: text,
        messages: historyMessages,
        conversationId: "default",
        provider: settings.providerType,
        apiKey: settings.apiKey,
        baseUrl: settings.baseUrl,
        model: settings.model,
        maxTokens: settings.maxTokens,
        temperature: settings.temperature,
      };
      await Api.AgentMessage(JSON.stringify(req));
    } catch (e) {
      toast("发送失败: " + e, "error");
      setAiLoading(false);
      currentStreamRef.current = null;
      sendingRef.current = null;
      setMessages((prev) => prev.map((m) => {
        if (m.id !== assistantMsgId) return m;
        return { ...m, content: `发送失败: ${e}`, streaming: false };
      }));
    }
  }, [dataLoaded, messages]);

  const cancelStream = useCallback(() => {
    Api.CancelStream();
    setAiLoading(false);
    currentStreamRef.current = null;
    sendingRef.current = null;
    toast("已取消", "info");
  }, []);

  const copyLastReply = useCallback(() => {
    const last = [...messages].reverse().find((m) => m.role === "assistant" && m.content);
    if (last) { navigator.clipboard.writeText(last.content); toast("已复制", "success"); }
  }, [messages]);

  const regenerate = useCallback(() => {
    const lastUser = [...messages].reverse().find((m) => m.role === "user");
    if (lastUser) {
      setMessages((prev) => prev.filter((m) => m.id !== lastUser.id));
      sendPrompt(lastUser.content);
    }
  }, [messages, sendPrompt]);

  const exportExcel = useCallback(async () => {
    if (!gridApi) return;
    const arrRows = rows.map((r) => headers.map((h) => r[h]));
    try {
      const path = await Api.ExportExcel(headers, arrRows);
      if (path) toast("已导出: " + path, "success");
    } catch (e) { toast("导出失败: " + e, "error"); }
  }, [gridApi, headers, rows]);

  const exportCSV = useCallback(async () => {
    if (!gridApi) return;
    const arrRows = rows.map((r) => headers.map((h) => r[h]));
    try {
      const path = await Api.ExportCSV(headers, arrRows);
      if (path) toast("已导出: " + path, "success");
    } catch (e) { toast("导出失败: " + e, "error"); }
  }, [gridApi, headers, rows]);

  const clearAll = useCallback(() => {
    setFileInfo(null); setHeaders([]); setRows([]); setTypes({});
    setSheets([]); setCurrentSheet("");
    setMessages([]);
  }, []);

  const numericCount = Object.values(types).filter((t) => t === "numeric").length;
  const textCount = Object.values(types).filter((t) => t === "categorical").length;

  return (
    <div className="flex h-screen flex-col">
      {/* ====== Header ====== */}
      <header className="relative flex h-14 shrink-0 items-center justify-between border-b border-border/60 bg-background-elevated/80 px-5 backdrop-blur-xl">
        <div className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/40 to-transparent" />
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-brand-gradient shadow-glow">
            <BarChart3 className="h-5 w-5 text-white" />
          </div>
          <div>
            <h1 className="text-[15px] font-semibold tracking-tight gradient-text">AI 文档工作台</h1>
            <p className="text-[11px] text-foreground-muted">Wails · Go · pi-ai-go Agent · React</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {fileInfo && (
            <span className="hidden items-center gap-1.5 rounded-lg border border-border bg-secondary/50 px-2.5 py-1 text-xs text-foreground-muted md:inline-flex">
              <DocIcon docType={docType} className="h-3 w-3 text-accent" />
              {fileInfo.name}
            </span>
          )}
          <button
            onClick={() => setSettingsOpen(true)}
            className="btn-ghost inline-flex h-9 w-9 items-center justify-center rounded-lg"
            title="设置"
          >
            <Settings className="h-4 w-4" />
          </button>
          <button
            onClick={openFile}
            className="btn-primary inline-flex items-center gap-2 rounded-lg px-3.5 py-2 text-[13px] font-medium"
          >
            <FolderOpen className="h-4 w-4" />
            打开文件
          </button>
          <button
            onClick={() => setSidebarOpen((v) => !v)}
            className="btn-ghost inline-flex h-9 w-9 items-center justify-center rounded-lg"
            title={sidebarOpen ? "收起 AI 侧边栏" : "展开 AI 侧边栏"}
          >
            <Sparkles className={`h-4 w-4 ${sidebarOpen ? "text-primary" : ""}`} />
          </button>
        </div>
      </header>

      {/* ====== 主体 ====== */}
      <div className="flex min-h-0 flex-1">
        <main className="flex min-w-0 flex-1 flex-col overflow-hidden">
          {!dataLoaded ? (
            <UploadView onOpen={openFile} onSample={generateSample} loading={loading} />
          ) : isWordDoc ? (
            <WordEditor
              file={null}
              docxBase64={rows[0]?.content}
              fileName={fileInfo?.name}
              onClose={() => clearAll()}
            />
          ) : isSpreadsheet ? (
            <DataView
              headers={headers} rows={rows} types={types}
              sheets={sheets} currentSheet={currentSheet}
              onSheetChange={loadSheet} onGridReady={setGridApi}
              onExportExcel={exportExcel} onExportCSV={exportCSV} onClear={clearAll}
              numericCount={numericCount} textCount={textCount}
            />
          ) : (
            <DocPlaceholder docType={docType} fileName={fileInfo?.name} onClose={clearAll} />
          )}
        </main>

        <AISidebar
          open={sidebarOpen} onToggle={() => setSidebarOpen((v) => !v)}
          messages={messages} onSend={sendPrompt} onClear={() => setMessages([])}
          onCopyLast={copyLastReply} onRegenerate={regenerate}
          onCancel={cancelStream}
          isLoading={aiLoading} dataLoaded={dataLoaded}
        />
      </div>

      {settingsOpen && (
        <SettingsPanel onClose={() => setSettingsOpen(false)} />
      )}
    </div>
  );
}

// ============================================================================
// 加载设置（缓存）
// ============================================================================
let cachedSettings = null;
async function loadSettings() {
  if (cachedSettings) return cachedSettings;
  try {
    const str = await Api.GetSettings();
    const s = JSON.parse(str);
    const cp = s.providers?.[s.currentProviderIndex] || s.providers?.[0] || {};
    cachedSettings = {
      providerType: cp.type || "openai",
      apiKey: cp.apiKey || "",
      baseUrl: cp.baseUrl || "",
      model: s.model || "gpt-4o-mini",
      maxTokens: s.maxTokens || 4096,
      temperature: s.temperature || 1.0,
    };
  } catch {
    cachedSettings = {
      providerType: "openai",
      apiKey: "",
      baseUrl: "",
      model: "gpt-4o-mini",
      maxTokens: 4096,
      temperature: 1.0,
    };
  }
  return cachedSettings;
}

// 暴露给 SettingsPanel 清空缓存
if (typeof window !== "undefined") {
  window.__invalidateSettingsCache = () => { cachedSettings = null; };
}

// ============================================================================
// 上传视图
// ============================================================================
function UploadView({ onOpen, onSample, loading }) {
  return (
    <div className="flex flex-1 items-center justify-center p-6">
      <div className="surface-glow surface-elevated fade-in w-full max-w-lg overflow-hidden p-10">
        <div className="mb-8 text-center">
          <div className="mx-auto mb-5 flex h-14 w-14 items-center justify-center rounded-2xl bg-brand-gradient shadow-glow">
            <Database className="h-7 w-7 text-white" />
          </div>
          <h2 className="mb-1.5 text-2xl font-semibold tracking-tight">AI 文档工作台</h2>
          <p className="text-sm text-foreground-muted">
            打开文档，AI 助手帮你分析、编辑、总结、生成图表
          </p>
        </div>

        <button
          onClick={onOpen}
          disabled={loading}
          className="group surface-glow relative flex w-full flex-col items-center rounded-xl border border-dashed border-border bg-brand-subtle p-10 transition-all hover:border-primary/50 hover:bg-primary/5 disabled:opacity-50"
        >
          <div className="pointer-events-none absolute inset-0 rounded-xl bg-gradient-to-b from-white/[0.03] to-transparent" />
          {loading ? (
            <Loader2 className="relative mb-4 h-12 w-12 animate-spin text-primary" />
          ) : (
            <div className="relative mb-4 flex h-12 w-12 items-center justify-center rounded-full border border-border bg-secondary">
              <FolderOpen className="h-5 w-5 text-primary" />
            </div>
          )}
          <p className="relative text-base font-medium text-foreground">
            点击选择文件
          </p>
          <p className="relative mt-1 text-xs text-foreground-muted">
            支持 Excel / CSV / Word / PPT / PDF / 文本
          </p>
        </button>

        <div className="mt-5 flex items-center justify-center gap-3">
          <button
            onClick={onSample}
            className="btn-ghost inline-flex items-center gap-2 rounded-lg px-3.5 py-2 text-[13px] font-medium"
          >
            <Database className="h-3.5 w-3.5" />
            生成示例数据（5万行）
          </button>
        </div>
      </div>
    </div>
  );
}

// ============================================================================
// 数据视图
// ============================================================================
function DataView({
  headers, rows, types, sheets, currentSheet, onSheetChange, onGridReady,
  onExportExcel, onExportCSV, onClear,
  numericCount, textCount,
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3 p-4">
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <StatCard icon={Rows3} label="总行数" value={rows.length.toLocaleString()} variant="brand" />
        <StatCard icon={Columns3} label="总列数" value={headers.length} />
        <StatCard icon={Hash} label="数值列" value={numericCount} accent="text-accent" />
        <StatCard icon={Type} label="文本列" value={textCount} accent="text-primary" />
      </div>

      {sheets.length > 0 && (
        <div className="surface flex items-center gap-2 px-3 py-2">
          <span className="text-xs text-foreground-muted">工作表</span>
          <select
            value={currentSheet}
            onChange={(e) => onSheetChange(e.target.value)}
            className="rounded-md border border-border bg-secondary px-2.5 py-1 text-xs text-foreground outline-none focus:border-primary"
          >
            {sheets.map((s) => <option key={s} value={s}>{s}</option>)}
          </select>
        </div>
      )}

      <section className="surface-glow surface flex min-h-0 flex-1 flex-col overflow-hidden">
        <div className="flex items-center justify-between border-b border-border/60 px-4 py-2.5">
          <h3 className="flex items-center gap-2 text-[13px] font-medium text-foreground">
            <Table2 className="h-4 w-4 text-primary" />
            数据预览
            <span className="ml-1 text-[11px] font-normal text-foreground-muted">
              虚拟滚动 · 排序 · 筛选
            </span>
          </h3>
          <div className="flex gap-1">
            <ToolButton onClick={onExportExcel} icon={FileSpreadsheet} label="Excel" />
            <ToolButton onClick={onExportCSV} icon={Download} label="CSV" />
            <ToolButton onClick={onClear} icon={Trash2} label="清除" danger />
          </div>
        </div>
        <div className="min-h-0 flex-1">
          <DataTable headers={headers} rows={rows} types={types} onReady={onGridReady} />
        </div>
      </section>
    </div>
  );
}

// ============================================================================
// 子组件
// ============================================================================
function StatCard({ icon: Icon, label, value, variant, accent }) {
  return (
    <div className={`surface-glow surface relative overflow-hidden p-3.5 ${variant === "brand" ? "bg-brand-subtle" : ""}`}>
      <div className="flex items-center justify-between">
        <p className="text-[11px] uppercase tracking-wider text-foreground-muted">{label}</p>
        <Icon className={`h-3.5 w-3.5 ${accent || "text-foreground-muted"}`} />
      </div>
      <p className={`mt-1.5 text-xl font-semibold tracking-tight ${accent || "text-foreground"}`}>
        {value}
      </p>
    </div>
  );
}

function ToolButton({ onClick, icon: Icon, label, danger }) {
  return (
    <button
      onClick={onClick}
      className={`inline-flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-xs font-medium transition-colors ${
        danger
          ? "text-danger hover:bg-danger/10"
          : "text-foreground-muted hover:bg-secondary hover:text-foreground"
      }`}
    >
      <Icon className="h-3.5 w-3.5" />
      {label}
    </button>
  );
}

function Tag({ color, children }) {
  const colors = {
    primary: "bg-primary/15 text-primary border-primary/20",
    accent: "bg-accent/15 text-accent border-accent/20",
    purple: "bg-purple-500/15 text-purple-300 border-purple-500/20",
  };
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[11px] font-medium ${colors[color]}`}>
      {children}
    </span>
  );
}

// ============================================================================
// 文档类型辅助
// ============================================================================
function getDocType(fileName) {
  if (!fileName) return "unknown";
  const ext = fileName.toLowerCase().split(".").pop();
  switch (ext) {
    case "xlsx": case "xls": return "excel";
    case "csv": return "csv";
    case "docx": return "word";
    case "pptx": return "ppt";
    case "pdf": return "pdf";
    case "txt": case "md": return "text";
    default: return "unknown";
  }
}

function DocIcon({ docType, className }) {
  const Icon = {
    excel: FileSpreadsheet,
    csv: FileSpreadsheet,
    word: FileText,
    ppt: Presentation,
    pdf: FileType,
    text: FileText,
    unknown: FileText,
  }[docType] || FileText;
  return <Icon className={className} />;
}

// 未实现的文档类型占位符
function DocPlaceholder({ docType, fileName, onClose }) {
  const labels = {
    ppt: "PowerPoint 演示文稿",
    pdf: "PDF 文档",
    text: "文本文件",
    unknown: "文档",
  };
  const label = labels[docType] || "文档";
  return (
    <div className="flex flex-1 items-center justify-center p-6">
      <div className="surface-glow surface-elevated fade-in w-full max-w-md p-10 text-center">
        <div className="mx-auto mb-5 flex h-16 w-16 items-center justify-center rounded-2xl bg-brand-subtle">
          <DocIcon docType={docType} className="h-8 w-8 text-primary" />
        </div>
        <h2 className="mb-1.5 text-xl font-semibold tracking-tight">{label}</h2>
        <p className="mb-1 text-sm text-foreground-muted break-all">{fileName}</p>
        <p className="mt-4 text-xs text-foreground-muted">
          该文档类型的查看器正在开发中。当前可通过右侧 AI 助手对话处理文档内容。
        </p>
        <button
          onClick={onClose}
          className="btn-ghost mt-6 inline-flex items-center gap-2 rounded-lg px-3.5 py-2 text-[13px] font-medium"
        >
          <Trash2 className="h-3.5 w-3.5" />
          关闭文档
        </button>
      </div>
    </div>
  );
}
