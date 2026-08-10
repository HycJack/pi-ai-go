// Agent 步骤展示组件：工具调用、工具结果、思考块
import { useState } from "react";
import {
  Wrench,
  ChevronRight,
  Check,
  AlertCircle,
  Loader2,
  Brain,
} from "lucide-react";

// 工具名称中文映射
const TOOL_NAMES = {
  get_data_summary: "数据概览",
  get_column_stats: "列统计",
  get_sample_rows: "预览数据",
  generate_chart: "生成图表",
};

// 工具调用步骤
export function ToolCallStep({ step }) {
  const [expanded, setExpanded] = useState(false);
  const status = step.status || "running";
  const displayName = TOOL_NAMES[step.toolName] || step.toolName;

  return (
    <div className="surface rounded-lg border border-border/60 text-[12px]">
      <button
        onClick={() => setExpanded((v) => !v)}
        className="flex w-full items-center gap-2 px-2.5 py-1.5 text-left transition-colors hover:bg-secondary/50"
      >
        <StatusIcon status={status} />
        <Wrench className="h-3 w-3 text-foreground-muted" />
        <span className="font-medium text-foreground">{displayName}</span>
        <span className="text-[10px] text-foreground-muted">· {step.toolName}</span>
        <ChevronRight
          className={`ml-auto h-3 w-3 text-foreground-muted transition-transform ${
            expanded ? "rotate-90" : ""
          }`}
        />
      </button>
      {expanded && step.content && (
        <div className="border-t border-border/60 px-2.5 py-2">
          <pre className="max-h-48 overflow-auto whitespace-pre-wrap break-words font-mono text-[11px] text-foreground-muted">
            {formatArgs(step.content)}
          </pre>
        </div>
      )}
    </div>
  );
}

// 工具结果步骤
export function ToolResultStep({ result }) {
  const [expanded, setExpanded] = useState(false);
  const preview = result.length > 120 ? result.slice(0, 120) + "..." : result;

  return (
    <div className="surface rounded-lg border border-border/60 text-[12px]">
      <button
        onClick={() => setExpanded((v) => !v)}
        className="flex w-full items-center gap-2 px-2.5 py-1.5 text-left transition-colors hover:bg-secondary/50"
      >
        <Check className="h-3 w-3 text-success" />
        <span className="font-medium text-foreground-muted">结果</span>
        <ChevronRight
          className={`ml-auto h-3 w-3 text-foreground-muted transition-transform ${
            expanded ? "rotate-90" : ""
          }`}
        />
      </button>
      {expanded ? (
        <div className="border-t border-border/60 px-2.5 py-2">
          <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-words font-mono text-[11px] text-foreground-muted">
            {result}
          </pre>
        </div>
      ) : (
        <div className="border-t border-border/60 px-2.5 py-1.5">
          <pre className="whitespace-pre-wrap break-words font-mono text-[11px] text-foreground-muted/70">
            {preview}
          </pre>
        </div>
      )}
    </div>
  );
}

// 思考块
export function ThinkingBlock({ content }) {
  const [expanded, setExpanded] = useState(false);
  const preview = content.length > 80 ? content.slice(0, 80) + "..." : content;

  return (
    <div className="mb-1.5 surface rounded-lg border border-primary/20 bg-primary/5 text-[12px]">
      <button
        onClick={() => setExpanded((v) => !v)}
        className="flex w-full items-center gap-2 px-2.5 py-1.5 text-left transition-colors hover:bg-primary/10"
      >
        <Brain className="h-3 w-3 text-primary" />
        <span className="font-medium text-primary">思考</span>
        <ChevronRight
          className={`ml-auto h-3 w-3 text-primary/60 transition-transform ${
            expanded ? "rotate-90" : ""
          }`}
        />
      </button>
      {expanded && (
        <div className="border-t border-primary/20 px-2.5 py-2">
          <pre className="max-h-48 overflow-auto whitespace-pre-wrap break-words font-mono text-[11px] text-foreground-muted">
            {content}
          </pre>
        </div>
      )}
    </div>
  );
}

// 状态图标
function StatusIcon({ status }) {
  switch (status) {
    case "running":
      return <Loader2 className="h-3 w-3 animate-spin text-primary" />;
    case "done":
      return <Check className="h-3 w-3 text-success" />;
    case "error":
      return <AlertCircle className="h-3 w-3 text-danger" />;
    default:
      return <Loader2 className="h-3 w-3 animate-spin text-primary" />;
  }
}

// 格式化工具参数（尝试 JSON 美化）
function formatArgs(content) {
  if (!content) return "(无参数)";
  try {
    const parsed = JSON.parse(content);
    return JSON.stringify(parsed, null, 2);
  } catch {
    return content;
  }
}
