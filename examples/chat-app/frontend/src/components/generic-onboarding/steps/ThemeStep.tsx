/**
 * ThemeStep.tsx — Step 3: Theme selection
 */

import { useState } from 'react';
import type { ThemeStepProps, ThemeOption } from '../types';
import { StepContainer, Multiline, Actions, PrimaryButton, SecondaryButton } from '../ui';

export function ThemeStep({
  config,
  activeTheme: initialTheme,
  goToStep,
  onThemeChange,
}: ThemeStepProps) {
  const [activeTheme, setActiveTheme] = useState(initialTheme);

  const handleSelect = (themeId: string) => {
    setActiveTheme(themeId);
    onThemeChange(themeId);
  };

  return (
    <StepContainer>
      <h1 className="gonboarding-title">Choose Your Theme</h1>
      <Multiline
        className="gonboarding-subtitle"
        text="Pick a look and feel that suits you."
      />

      <div className="gonboarding-theme-grid">
        {config.themes.map((theme: ThemeOption) => (
          <button
            key={theme.id}
            className={`gonboarding-theme-card${activeTheme === theme.id ? ' active' : ''}`}
            onClick={() => handleSelect(theme.id)}
          >
            <div className="gonboarding-theme-preview" data-theme={theme.id} />
            <div className="gonboarding-theme-name">{theme.label}</div>
            {theme.description && (
              <div className="gonboarding-theme-desc">{theme.description}</div>
            )}
          </button>
        ))}
      </div>

      <Actions>
        <SecondaryButton onClick={() => goToStep(2 as any)}>
          Back
        </SecondaryButton>
        <PrimaryButton onClick={() => goToStep(4 as any)}>
          Next
        </PrimaryButton>
      </Actions>
    </StepContainer>
  );
}
