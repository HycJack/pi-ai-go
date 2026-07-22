/**
 * ui.tsx — Shared UI primitives for the generic onboarding wizard
 */

import React from 'react';

// ── Step container ──

interface StepContainerProps {
  children: React.ReactNode;
}

export function StepContainer({ children }: StepContainerProps) {
  return <div className="gonboarding-step active">{children}</div>;
}

// ── Multiline text helper ──

interface MultilineProps {
  className?: string;
  text: string;
}

export function Multiline({ className, text }: MultilineProps) {
  const parts = text.split('\n');
  return (
    <p className={className}>
      {parts.map((line, idx) => (
        <span key={idx}>
          {idx > 0 && <br />}
          {line}
        </span>
      ))}
    </p>
  );
}

// ── Progress dots ──

interface ProgressDotsProps {
  total: number;
  current: number;
  labels?: string[];
}

export function ProgressDots({ total, current, labels }: ProgressDotsProps) {
  return (
    <div className="gonboarding-progress" role="progressbar" aria-valuenow={current + 1} aria-valuemin={1} aria-valuemax={total}>
      {Array.from({ length: total }, (_, i) => (
        <div
          key={i}
          className={`gonboarding-dot${i === current ? ' active' : ''}${i < current ? ' done' : ''}`}
          title={labels?.[i] ?? `Step ${i + 1}`}
        />
      ))}
    </div>
  );
}

// ── Toast ──

interface ToastProps {
  message: string;
}

export function Toast({ message }: ToastProps) {
  if (!message) return null;
  return (
    <div className="gonboarding-toast">
      {message}
    </div>
  );
}

// ── Action buttons row ──

interface ActionsProps {
  children: React.ReactNode;
}

export function Actions({ children }: ActionsProps) {
  return <div className="gonboarding-actions">{children}</div>;
}

// ── Primary button ──

interface PrimaryButtonProps {
  onClick: () => void;
  disabled?: boolean;
  children: React.ReactNode;
}

export function PrimaryButton({ onClick, disabled, children }: PrimaryButtonProps) {
  return (
    <button className="gob-btn gob-btn-primary" onClick={onClick} disabled={disabled}>
      {children}
    </button>
  );
}

// ── Secondary button ──

interface SecondaryButtonProps {
  onClick: () => void;
  children: React.ReactNode;
}

export function SecondaryButton({ onClick, children }: SecondaryButtonProps) {
  return (
    <button className="gob-btn gob-btn-secondary" onClick={onClick}>
      {children}
    </button>
  );
}
