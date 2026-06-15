import { Terminal, ChevronDown, ChevronUp } from 'lucide-react';
import { useState } from 'react';

interface ToolCall {
  id: string;
  name: string;
  arguments: string;
}

interface ToolCallBlockProps {
  toolCalls: ToolCall[];
  defaultExpanded?: boolean;
}

export default function ToolCallBlock({ toolCalls, defaultExpanded = true }: ToolCallBlockProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  if (!toolCalls || toolCalls.length === 0) {
    return null;
  }

  return (
    <div className="bg-cyan-900/30 border border-cyan-700/50 rounded-lg overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between p-3 hover:bg-cyan-900/40 transition-colors"
      >
        <div className="flex items-center gap-2">
          <Terminal className="w-4 h-4 text-cyan-500" />
          <span className="text-sm font-medium text-cyan-400">工具调用 ({toolCalls.length})</span>
        </div>
        {expanded ? (
          <ChevronUp className="w-4 h-4 text-cyan-400" />
        ) : (
          <ChevronDown className="w-4 h-4 text-cyan-400" />
        )}
      </button>
      {expanded && (
        <div className="px-3 pb-3 space-y-2">
          {toolCalls.map((toolCall, index) => (
            <div key={toolCall.id || index}>
              <div className="flex items-center gap-2 mb-1">
                <span className="text-sm font-medium text-cyan-400">{toolCall.name}</span>
                <span className="text-xs text-slate-400">ID: {toolCall.id}</span>
              </div>
              <pre className="text-sm text-slate-300 bg-slate-900/50 rounded px-2 py-1 overflow-x-auto">
                <code>{toolCall.arguments}</code>
              </pre>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
