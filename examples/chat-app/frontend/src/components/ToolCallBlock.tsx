import { useState } from 'react';
import { Terminal, ChevronDownOutlined, ChevronUpOutlined } from '../icons';

interface ToolCall {
  id: string;
  name: string;
  arguments: string;
}

interface ToolCallBlockProps {
  toolCalls: ToolCall[];
}

export default function ToolCallBlock({ toolCalls }: ToolCallBlockProps) {
  const [expanded, setExpanded] = useState(false);

  if (!toolCalls || toolCalls.length === 0) return null;

  return (
    <div className="tools-block">
      <button
        onClick={() => setExpanded(!expanded)}
        className="tools-header"
      >
        <span className="tools-title">
          <Terminal size={14} />
          <span>Tool calls ({toolCalls.length})</span>
        </span>
        {expanded ? <ChevronUpOutlined size={14} /> : <ChevronDownOutlined size={14} />}
      </button>
      {expanded && (
        <div className="tools-body">
          {toolCalls.map((toolCall, index) => (
            <div key={toolCall.id || index} className="tool-row">
              <div className="tool-row-header">
                <span className="tool-name">{toolCall.name}</span>
                <span className="tool-badge ok">running</span>
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
