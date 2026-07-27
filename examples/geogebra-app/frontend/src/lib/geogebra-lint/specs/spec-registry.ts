import commandSignatures from './commandSignatures.json';

/**
 * 命令签名信息
 */
export interface CommandSignature {
    signature: string;        // 完整签名，如 "SetValue( <Object>, <Number> )"
    commandBase: string;      // 命令名，如 "SetValue"
    description: string;      // 命令描述
    examples: Array<{
        description: string;
        command: string;
    }>;
    note: string;
}

/**
 * 参数类型定义
 */
export type ArgumentType = 
    | 'Object'
    | 'Number'
    | 'Boolean'
    | 'String'
    | 'Point'
    | 'Line'
    | 'Vector'
    | 'Polygon'
    | 'Conic'
    | 'Function'
    | 'List'
    | 'Text'
    | 'Angle'
    | 'Matrix'
    | 'Quadric'
    | 'View Number'
    | string; // 其他类型

/**
 * 解析后的命令参数信息
 */
export interface CommandParameter {
    type: ArgumentType;
    optional: boolean;
    alternatives?: ArgumentType[]; // 对于 | 分隔的类型
}

/**
 * 解析后的命令规范
 */
export interface CommandSpec {
    name: string;
    parameters: CommandParameter[];
    description: string;
    signature: string;
    examples: Array<{ description: string; command: string }>;
    note: string;
}

/**
 * GeoGebra 命令规范注册表
 */
export class SpecRegistry {
    private specs: Map<string, CommandSpec[]> = new Map();
    private allCommandNames: Set<string> = new Set();

    constructor() {
        this.loadSpecs();
    }

    /**
     * 加载命令签名
     */
    private loadSpecs(): void {
        const signatures = commandSignatures as CommandSignature[];

        signatures.forEach(sig => {
            // 支持 signature 中通过顶层 | 分隔的多个重载签名，例如:
            // "SetColor( ... ) | SetColor( ... )"
            const parts = this.splitTopLevelSignatures(sig.signature);

            parts.forEach(part => {
                const spec = this.parseSignaturePart(part.trim(), sig.commandBase, sig.description, sig.examples, sig.note);

                if (!this.specs.has(spec.name)) {
                    this.specs.set(spec.name, []);
                }

                this.specs.get(spec.name)!.push(spec);
                this.allCommandNames.add(spec.name);
            });
        });
    }

    /**
     * 解析命令签名
     * 例如: "SetValue( <Object>, <Number> )" -> { name: 'SetValue', parameters: [...] }
     */
    /**
     * 将单个签名片段解析为 CommandSpec
     */
    private parseSignaturePart(signaturePart: string, commandBase: string, description: string, examples: any, note: string): CommandSpec {
        // 提取参数部分（匹配第一个小括号内的内容）
        const match = signaturePart.match(/\((.*?)\)/);
        const parameters: CommandParameter[] = [];

        if (match && match[1].trim()) {
            const paramStr = match[1];
            const params = this.splitParameters(paramStr);

            params.forEach(param => {
                const parsedParam = this.parseParameter(param.trim());
                if (parsedParam) {
                    parameters.push(parsedParam);
                }
            });
        }

        return {
            name: commandBase,
            parameters,
            description,
            signature: signaturePart,
            examples,
            note
        };
    }

    /**
     * 将完整 signature 字符串按顶层 | 分割为多个签名片段（忽略括号内的 |）
     */
    private splitTopLevelSignatures(signature: string): string[] {
        const parts: string[] = [];
        let current = '';
        let depth = 0;

        for (let i = 0; i < signature.length; i++) {
            const ch = signature[i];

            if (ch === '(') {
                depth++;
            } else if (ch === ')') {
                depth = Math.max(0, depth - 1);
            }

            if (ch === '|' && depth === 0) {
                parts.push(current);
                current = '';
                continue;
            }

            current += ch;
        }

        if (current.trim()) parts.push(current);
        return parts;
    }

    /**
     * 分割参数字符串
     */
    private splitParameters(paramStr: string): string[] {
        const params: string[] = [];
        let current = '';
        let depth = 0;

        for (let i = 0; i < paramStr.length; i++) {
            const char = paramStr[i];

            if (char === '<') {
                depth++;
            } else if (char === '>') {
                depth--;
            } else if (char === ',' && depth === 0) {
                params.push(current.trim());
                current = '';
                continue;
            }

            current += char;
        }

        if (current.trim()) {
            params.push(current.trim());
        }

        return params;
    }

    /**
     * 解析单个参数
     * 例如: "<Object>", "<Number 1|2|-1>", "..."
     */
    private parseParameter(param: string): CommandParameter | null {
        // 可变参数
        if (param === '...') {
            return {
                type: 'any',
                optional: true
            };
        }

        // 移除 < > 符号
        const cleaned = param.replace(/[<>]/g, '').trim();

        // 检查是否可选（某些情况下）
        const optional = false; // 可以根据需要扩展

        // 如果是带引号的文字类型（例如 "Color"），视为 String
        if (/^".*"$/.test(cleaned)) {
            return {
                type: 'String',
                optional
            };
        }

        // 处理多选类型，如 "Number 1|2|-1"
        if (cleaned.includes('|')) {
            // 对于像 "Number 1|2|-1" 这样的情况，首项通常是类型
            const parts = cleaned.split(/\s+/);
            const baseType = parts[0];
            return {
                type: baseType,
                optional,
                alternatives: parts.slice(1).flatMap(p => p.split('|'))
            };
        }

        return {
            type: cleaned,
            optional
        };
    }

    /**
     * 获取命令的所有重载
     */
    public getCommandSpecs(commandName: string): CommandSpec[] | undefined {
        return this.specs.get(commandName);
    }

    /**
     * 检查命令是否存在
     */
    public hasCommand(commandName: string): boolean {
        return this.allCommandNames.has(commandName);
    }

    /**
     * 获取所有命令名
     */
    public getAllCommandNames(): string[] {
        return Array.from(this.allCommandNames);
    }

    /**
     * 查找最匹配的命令重载
     */
    public findBestMatch(commandName: string, argCount: number): CommandSpec | undefined {
        const specs = this.getCommandSpecs(commandName);
        if (!specs) return undefined;

        // 优先选择参数数量匹配的
        const exactMatch = specs.find(spec => spec.parameters.length === argCount);
        if (exactMatch) return exactMatch;

        // 如果没有精确匹配，返回第一个
        return specs[0];
    }

    /**
     * 获取命令的描述
     */
    public getCommandDescription(commandName: string): string | undefined {
        const specs = this.getCommandSpecs(commandName);
        return specs?.[0]?.description;
    }

    /**
     * 获取命令总数
     */
    public getCommandCount(): number {
        return this.allCommandNames.size;
    }
}

// 导出单例
export const specRegistry = new SpecRegistry();
