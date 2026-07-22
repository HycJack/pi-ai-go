/**
 * LanguageStep.tsx — Step 1: Language selection
 */

import React from 'react';
import type { LanguageStepProps } from '../types';
import { StepContainer, Multiline, Actions, PrimaryButton, SecondaryButton } from '../ui';

export function LanguageStep({
  config,
  initialLocale,
  goToStep,
  onLocaleChange,
}: LanguageStepProps) {
  const [locale, setLocale] = React.useState(initialLocale);

  const handleNext = () => {
    onLocaleChange(locale);
    goToStep(2 as any);
  };

  return (
    <StepContainer>
      <h1 className="gonboarding-title">Choose Your Language</h1>
      <Multiline
        className="gonboarding-subtitle"
        text="Select your preferred language for the interface."
      />

      <div className="gonboarding-locale-picker">
        {config.locales.map((loc) => (
          <button
            key={loc.value}
            className={`gonboarding-locale-btn${locale === loc.value ? ' active' : ''}`}
            onClick={() => setLocale(loc.value)}
          >
            {loc.label}
          </button>
        ))}
      </div>

      <Actions>
        <SecondaryButton onClick={() => goToStep(0 as any)}>
          Back
        </SecondaryButton>
        <PrimaryButton onClick={handleNext}>
          Next
        </PrimaryButton>
      </Actions>
    </StepContainer>
  );
}
