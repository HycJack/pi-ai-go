import { Program, CommandStatement, ASTNode } from '../parser/ast';
import { Rule, RuleContext, RuleVisitor } from './rule';
import { LintMessage, LintResult, LintSeverity, LintConfig } from '../types-linting';
import { parseGeoGebraScript } from '../parser/parser';

/**
 * 规则引擎 - 负责运行所有规则并收集 lint 消息
 */
export class RuleEngine {
    private rules: Map<string, Rule> = new Map();
    private config: LintConfig;

    constructor(config?: LintConfig) {
        this.config = config || { rules: {} };
    }

    /**
     * 注册一个规则
     */
    public registerRule(rule: Rule): void {
        this.rules.set(rule.id, rule);
    }

    /**
     * 注册多个规则
     */
    public registerRules(rules: Rule[]): void {
        rules.forEach(rule => this.registerRule(rule));
    }

    /**
     * 获取规则的配置严重程度
     */
    private getRuleSeverity(ruleId: string, defaultSeverity: LintSeverity): LintSeverity | null {
        const ruleConfig = this.config.rules[ruleId];

        if (ruleConfig === 'off') {
            return null;
        }

        if (ruleConfig === 'warn') {
            return LintSeverity.Warning;
        }

        if (ruleConfig === 'error') {
            return LintSeverity.Error;
        }

        if (Array.isArray(ruleConfig)) {
            return ruleConfig[0];
        }

        return defaultSeverity;
    }

    /**
     * 获取规则的配置选项
     */
    private getRuleOptions(ruleId: string): any {
        const ruleConfig = this.config.rules[ruleId];

        if (Array.isArray(ruleConfig) && ruleConfig.length > 1) {
            return ruleConfig[1];
        }

        return {};
    }

    /**
     * Lint 源代码
     */
    public lint(source: string, filePath?: string): LintResult {
        const messages: LintMessage[] = [];

        try {
            // 解析源代码为 AST
            const ast = parseGeoGebraScript(source);

            // 运行所有规则
            this.rules.forEach((rule, ruleId) => {
                const severity = this.getRuleSeverity(ruleId, rule.defaultSeverity);

                // 如果规则被禁用，跳过
                if (severity === null) {
                    return;
                }

                const options = this.getRuleOptions(ruleId);

                // 创建规则上下文
                const context = this.createContext(
                    ast,
                    source,
                    ruleId,
                    severity,
                    options,
                    messages
                );

                // 创建规则访问器
                const visitor = rule.create(context);

                // 遍历 AST
                this.traverseAST(ast, visitor, context);
            });

        } catch (error) {
            // 解析错误
            messages.push({
                ruleId: 'parse-error',
                message: error instanceof Error ? error.message : String(error),
                severity: LintSeverity.Error,
                loc: {
                    start: { line: 1, column: 1 },
                    end: { line: 1, column: 1 }
                }
            });
        }

        // 统计错误和警告数量
        const errorCount = messages.filter(m => m.severity === LintSeverity.Error).length;
        const warningCount = messages.filter(m => m.severity === LintSeverity.Warning).length;

        return {
            filePath,
            source,
            messages,
            errorCount,
            warningCount
        };
    }

    /**
     * 创建规则上下文
     */
    private createContext(
        ast: Program,
        source: string,
        ruleId: string,
        severity: LintSeverity,
        options: any,
        messages: LintMessage[]
    ): RuleContext {
        return {
            report: ({ node, message, severity: customSeverity, suggestions }) => {
                messages.push({
                    ruleId,
                    message,
                    severity: customSeverity || severity,
                    loc: node.loc,
                    node,
                    suggestions
                });
            },
            getSourceCode: () => source,
            getProgram: () => ast,
            options
        };
    }

    /**
     * 遍历 AST 并调用访问器
     */
    private traverseAST(node: ASTNode, visitor: RuleVisitor, context: RuleContext): void {
        // 进入节点
        if (node.type === 'Program' && visitor.Program) {
            visitor.Program(node as Program, context);
        } else if (node.type === 'CommandStatement' && visitor.CommandStatement) {
            visitor.CommandStatement(node as CommandStatement, context);
        }

        // 递归遍历子节点
        if (node.type === 'Program') {
            const program = node as Program;
            program.body.forEach(child => this.traverseAST(child, visitor, context));
        } else if (node.type === 'CommandStatement') {
            const cmd = node as CommandStatement;
            this.traverseAST(cmd.commandName, visitor, context);
            cmd.arguments.forEach(arg => this.traverseAST(arg, visitor, context));
        } else if (node.type === 'FunctionCall') {
            const func = node as any;
            this.traverseAST(func.callee, visitor, context);
            func.arguments.forEach((arg: ASTNode) => this.traverseAST(arg, visitor, context));
        } else if (node.type === 'ListLiteral') {
            const list = node as any;
            list.elements.forEach((elem: ASTNode) => this.traverseAST(elem, visitor, context));
        }

        // 退出节点
        if (node.type === 'Program' && visitor['Program:exit']) {
            visitor['Program:exit'](node as Program, context);
        } else if (node.type === 'CommandStatement' && visitor['CommandStatement:exit']) {
            visitor['CommandStatement:exit'](node as CommandStatement, context);
        }
    }

    /**
     * 获取所有注册的规则
     */
    public getRules(): Map<string, Rule> {
        return this.rules;
    }

    /**
     * 获取规则数量
     */
    public getRuleCount(): number {
        return this.rules.size;
    }

    /**
     * 设置配置
     */
    public setConfig(config: LintConfig): void {
        this.config = config;
    }
}

/**
 * 格式化 lint 结果为可读的文本
 */
export function formatLintResults(results: LintResult): string {
    const lines: string[] = [];

    if (results.filePath) {
        lines.push(`\n文件: ${results.filePath}`);
    }

    if (results.messages.length === 0) {
        lines.push('✓ 没有发现问题');
        return lines.join('\n');
    }

    lines.push(`\n发现 ${results.errorCount} 个错误，${results.warningCount} 个警告：\n`);

    results.messages.forEach((msg, index) => {
        const icon = msg.severity === LintSeverity.Error ? '❌' :
                     msg.severity === LintSeverity.Warning ? '⚠️' : 'ℹ️';
        
        const location = `${msg.loc.start.line}:${msg.loc.start.column}`;
        
        lines.push(`${index + 1}. ${icon} [${msg.ruleId}] ${msg.message}`);
        lines.push(`   位置: 行 ${location}`);

        if (msg.suggestions && msg.suggestions.length > 0) {
            lines.push(`   建议: ${msg.suggestions.join(', ')}`);
        }

        lines.push('');
    });

    return lines.join('\n');
}
