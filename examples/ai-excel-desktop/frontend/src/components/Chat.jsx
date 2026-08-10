// ai-elements 风格的对话组件 + 内联图表块 + 全屏弹窗
import { useEffect, useRef, useState, useCallback, Component } from "react";
import Plotly from "plotly.js-dist-min";
import { Streamdown } from "streamdown";
import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";
import "katex/dist/katex.min.css";
import {
  ArrowDown,
  Copy,
  Check,
  RefreshCw,
  User,
  Sparkles,
  CornerDownLeft,
  Maximize2,
  Minimize2,
  BarChart3,
  X,
} from "lucide-react";
import { buildChartTraces, buildChartLayout, FG } from "../lib/plotly";
import { createPortal } from "react-dom";

// ============================================================================
// Conversation - 滚动容器，自动吸底
// ============================================================================
export function Conversation({ children, className = "" }) {
  const ref = useRef(null);
  const [atBottom, setAtBottom] = useState(true);

  const scrollToBottom = useCallback(() => {
    const el = ref.current;
    if (el) el.scrollTo({ top: el.scrollHeight, behavior: "smooth" });
  }, []);

  const onScroll = () => {
    const el = ref.current;
    if (!el) return;
    const dist = el.scrollHeight - el.scrollTop - el.clientHeight;
    setAtBottom(dist < 60);
  };

  useEffect(() => {
    if (atBottom) scrollToBottom();
  });

  return (
    <div className={`relative flex-1 overflow-hidden ${className}`}>
      <div ref={ref} onScroll={onScroll} className="h-full overflow-y-auto" role="log">
        <div className="flex flex-col gap-5 p-4">{children}</div>
      </div>
      {!atBottom && (
        <button
          onClick={scrollToBottom}
          className="absolute bottom-4 left-1/2 -translate-x-1/2 rounded-full border border-border bg-background-elevated p-2 shadow-lg backdrop-blur transition-colors hover:bg-secondary"
          aria-label="滚动到底部"
        >
          <ArrowDown className="h-3.5 w-3.5 text-foreground-muted" />
        </button>
      )}
    </div>
  );
}

// ============================================================================
// Message - 单条消息
// ============================================================================
export function Message({ from, children, className = "" }) {
  const isUser = from === "user";
  return (
    <div
      className={`group flex w-full max-w-[95%] flex-col gap-1.5 ${
        isUser ? "is-user ml-auto items-end" : "is-assistant"
      } ${className}`}
    >
      {/* 角色标签 */}
      <div className="flex items-center gap-1.5 text-[11px] text-foreground-muted">
        {isUser ? (
          <>
            <div className="flex h-5 w-5 items-center justify-center rounded-full bg-secondary">
              <User className="h-2.5 w-2.5" />
            </div>
            <span>你</span>
          </>
        ) : (
          <>
            <div className="flex h-5 w-5 items-center justify-center rounded-full bg-brand-gradient">
              <Sparkles className="h-2.5 w-2.5 text-white" />
            </div>
            <span className="text-primary">AI 助手</span>
          </>
        )}
      </div>
      {children}
    </div>
  );
}

// ============================================================================
// MessageContent - 文本内容
// ============================================================================
export function MessageContent({ children, from }) {
  const isUser = from === "user";
  if (isUser) {
    return (
      <div className="w-fit max-w-full rounded-2xl rounded-tr-sm border border-primary/20 bg-primary/10 px-3.5 py-2.5 text-[13px] text-foreground whitespace-pre-wrap break-words">
        {children}
      </div>
    );
  }
  return (
    <div className="min-w-0 max-w-full text-[13px] text-foreground">
      {typeof children === "string" ? (
        <Streamdown animated isStreaming={false} mermaid={{ config: { theme: "dark" } }} plugins={{ math: mathPlugin }}>
          {children}
        </Streamdown>
      ) : (
        children
      )}
    </div>
  );
}

// ============================================================================
// mathPlugin - 用于 Streamdown 的 KaTeX 数学支持
// ============================================================================
const mathPlugin = {
  name: "katex",
  type: "math",
  remarkPlugin: remarkMath,
  rehypePlugin: rehypeKatex,
};

// ============================================================================
// MessageActions
// ============================================================================
export function MessageActions({ children, className = "" }) {
  return (
    <div className={`flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100 ${className}`}>
      {children}
    </div>
  );
}

export function MessageAction({ icon: Icon, label, onClick }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      onClick={() => {
        onClick?.();
        if (label === "复制") {
          setCopied(true);
          setTimeout(() => setCopied(false), 1500);
        }
      }}
      title={label}
      className="inline-flex h-6 items-center gap-1 rounded-md px-1.5 text-[11px] text-foreground-muted transition-colors hover:bg-secondary hover:text-foreground"
    >
      {label === "复制" && copied ? (
        <Check className="h-3 w-3 text-success" />
      ) : (
        <Icon className="h-3 w-3" />
      )}
      <span>{label}</span>
    </button>
  );
}

// ============================================================================
// ConversationEmptyState
// ============================================================================
export function ConversationEmptyState({
  title = "暂无消息",
  description = "开始对话以查看 AI 分析结果",
  icon,
}) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 p-8 text-center">
      {icon && (
        <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-brand-subtle text-primary">
          {icon}
        </div>
      )}
      <div className="space-y-1">
        <h3 className="text-sm font-medium text-foreground">{title}</h3>
        <p className="mx-auto max-w-[240px] text-xs text-foreground-muted">{description}</p>
      </div>
    </div>
  );
}

// ============================================================================
// PromptInput - 输入框
// ============================================================================
export function PromptInput({
  value: controlledValue,
  onChange: controlledOnChange,
  onSubmit,
  isLoading,
  placeholder = "输入指令...",
  suggestions = [],
}) {
  const [internalValue, setInternalValue] = useState("");
  const value = controlledValue !== undefined ? controlledValue : internalValue;
  const onChange = controlledOnChange || setInternalValue;
  const ref = useRef(null);

  const handleSubmit = () => {
    const text = value.trim();
    if (!text || isLoading) return;
    onSubmit?.(text);
    if (controlledValue === undefined) setInternalValue("");
  };

  const handleKey = (e) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = Math.min(el.scrollHeight, 128) + "px";
  }, [value]);

  return (
    <div className="border-t border-border/60 bg-background-elevated/80 p-3 backdrop-blur">
      {suggestions.length > 0 && (
        <div className="mb-2 flex flex-wrap gap-1.5">
          {suggestions.map((s, i) => (
            <button
              key={i}
              onClick={() => onChange(s)}
              className="rounded-full border border-border bg-secondary/50 px-2.5 py-1 text-[11px] text-foreground-muted transition-colors hover:border-primary/40 hover:bg-primary/10 hover:text-primary"
            >
              {s}
            </button>
          ))}
        </div>
      )}
      <div className="surface-glow flex items-end gap-2 rounded-xl border border-border bg-secondary/40 p-1.5 focus-within:border-primary/50 focus-within:shadow-glow">
        <textarea
          ref={ref}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={handleKey}
          rows={1}
          placeholder={placeholder}
          className="max-h-32 min-h-[28px] flex-1 resize-none bg-transparent px-2 py-1.5 text-[13px] text-foreground outline-none placeholder:text-foreground-muted"
        />
        <button
          onClick={handleSubmit}
          disabled={isLoading || !value.trim()}
          className="btn-primary inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg disabled:opacity-40 disabled:cursor-not-allowed"
        >
          {isLoading ? (
            <RefreshCw className="h-3.5 w-3.5 animate-spin" />
          ) : (
            <CornerDownLeft className="h-3.5 w-3.5" />
          )}
        </button>
      </div>
      <p className="mt-1.5 px-1 text-[10px] text-foreground-muted">
        Enter 发送 · Shift+Enter 换行
      </p>
    </div>
  );
}

// ============================================================================
// 图表 ID 自增计数器（保证每个 ChartBlock 实例唯一）
// ============================================================================
let chartKeyCounter = 0;

// ============================================================================
// ErrorBoundary - 捕获子组件异常，防止整个应用崩溃
// ============================================================================
class ChartErrorBoundary extends Component {
  constructor(props) {
    super(props);
    this.state = { hasError: false, error: null };
  }
  static getDerivedStateFromError(error) {
    return { hasError: true, error };
  }
  componentDidCatch(error, info) {
    console.error("[ChartErrorBoundary]", error, info);
  }
  render() {
    if (this.state.hasError) {
      return (
        <div className="my-2 rounded-xl border border-danger/30 bg-danger/5 p-3">
          <p className="text-xs text-danger">
            图表渲染失败: {this.state.error?.message || "未知错误"}
          </p>
        </div>
      );
    }
    return this.props.children;
  }
}

// ============================================================================
// ChartBlock - 内联图表（Plotly）+ 全屏放大
// ============================================================================
export function ChartBlock({ chartConfig, chartData }) {
  if (!chartConfig) return null;
  console.log("[ChartBlock] render type=%s x=%s y=%j rows=%d",
    chartConfig.type, chartConfig.xAxis, chartConfig.yAxes, chartData?.length);
  return (
    <ChartErrorBoundary>
      <ChartBlockInner chartConfig={chartConfig} chartData={chartData} />
    </ChartErrorBoundary>
  );
}

function ChartBlockInner({ chartConfig, chartData }) {
  const inlineRef = useRef(null);
  const [expanded, setExpanded] = useState(false);
  // 每个实例生成一次唯一 ID
  const [ids] = useState(() => {
    const n = ++chartKeyCounter;
    return {
      inline: `chart-inline-${n}`,
      modal: `chart-modal-${n}`,
    };
  });

  // 渲染内联图表
  useEffect(() => {
    if (!inlineRef.current || !chartConfig) return;
    const traces = buildChartTraces(chartConfig, chartData);
    const layout = buildChartLayout(chartConfig, false, 180);
    Plotly.newPlot(ids.inline, traces, layout, {
      responsive: true,
      displayModeBar: false,
      scrollZoom: false,
    });
    return () => { try { Plotly.purge(ids.inline); } catch {} };
  }, [chartConfig, chartData, ids.inline]);

  // 展开时渲染/更新全屏图表 — 改为使用 FullScreenModal 子组件

  return (
    <>
      {/* 内联预览 */}
      <div className="my-2 overflow-hidden rounded-xl border border-border/60 bg-black/20">
        <div className="flex items-center justify-between bg-secondary/30 px-2.5 py-1.5">
          <div className="flex items-center gap-1.5 text-[11px] text-foreground-muted">
            <BarChart3 className="h-3 w-3 text-primary" />
            <span>{chartConfig.type} 图表</span>
            <span className="text-[10px]">
              · {chartConfig.xAxis} × {chartConfig.yAxes?.join(", ")}
            </span>
          </div>
          <button
            onClick={() => setExpanded(true)}
            className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-[11px] text-foreground-muted transition-colors hover:bg-secondary hover:text-foreground"
            title="放大查看"
          >
            <Maximize2 className="h-3 w-3" />
            放大
          </button>
        </div>
        <div ref={inlineRef} id={ids.inline} className="h-[180px] w-full" />
      </div>

      {/* 全屏模态框 — 通过 Portal 渲染到 body，确保覆盖整个窗口 */}
      {expanded && createPortal(
        <FullScreenModal
          chartConfig={chartConfig}
          chartData={chartData}
          ids={ids}
          onClose={() => setExpanded(false)}
        />,
        document.body
      )}
    </>
  );
}

// ============================================================================
// 全屏模态框子组件（独立 useEffect，保证 DOM 已挂载后再渲染 Plotly）
// ============================================================================
function FullScreenModal({ chartConfig, chartData, ids, onClose }) {
  const modalRef = useRef(null);

  useEffect(() => {
    if (!modalRef.current) return;
    const traces = buildChartTraces(chartConfig, chartData);
    const layout = buildChartLayout(chartConfig, true, undefined);
    Plotly.newPlot(ids.modal, traces, layout, {
      responsive: true,
      displayModeBar: true,
      modeBarButtonsToRemove: ["sendDataToCloud", "lasso2d", "select2d"],
      displaylogo: false,
      scrollZoom: true,
    });
    return () => {
      try { Plotly.purge(ids.modal); } catch {}
    };
  }, [chartConfig, chartData, ids.modal]);

  return (
    <div
      className="fixed inset-0 z-[9999] flex flex-col bg-black/90"
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div className="flex items-center justify-between border-b border-white/10 px-5 py-3">
        <div className="flex items-center gap-2.5 text-sm font-medium text-foreground">
          <BarChart3 className="h-4 w-4 text-primary" />
          {chartConfig.title || `${chartConfig.type} 图表`}
          <span className="text-[12px] font-normal text-foreground-muted">
            {chartConfig.xAxis} × {chartConfig.yAxes?.join(", ")}
            {chartConfig.type === "scatter3d" && " · 拖拽旋转 · 滚轮缩放"}
          </span>
        </div>
        <button
          onClick={onClose}
          className="inline-flex h-9 w-9 items-center justify-center rounded-lg text-foreground-muted transition-colors hover:bg-white/10 hover:text-foreground"
        >
          <X className="h-5 w-5" />
        </button>
      </div>
      <div ref={modalRef} id={ids.modal} className="min-h-0 flex-1" />
    </div>
  );
}
