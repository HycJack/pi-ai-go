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
                        arg.type === 'Identifier' ||    // 支持变量赋值 A = B
                        arg.type === 'FunctionCall' ||  // 支持变量赋值 A = Command(...)
                        arg.type === 'BinaryExpression') {  // 支持变量赋值 A = 2 * pi + 1
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
 * 查找相似的命令名（基于编辑距离和子串匹配）
 */
function findSimilarCommands(target: string, commands: string[], _maxDistance: number = 3): string[] {
    const lowerTarget = target.toLowerCase();
    
    // 动态调整最大编辑距离：较长命令允许更大的距离，按相对比例（目标长度的 50%）
    const maxDistance = Math.min(Math.max(3, Math.floor(target.length * 0.5)), 6);
    
    const similar: Array<{ command: string; distance: number }> = [];
    // 收集"好前缀"匹配：共享至少 50% 前缀且有包含关系
    const prefixMatches: string[] = [];

    for (const command of commands) {
        const lowerCommand = command.toLowerCase();
        const distance = levenshteinDistance(lowerTarget, lowerCommand);

        if (distance <= maxDistance) {
            similar.push({ command, distance });
        } else {
            // 子串匹配：一方包含另一方
            if (lowerCommand.includes(lowerTarget) || lowerTarget.includes(lowerCommand)) {
                prefixMatches.push(command);
                continue;
            }
            // 共享前缀匹配：有共同前缀且长度都较长
            const minLen = Math.min(lowerTarget.length, lowerCommand.length);
            let prefixLen = 0;
            for (let i = 0; i < minLen; i++) {
                if (lowerTarget[i] === lowerCommand[i]) {
                    prefixLen++;
                } else {
                    break;
                }
            }
            if (prefixLen >= 4 && prefixLen / Math.max(lowerTarget.length, lowerCommand.length) >= 0.3) {
                prefixMatches.push(command);
            }
        }
    }

    // 按距离排序，取前 2 个编辑距离结果
    const result: string[] = similar
        .sort((a, b) => a.distance - b.distance)
        .slice(0, 2)
        .map(s => s.command);
    
    // 前缀匹配按与 target 的共同前缀长度降序排列（最长的最相关）
    prefixMatches.sort((a, b) => {
        const aLower = a.toLowerCase(), bLower = b.toLowerCase();
        let aLen = 0, bLen = 0;
        const minLenA = Math.min(aLower.length, lowerTarget.length);
        const minLenB = Math.min(bLower.length, lowerTarget.length);
        for (let i = 0; i < minLenA; i++) { if (aLower[i] === lowerTarget[i]) aLen++; else break; }
        for (let i = 0; i < minLenB; i++) { if (bLower[i] === lowerTarget[i]) bLen++; else break; }
        return bLen - aLen;
    });
    
    // 优先补充前缀/子串匹配结果
    for (const match of prefixMatches) {
        if (!result.includes(match)) {
            result.push(match);
            if (result.length >= 3) break;
        }
    }
    
    // 如果 prefix 补充后仍然不足 3 个，从编辑距离结果中再补充一个
    if (result.length < 3 && similar.length > 2) {
        const third = similar[2];
        if (!result.includes(third.command)) {
            result.push(third.command);
        }
    }
    
    return result;
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
