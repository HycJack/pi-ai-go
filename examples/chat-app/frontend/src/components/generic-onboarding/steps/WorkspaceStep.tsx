/**
 * WorkspaceStep.tsx — Step 4: Workspace (project directory) selection
 */

import { useState, useCallback } from 'react';
import type { WorkspaceStepProps } from '../types';
import { StepContainer, Multiline, Actions, PrimaryButton, SecondaryButton } from '../ui';

export function WorkspaceStep({
  config,
  workspace,
  goToStep,
  onWorkspaceChange,
  selectFolder,
}: WorkspaceStepProps) {
  const [customPath, setCustomPath] = useState(
    workspace.useDefault ? '' : workspace.path
  );
  const [showCustom, setShowCustom] = useState(!workspace.useDefault);

  const defaultPath = `~/Desktop/${config.defaultWorkspaceDirname}`;
  const visiblePath = customPath || defaultPath;
  const usingDefault = !customPath;

  const handleBrowse = useCallback(async () => {
    if (!selectFolder) return;
    const folder = await selectFolder();
    if (folder) {
      setCustomPath(folder);
      setShowCustom(true);
    }
  }, [selectFolder]);

  const handleUseDefault = useCallback(() => {
    setCustomPath('');
    setShowCustom(false);
  }, []);

  const handleNext = useCallback(() => {
    onWorkspaceChange({
      path: visiblePath,
      useDefault: usingDefault,
    });
    goToStep(5 as any);
  }, [visiblePath, usingDefault, onWorkspaceChange, goToStep]);

  return (
    <StepContainer>
      <h1 className="gonboarding-title">Workspace Directory</h1>
      <Multiline
        className="gonboarding-subtitle"
        text="Choose where your AI assistant will store project files."
      />

      <div className="gonboarding-workspace-card">
        <div className="gonboarding-workspace-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" width="32" height="32">
            <path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z" />
          </svg>
        </div>
        <div className="gonboarding-workspace-path">{visiblePath}</div>
        <div className="gonboarding-workspace-hint">
          {usingDefault
            ? 'Default workspace – your files will be stored here.'
            : 'Custom location selected.'}
        </div>
      </div>

      <div className="gonboarding-workspace-actions">
        <button className="gonboarding-test-btn" onClick={handleBrowse}>
          Browse...
        </button>
        {!usingDefault && (
          <button className="gonboarding-test-btn" onClick={handleUseDefault}>
            Use Default
          </button>
        )}
      </div>

      <Actions>
        <SecondaryButton onClick={() => goToStep(3 as any)}>
          Back
        </SecondaryButton>
        <PrimaryButton onClick={handleNext}>
          Next
        </PrimaryButton>
      </Actions>
    </StepContainer>
  );
}
