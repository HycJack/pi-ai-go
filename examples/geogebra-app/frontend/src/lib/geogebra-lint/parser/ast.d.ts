// --- 核心位置信息类型 ---
/**
 * 源代码中的精确位置信息
 */
export interface Position {
    line: number;   // 从 1 开始
    column: number; // 从 1 开始
}

/**
 * 源代码片段的范围
 */
export interface SourceLocation {
    start: Position;
    end: Position;
}

// --- AST 节点基类型 ---
/**
 * 所有 AST 节点的共同基类型
 */
export interface BaseNode {
    type: string; // 节点的类型，用于识别
    loc: SourceLocation; // 节点在源代码中的位置
}

// --- 根节点 ---
/**
 * 程序的根节点，代表整个 GeoGebraScript
 */
export interface Program extends BaseNode {
    type: 'Program';
    body: CommandStatement[]; // 脚本中的一系列命令语句
}

// --- 语句节点 ---
/**
 * 命令调用语句节点 (例如: SetValue(a, 1) )
 */
export interface CommandStatement extends BaseNode {
    type: 'CommandStatement';
    commandName: Identifier;    // 命令名，例如 'SetValue'
    arguments: Expression[];    // 命令的参数列表
    // 假设 GeoGebraScript 允许赋值：
    // assignee?: Identifier;   // 如果是赋值操作，例如 'P = Point(1, 2)'
}

// --- 表达式节点 (作为参数) ---
/**
 * 所有可用作命令参数的表达式基类型
 */
export type Expression = 
    | Identifier
    | NumberLiteral
    | StringLiteral
    | BooleanLiteral
    | ListLiteral
    | TupleLiteral
    | FunctionCall; // 嵌套命令调用 (例如: SetValue(A, x(B)))

/**
 * 标识符 (变量名，如 'A', 'slider1', 'f')
 */
export interface Identifier extends BaseNode {
    type: 'Identifier';
    name: string;
}

/**
 * 数字字面量 (例如: 1, 0.5, -3)
 */
export interface NumberLiteral extends BaseNode {
    type: 'NumberLiteral';
    value: number;
}

/**
 * 字符串字面量 (例如: "red", "Hello")
 */
export interface StringLiteral extends BaseNode {
    type: 'StringLiteral';
    value: string;
}

/**
 * 布尔字面量 (例如: true, false)
 */
export interface BooleanLiteral extends BaseNode {
    type: 'BooleanLiteral';
    value: boolean;
}

/**
 * 列表字面量 (例如: {1, 2, 3} )
 */
export interface ListLiteral extends BaseNode {
    type: 'ListLiteral';
    elements: Expression[]; // 列表中的元素也是表达式
}

/**
 * 元组/坐标字面量 (例如: (0, 0, 3) )
 * 用于表示点的坐标或其他元组值
 */
export interface TupleLiteral extends BaseNode {
    type: 'TupleLiteral';
    elements: Expression[]; // 元组中的元素也是表达式
}

/**
 * 函数/嵌套命令调用表达式 (例如: x(A) 中的 x(...) 部分)
 */
export interface FunctionCall extends BaseNode {
    type: 'FunctionCall';
    callee: Identifier;    // 被调用的函数或命令名
    arguments: Expression[];
}

// --- 组合类型 ---
export type ASTNode = Program | CommandStatement | Expression;