import { Brain, ChevronDown, ChevronUp } from 'lucide-react';
import { useState } from 'react';
import MarkdownRenderer from './MarkdownRenderer';

interface ThinkingBlockProps {
  content: string;
  defaultExpanded?: boolean;
}

export default function ThinkingBlock({ content, defaultExpanded = true }: ThinkingBlockProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  if (!content || content.trim() === '') {
    return null;
  }

  return (
    <div className="bg-amber-900/30 border border-amber-700/50 rounded-lg overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between p-3 hover:bg-amber-900/40 transition-colors"
      >
        <div className="flex items-center gap-2">
          <Brain className="w-4 h-4 text-amber-500" />
          <span className="text-sm font-medium text-amber-400">思考</span>
        </div>
        {expanded ? (
          <ChevronUp className="w-4 h-4 text-amber-400" />
        ) : (
          <ChevronDown className="w-4 h-4 text-amber-400" />
        )}
      </button>
      {expanded && (
        <div className="px-3 pb-3">
          <MarkdownRenderer content={content} />
        </div>
      )}
    </div>
  );
}
