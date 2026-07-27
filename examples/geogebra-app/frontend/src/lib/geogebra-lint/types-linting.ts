import { ASTNode, SourceLocation } from './parser/ast';

/**
 * Lint 消息的严重程度
 */
export enum LintSeverity {
    Error = 'error',
    Warning = 'warning',
    Info = 'info'
}

/**
 * Lint 问题信息
 */
export interface LintMessage {
    ruleId: string;           // 规则 ID
    message: string;          // 错误消息
    severity: LintSeverity;   // 严重程度
    loc: SourceLocation;      // 源码位置
    node?: ASTNode;           // 相关的 AST 节点
    suggestions?: string[];   // 修复建议
}

/**
 * Lint 结果
 */
export interface LintResult {
    filePath?: string;        // 文件路径（可选）
    source: string;           // 源代码
    messages: LintMessage[];  // 所有 lint 消息
    errorCount: number;       // 错误数量
    warningCount: number;     // 警告数量
}

/**
 * Lint 配置
 */
export interface LintConfig {
    rules: {
        [ruleId: string]: 'off' | 'warn' | 'error' | [LintSeverity, any];
    };
}
