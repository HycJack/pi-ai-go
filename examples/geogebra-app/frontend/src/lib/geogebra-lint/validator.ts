import { RuleEngine, noUnknownCommand, correctArgTypes, formatLintResults } from './index';

/**
 * Validate a GeoGebra script and return a validation report.
 * Returns null if validation passes, or a string of error messages if it fails.
 */
export function validateGGB(ggbCode: string): string | null {
  if (!ggbCode || !ggbCode.trim()) return null;

  const engine = new RuleEngine();
  engine.registerRule(noUnknownCommand);
  engine.registerRule(correctArgTypes);

  const result = engine.lint(ggbCode);

  if (result.errorCount === 0 && result.warningCount === 0) {
    return null; // No issues
  }

  return formatLintResults(result);
}
