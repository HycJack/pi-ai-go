// GeoGebra Lint - 重新导出所有公共 API（扁平化结构适配）

export { Parser, parseGeoGebraScript, ParseError } from './parser/parser';
export { Lexer, TokenType } from './parser/lexer';
export type { Token } from './parser/lexer';

export type {
  Program,
  CommandStatement,
  Expression,
  Identifier,
  NumberLiteral,
  StringLiteral,
  BooleanLiteral,
  ListLiteral,
  TupleLiteral,
  FunctionCall,
  BinaryExpression,
  ASTNode,
  BaseNode,
  SourceLocation,
  Position,
} from './parser/ast';

export { RuleEngine, formatLintResults } from './rules/rule-engine';
export type { Rule, RuleContext, RuleVisitor } from './rules/rule';

export { noUnknownCommand } from './rules/no-unknown-command';
export { correctArgTypes } from './rules/correct-arg-types';

export { SpecRegistry, specRegistry } from './specs/spec-registry';
export type { CommandSignature, CommandSpec, CommandParameter, ArgumentType } from './specs/spec-registry';

export type { LintMessage, LintResult, LintConfig } from './types-linting';
export { LintSeverity } from './types-linting';
