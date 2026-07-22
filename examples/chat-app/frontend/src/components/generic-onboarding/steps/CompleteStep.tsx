/**
 * CompleteStep.tsx — Step 5: Tutorial cards + finish
 */

import React from 'react';
import type { CompleteStepProps } from '../types';
import { StepContainer, Multiline, PrimaryButton } from '../ui';

// ── Feature card ──

interface FeatureCardProps {
  icon: React.ReactNode;
  title: string;
  description: string;
}

function FeatureCard({ icon, title, description }: FeatureCardProps) {
  return (
    <div className="gonboarding-feature-card">
      <div className="gonboarding-feature-header">
        <span className="gonboarding-feature-icon">{icon}</span>
        <span className="gonboarding-feature-title">{title}</span>
      </div>
      <Multiline className="gonboarding-feature-desc" text={description} />
    </div>
  );
}

// ── Default features ──

const DEFAULT_FEATURES = [
  {
    key: 'memory',
    title: 'Memory',
    description: 'Your AI assistant remembers context\nacross conversations for a seamless experience.',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" width="20" height="20">
        <circle cx="12" cy="12" r="9" />
        <path d="M12 8v0m0 8c0-2 1.5-2.5 1.5-4.5a1.5 1.5 0 10-3 0C10.5 13.5 12 14 12 16z" />
      </svg>
    ),
  },
  {
    key: 'skills',
    title: 'Skills & Tools',
    description: 'Extend capabilities with plugins,\nweb search, file operations, and more.',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" width="20" height="20">
        <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z" />
      </svg>
    ),
  },
  {
    key: 'workspace',
    title: 'Workspace',
    description: 'Your AI has a dedicated workspace\nfor managing files and projects.',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" width="20" height="20">
        <path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z" />
      </svg>
    ),
  },
  {
    key: 'multiagent',
    title: 'Multi-Agent',
    description: 'Create multiple AI agents\nwith different roles and personalities.',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" width="20" height="20">
        <circle cx="8" cy="8" r="3" />
        <circle cx="17" cy="7" r="2.5" />
        <path d="M3.5 19a4.5 4.5 0 019 0" />
        <path d="M13.5 18.5a3.5 3.5 0 017 0" />
      </svg>
    ),
  },
  {
    key: 'chat',
    title: 'Chat & More',
    description: 'Send text, images, code, and files.\nYour AI understands multiple formats.',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" width="20" height="20">
        <path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z" />
      </svg>
    ),
  },
];

export function CompleteStep({
  config,
  result,
  goToStep,
  onFinish,
}: CompleteStepProps) {
  const [finishing, setFinishing] = React.useState(false);

  const handleFinish = async () => {
    setFinishing(true);
    try {
      await onFinish();
    } finally {
      setFinishing(false);
    }
  };

  const features = DEFAULT_FEATURES;

  return (
    <StepContainer>
      <h1 className="gonboarding-title">
        You're All Set!
      </h1>
      <Multiline
        className="gonboarding-subtitle"
        text={`Your AI assistant is ready.\nHere's what you can do:`}
      />

      <div className="gonboarding-feature-cards">
        {features.map((feature) => (
          <FeatureCard
            key={feature.key}
            icon={feature.icon}
            title={feature.title}
            description={feature.description}
          />
        ))}
      </div>

      <div className="gonboarding-summary">
        <div className="gonboarding-summary-item">
          <span>Language:</span> {result.locale}
        </div>
        <div className="gonboarding-summary-item">
          <span>Theme:</span> {result.theme}
        </div>
        {result.model.chatModel && (
          <div className="gonboarding-summary-item">
            <span>Model:</span> {result.model.chatModel}
          </div>
        )}
      </div>

      <button
        className="gonboarding-finish-btn"
        disabled={finishing}
        onClick={handleFinish}
      >
        {finishing ? 'Starting...' : "Let's Go!"}
      </button>
    </StepContainer>
  );
}
