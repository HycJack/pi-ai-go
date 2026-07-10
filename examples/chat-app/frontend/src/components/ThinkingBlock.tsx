import { useState } from 'react';
import { Brain, ChevronDownOutlined, ChevronUpOutlined } from '../icons';
import MarkdownRenderer from './MarkdownRenderer';

interface ThinkingBlockProps {
  content: string;
  defaultExpanded?: boolean;
}

export default function ThinkingBlock({ content, defaultExpanded = true }: ThinkingBlockProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  if (!content || content.trim() === '') return null;

  return (
    <div className="think-block">
      <button
        onClick={() => setExpanded(!expanded)}
        className="think-header"
      >
        <span className="think-title">
          <span className="think-dot" />
          <span>Thinking</span>
        </span>
        {expanded ? <ChevronUpOutlined size={14} /> : <ChevronDownOutlined size={14} />}
      </button>
      {expanded && (
        <div className="think-body">
          <MarkdownRenderer content={content} />
        </div>
      )}
    </div>
  );
}
