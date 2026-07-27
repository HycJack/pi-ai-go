import { useState } from 'react';
import { Brain, Terminal, ChevronDownOutlined, ChevronUpOutlined, CheckOutlined, CloseOutlined, Loader2 } from '../icons';
import MarkdownRenderer from './MarkdownRenderer';
import type { AgentStep } from '../types';
import { useT } from '../i18n';

interface AgentStepsProps {
  steps: AgentStep[];
  isLoading?: boolean;
}

function StepIcon({ step }: { step: AgentStep }) {
  switch (step.type) {
    case 'thinking':
      return <Brain size={13} />;
    case 'tool_call':
    case 'tool_result':
      return <Terminal size={13} />;
    default:
      return null;
  }
}

function StepStatus({ step }: { step: AgentStep }) {
  if (step.status === 'running') {
    return <Loader2 size={12} />;
  }
  if (step.status === 'error') {
    return <CloseOutlined size={12} />;
  }
  if (step.status === 'done') {
    return <CheckOutlined size={12} />;
  }
  return null;
}

function StepLabel({ step }: { step: AgentStep }) {
  const t = useT();
  switch (step.type) {
    case 'thinking':
      return <span>{t('thinking.title')}</span>;
    case 'tool_call':
      return <span>{step.toolName || 'tool'}</span>;
    case 'tool_result':
      return <span>{t('tool.result')}</span>;
    case 'text':
      return <span>{t('msg.assistant')}</span>;
    default:
      return null;
  }
}

function StepContent({ step }: { step: AgentStep }) {
  if (!step.content) return null;

  switch (step.type) {
    case 'thinking':
    case 'text':
      return (
        <div className="agent-step-content">
          <MarkdownRenderer content={step.content} />
        </div>
      );
    case 'tool_call':
      return (
        <div className="agent-step-content">
          <pre className="agent-step-code"><code>{step.content}</code></pre>
        </div>
      );
    case 'tool_result':
      return (
        <div className="agent-step-content">
          <pre className="agent-step-code agent-step-result"><code>{step.content}</code></pre>
        </div>
      );
    default:
      return null;
  }
}

function StepItem({ step, isLoading }: { step: AgentStep; isLoading?: boolean }) {
  const [expanded, setExpanded] = useState(false);
  const hasContent = step.content && step.content.trim().length > 0;
  const isRunning = step.status === 'running' && isLoading;

  return (
    <div className={`agent-step agent-step-${step.type}`}>
      <button
        className="agent-step-header"
        onClick={() => hasContent && setExpanded(!expanded)}
        disabled={!hasContent}
      >
        <span className="agent-step-icon">
          <StepIcon step={step} />
        </span>
        <span className="agent-step-label">
          <StepLabel step={step} />
        </span>
        <span className="agent-step-status">
          {isRunning ? <Loader2 size={12} /> : <StepStatus step={step} />}
        </span>
        {hasContent && (
          <span className="agent-step-chevron">
            {expanded ? <ChevronUpOutlined size={12} /> : <ChevronDownOutlined size={12} />}
          </span>
        )}
      </button>
      {expanded && hasContent && <StepContent step={step} />}
    </div>
  );
}

export default function AgentSteps({ steps, isLoading }: AgentStepsProps) {
  const t = useT();
  const [expanded, setExpanded] = useState(false);

  if (!steps || steps.length === 0) return null;

  // Count step types for the summary.
  const toolCount = steps.filter((s) => s.type === 'tool_call').length;
  const thinkCount = steps.filter((s) => s.type === 'thinking').length;
  const textCount = steps.filter((s) => s.type === 'text').length;

  const summary: string[] = [];
  if (thinkCount > 0) summary.push(`${thinkCount} ${t('thinking.title')}`);
  if (toolCount > 0) summary.push(`${toolCount} ${t('tool.calls')}`);
  if (textCount > 0 && steps.some((s) => s.type === 'text' && s.content !== steps[steps.length - 1]?.content)) {
    summary.push(`${textCount} ${t('msg.assistant')}`);
  }

  return (
    <div className="agent-steps">
      <button
        className="agent-steps-header"
        onClick={() => setExpanded(!expanded)}
      >
        <span className="agent-steps-summary">
          {summary.join(' · ') || t('tool.process')}
        </span>
        <span className="agent-steps-chevron">
          {expanded ? <ChevronUpOutlined size={14} /> : <ChevronDownOutlined size={14} />}
        </span>
      </button>
      {expanded && (
        <div className="agent-steps-body">
          {steps.map((step, i) => (
            <StepItem key={i} step={step} isLoading={isLoading} />
          ))}
        </div>
      )}
    </div>
  );
}
