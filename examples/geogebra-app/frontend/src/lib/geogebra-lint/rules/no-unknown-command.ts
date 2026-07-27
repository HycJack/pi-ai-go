import { Rule, RuleContext } from './rule';
import { LintSeverity } from '../types-linting';
import { specRegistry } from '../specs/spec-registry';

/**
 * 规则: 检测未知的 GeoGebra 命令
 * 
 * 该规则会检查所有命令调用，确保命令名存在于 GeoGebra 命令规范中。
 * 
 * @example
 * // ❌ 错误
 * UnknownCommand(a, 1)
 * 
 * // ✅ 正确
 * SetValue(a, 1)
 */
export const noUnknownCommand: Rule = {
    id: 'no-unknown-command',
    description: '检测未知的 GeoGebra 命令',
    defaultSeverity: LintSeverity.Error,
    category: 'error',

    create(context: RuleContext) {
        return {
            CommandStatement(node, ctx) {
                const commandName = node.commandName.name;

                // 跳过变量赋值语句 (如: A = {1, 2, 3}, A = (0, 0), x = 5)
                // 这些语句的特征是：只有一个参数，且参数不是函数调用
                if (node.arguments.length === 1) {
                    const arg = node.arguments[0];
                    // 如果参数是字面量（列表、元组、数字、字符串、布尔值），说明这是赋值语句，不是命令
                    if (arg.type === 'ListLiteral' || 
                        arg.type === 'TupleLiteral' ||  // 支持点字面量 A = (0, 0, 3)
                        arg.type === 'NumberLiteral' || 
                        arg.type === 'StringLiteral' || 
                        arg.type === 'BooleanLiteral' ||
                        arg.type === 'Identifier') {  // 支持变量赋值 A = B
                        return; // 跳过检查
                    }
                }

                // 检查命令是否存在于规范中
                if (!specRegistry.hasCommand(commandName)) {
                    const allCommands = specRegistry.getAllCommandNames();
                    
                    // 尝试找到相似的命令（简单的相似度匹配）
                    const suggestions = findSimilarCommands(commandName, allCommands);

                    ctx.report({
                        node: node.commandName,
                        message: `未知的命令 "${commandName}"`,
                        suggestions: suggestions.length > 0 
                            ? [`你是否想使用: ${suggestions.join(', ')}?`]
                            : ['请检查命令名是否正确']
                    });
                }
            }
        };
    },

    meta: {
        fixable: false
    }
};

/**
 * 查找相似的命令名（基于编辑距离）
 */
function findSimilarCommands(target: string, commands: string[], maxDistance: number = 3): string[] {
    const similar: Array<{ command: string; distance: number }> = [];

    for (const command of commands) {
        const distance = levenshteinDistance(
            target.toLowerCase(),
            command.toLowerCase()
        );

        if (distance <= maxDistance) {
            similar.push({ command, distance });
        }
    }

    // 按距离排序，返回前 3 个
    return similar
        .sort((a, b) => a.distance - b.distance)
        .slice(0, 3)
        .map(s => s.command);
}

/**
 * 计算编辑距离（Levenshtein Distance）
 */
function levenshteinDistance(str1: string, str2: string): number {
    const len1 = str1.length;
    const len2 = str2.length;
    const matrix: number[][] = [];

    // 初始化矩阵
    for (let i = 0; i <= len1; i++) {
        matrix[i] = [i];
    }

    for (let j = 0; j <= len2; j++) {
        matrix[0][j] = j;
    }

    // 填充矩阵
    for (let i = 1; i <= len1; i++) {
        for (let j = 1; j <= len2; j++) {
            const cost = str1[i - 1] === str2[j - 1] ? 0 : 1;
            matrix[i][j] = Math.min(
                matrix[i - 1][j] + 1,      // 删除
                matrix[i][j - 1] + 1,      // 插入
                matrix[i - 1][j - 1] + cost // 替换
            );
        }
    }

    return matrix[len1][len2];
}
