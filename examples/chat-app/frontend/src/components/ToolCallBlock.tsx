import { useState } from 'react';
import { Terminal, ChevronDownOutlined, ChevronUpOutlined } from '../icons';
import { useT } from '../i18n';

interface ToolCall {
  id: string;
  name: string;
  arguments: string;
}

interface ToolCallBlockProps {
  toolCalls: ToolCall[];
  isLoading?: boolean;
}

export default function ToolCallBlock({ toolCalls, isLoading }: ToolCallBlockProps) {
  const t = useT();
  const [expanded, setExpanded] = useState(true);

  if (!toolCalls || toolCalls.length === 0) return null;

  return (
    <div className="tools-block">
      <button
        onClick={() => setExpanded(!expanded)}
        className="tools-header"
      >
        <span className="tools-title">
          <Terminal size={14} />
          <span>{t('tool.calls')} ({toolCalls.length})</span>
        </span>
        {expanded ? <ChevronUpOutlined size={14} /> : <ChevronDownOutlined size={14} />}
      </button>
      {expanded && (
        <div className="tools-body">
          {toolCalls.map((toolCall, index) => (
            <div key={toolCall.id || index} className="tool-row">
              <div className="tool-row-header">
                <span className="tool-name">{toolCall.name}</span>
                <span className={`tool-badge ${isLoading ? 'running' : 'done'}`}>
                  {isLoading ? t('tool.running') : t('tool.done')}
                </span>
              </div>
              <div className="tool-args">
                <code>{toolCall.arguments}</code>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
