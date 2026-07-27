import { Rule, RuleContext } from './rule';
import { LintSeverity } from '../types-linting';
import { specRegistry, CommandSpec } from '../specs/spec-registry';
import { Expression } from '../parser/ast';

/**
 * 规则: 检查命令参数类型是否正确
 * 
 * 该规则会验证命令的参数数量和类型是否符合 GeoGebra 命令规范。
 * 
 * @example
 * // ❌ 错误：参数数量不对
 * SetValue(a)
 * 
 * // ❌ 错误：参数类型不对
 * SetValue("string", "string")
 * 
 * // ✅ 正确
 * SetValue(a, 1)
 */
export const correctArgTypes: Rule = {
    id: 'correct-arg-types',
    description: '检查命令参数类型是否正确',
    defaultSeverity: LintSeverity.Warning,
    category: 'warning',

    create(context: RuleContext) {
        return {
            CommandStatement(node, ctx) {
                const commandName = node.commandName.name;
                const args = node.arguments;

                // 获取命令规范
                const specs = specRegistry.getCommandSpecs(commandName);
                if (!specs) {
                    // 命令不存在，由 no-unknown-command 规则处理
                    return;
                }

                // 查找最匹配的重载
                const spec = specRegistry.findBestMatch(commandName, args.length);
                if (!spec) {
                    return;
                }

                // 检查参数数量
                checkArgumentCount(node, spec, ctx);

                // 检查参数类型
                checkArgumentTypes(node, spec, ctx);
            }
        };
    },

    meta: {
        fixable: false
    }
};

/**
 * 检查参数数量
 */
function checkArgumentCount(
    node: any,
    spec: CommandSpec,
    context: RuleContext
): void {
    const expectedCount = spec.parameters.length;
    const actualCount = node.arguments.length;

    if (actualCount !== expectedCount) {
        // 检查是否有可选参数
        const optionalCount = spec.parameters.filter(p => p.optional).length;
        const minCount = expectedCount - optionalCount;

        if (actualCount < minCount || actualCount > expectedCount) {
            context.report({
                node: node.commandName,
                message: `命令 "${node.commandName.name}" 期望 ${expectedCount} 个参数，但收到了 ${actualCount} 个`,
                suggestions: [
                    `查看命令签名: ${spec.signature}`,
                    spec.description
                ]
            });
        }
    }
}

/**
 * 检查参数类型
 */
function checkArgumentTypes(
    node: any,
    spec: CommandSpec,
    context: RuleContext
): void {
    const args = node.arguments;

    args.forEach((arg: Expression, index: number) => {
        if (index >= spec.parameters.length) {
            return; // 参数数量问题已经在 checkArgumentCount 中处理
        }

        const expectedParam = spec.parameters[index];
        const actualType = inferArgumentType(arg);

        if (!isTypeCompatible(actualType, expectedParam.type)) {
            // 检查是否有备选类型
            if (expectedParam.alternatives) {
                const compatible = expectedParam.alternatives.some(alt =>
                    isTypeCompatible(actualType, alt)
                );
                if (compatible) {
                    return;
                }
            }

            context.report({
                node: arg,
                message: `参数 ${index + 1} 类型不匹配。期望 ${expectedParam.type}，但收到了 ${actualType}`,
                severity: LintSeverity.Warning,
                suggestions: [
                    `该参数应该是 ${expectedParam.type} 类型`
                ]
            });
        }
    });
}

/**
 * 推断参数类型
 */
function inferArgumentType(arg: Expression): string {
    switch (arg.type) {
        case 'NumberLiteral':
            return 'Number';
        case 'StringLiteral':
            return 'String';
        case 'BooleanLiteral':
            return 'Boolean';
        case 'Identifier':
            // 标识符可以是任何对象
            return 'Object';
        case 'ListLiteral':
            return 'List';
        case 'FunctionCall':
            // 函数调用的返回类型未知，假设为 Object
            return 'Object';
        default:
            return 'Unknown';
    }
}

/**
 * 检查类型兼容性
 */
function isTypeCompatible(actualType: string, expectedType: string): boolean {
    // 精确匹配
    if (actualType === expectedType) {
        return true;
    }

    // Object（标识符）可以匹配任何预期类型
    // 因为在运行时标识符可能引用任何类型的对象
    if (actualType === 'Object') {
        return true;
    }

    // Object 可以接受任何类型（除了原始类型与 Object 的不匹配）
    if (expectedType === 'Object' && actualType !== 'Unknown') {
        return true;
    }

    // Number 可以是整数或小数
    if (expectedType.includes('Number') && actualType === 'Number') {
        return true;
    }

    // 特殊情况：某些命令接受特定值
    if (expectedType.includes('View Number') && actualType === 'Number') {
        return true;
    }

    // Text 和 String 是兼容的
    if ((expectedType === 'Text' && actualType === 'String') ||
        (expectedType === 'String' && actualType === 'Text')) {
        return true;
    }

    // Point, Line, Vector 等几何对象也可以是 Object
    const geometricTypes = ['Point', 'Line', 'Vector', 'Polygon', 'Conic', 'Function'];
    if (geometricTypes.includes(expectedType) && actualType === 'Object') {
        return true;
    }

    return false;
}

/**
 * 获取所有可能的参数类型组合
 */
export function getExpectedTypes(spec: CommandSpec): string {
    return spec.parameters
        .map((param, index) => {
            const typeStr = param.alternatives
                ? `${param.type}(${param.alternatives.join('|')})`
                : param.type;
            return `${index + 1}. ${typeStr}${param.optional ? ' (可选)' : ''}`;
        })
        .join(', ');
}
