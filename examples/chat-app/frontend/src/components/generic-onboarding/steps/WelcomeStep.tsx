/**
 * WelcomeStep.tsx — Step 0: Welcome screen with locale selection
 */

import { useState, useCallback } from 'react';
import { OnboardingStep } from '../types';
import type { LocaleOption, WelcomeStepProps } from '../types';
import { StepContainer, Multiline, Actions, PrimaryButton } from '../ui';

export function WelcomeStep({
  config,
  avatarSrc,
  onComplete,
  showError,
}: WelcomeStepProps) {
  const [locale, setLocale] = useState(config.locales[0]?.value ?? 'en');

  const handleNext = useCallback(async () => {
    try {
      await onComplete({
        locale,
        identity: { userName: '', agentName: '', memoryEnabled: true },
        model: {
          chatModel: '',
          providerName: '',
          providerUrl: '',
          apiType: 'openai-completions',
          apiKey: '',
          addedModels: [],
        },
        theme: config.themes[0]?.id ?? 'default',
        workspace: { path: '', useDefault: true },
        step: OnboardingStep.Welcome,
      } as any);
    } catch (err: any) {
      showError(err.message ?? String(err));
    }
  }, [config, locale, onComplete, showError]);

  return (
    <StepContainer>
      {avatarSrc && (
        <img className="gonboarding-avatar" src={avatarSrc} draggable={false} alt="" />
      )}

      <h1 className="gonboarding-title">
        {config.branding?.welcomeTitle ?? `Welcome to ${config.branding?.appName ?? 'AI Chat'}`}
      </h1>

      <Multiline
        className="gonboarding-subtitle"
        text={config.branding?.welcomeSubtitle ?? 'Choose your language to get started.'}
      />

      <div className="gonboarding-locale-picker">
        {config.locales.map((loc: LocaleOption) => (
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
        <PrimaryButton onClick={handleNext}>
          Get Started
        </PrimaryButton>
      </Actions>
    </StepContainer>
  );
}
