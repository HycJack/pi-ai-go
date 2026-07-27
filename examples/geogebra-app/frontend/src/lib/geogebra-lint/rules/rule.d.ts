import { Program, CommandStatement, ASTNode } from '../parser/ast';
import { LintSeverity } from '../types-linting';

/**
 * 规则上下文，提供给规则使用的工具和信息
 */
export interface RuleContext {
    /**
     * 报告一个 lint 问题
     */
    report(options: {
        node: ASTNode;
        message: string;
        severity?: LintSeverity;
        suggestions?: string[];
    }): void;

    /**
     * 获取源代码
     */
    getSourceCode(): string;

    /**
     * 获取 AST 根节点
     */
    getProgram(): Program;

    /**
     * 规则配置选项
     */
    options: any;
}

/**
 * 规则访问器，定义如何访问 AST 节点
 */
export interface RuleVisitor {
    /**
     * 访问程序根节点
     */
    Program?: (node: Program, context: RuleContext) => void;

    /**
     * 访问命令语句节点
     */
    CommandStatement?: (node: CommandStatement, context: RuleContext) => void;

    /**
     * 退出程序节点时调用
     */
    'Program:exit'?: (node: Program, context: RuleContext) => void;

    /**
     * 退出命令语句节点时调用
     */
    'CommandStatement:exit'?: (node: CommandStatement, context: RuleContext) => void;
}

/**
 * 规则定义
 */
export interface Rule {
    /**
     * 规则的唯一标识符
     */
    id: string;

    /**
     * 规则的描述
     */
    description: string;

    /**
     * 默认严重程度
     */
    defaultSeverity: LintSeverity;

    /**
     * 规则的分类
     */
    category: 'error' | 'warning' | 'suggestion';

    /**
     * 创建规则访问器
     */
    create(context: RuleContext): RuleVisitor;

    /**
     * 规则的元数据（可选）
     */
    meta?: {
        fixable?: boolean;      // 是否可自动修复
        deprecated?: boolean;   // 是否已弃用
        replacedBy?: string[];  // 被哪些规则替代
    };
}
