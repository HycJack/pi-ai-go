import {
    Program,
    CommandStatement,
    Expression,
    Identifier,
    NumberLiteral,
    StringLiteral,
    BooleanLiteral,
    ListLiteral,
    TupleLiteral,
    SourceLocation,
    Position
} from './ast.js';
import { Token, TokenType, Lexer } from './lexer.js';

/**
 * Parser 错误类
 */
export class ParseError extends Error {
    constructor(
        message: string,
        public position: Position
    ) {
        super(`Parse error at line ${position.line}, column ${position.column}: ${message}`);
        this.name = 'ParseError';
    }
}

/**
 * GeoGebra 脚本解析器
 */
export class Parser {
    private tokens: Token[] = [];
    private current: number = 0;
    
    /**
     * 解析源代码为 AST
     */
    public parse(input: string): Program {
        const lexer = new Lexer(input);
        this.tokens = lexer.tokenize();
        this.current = 0;
        
        return this.parseProgram();
    }
    
    /**
     * 解析程序 (根节点)
     */
    private parseProgram(): Program {
        const startPos = this.getCurrentToken().position;
        const body: CommandStatement[] = [];
        
        while (!this.isAtEnd()) {
            // 跳过换行符和分号
            if (this.match(TokenType.NEWLINE, TokenType.SEMICOLON)) {
                continue;
            }
            
            if (this.check(TokenType.EOF)) {
                break;
            }
            
            try {
                const statement = this.parseCommandStatement();
                body.push(statement);
            } catch (error) {
                // 错误恢复：跳到下一行
                this.synchronize();
                throw error;
            }
        }
        
        const endPos = this.previous().position;
        
        return {
            type: 'Program',
            body,
            loc: this.createLocation(startPos, endPos)
        };
    }
    
    /**
     * 解析命令语句
     * 支持格式：
     * 1. CommandName(arg1, arg2, ...)
     * 2. var = CommandName(arg1, arg2, ...)
     * 3. var = Expression (例如: A = {1, 2, 3})
     */
    private parseCommandStatement(): CommandStatement {
        const startPos = this.getCurrentToken().position;
        
        // 解析命令名或变量名
        const firstIdentifier = this.parseIdentifier();
        
        // 检查是否为赋值语句 (var = Command(...) 或 var = Expression)
        if (this.match(TokenType.EQUALS)) {
            // 这是赋值语句，firstIdentifier 是被赋值的变量
            // 检查右侧是什么类型的表达式
            
            // 如果右侧是标识符且后面跟着左括号，则是命令调用
            if (this.check(TokenType.IDENTIFIER)) {
                // 保存当前位置
                const savedPosition = this.current;
                this.advance(); // 跳过标识符
                const isCommandCall = this.check(TokenType.LPAREN);
                this.current = savedPosition; // 恢复位置
                
                if (isCommandCall) {
                    // var = CommandName(...)
                    const commandName = this.parseIdentifier();
                    this.consume(TokenType.LPAREN, "Expected '(' after command name");
                    
                    const args = this.parseArgumentList();
                    
                    this.consume(TokenType.RPAREN, "Expected ')' after arguments");
                    
                    const endPos = this.previous().position;
                    
                    return {
                        type: 'CommandStatement',
                        commandName,
                        arguments: args,
                        loc: this.createLocation(startPos, endPos)
                    };
                }
            }
            
            // 否则，右侧是普通表达式 (如列表字面量、数字等)
            // 将其作为特殊命令处理，命令名为变量名，表达式作为参数
            const expression = this.parseExpression();
            const endPos = this.previous().position;
            
            return {
                type: 'CommandStatement',
                commandName: firstIdentifier,
                arguments: [expression],
                loc: this.createLocation(startPos, endPos)
            };
        } else {
            // 这是普通命令调用
            this.consume(TokenType.LPAREN, "Expected '(' after command name");
            
            const args = this.parseArgumentList();
            
            this.consume(TokenType.RPAREN, "Expected ')' after arguments");
            
            const endPos = this.previous().position;
            
            return {
                type: 'CommandStatement',
                commandName: firstIdentifier,
                arguments: args,
                loc: this.createLocation(startPos, endPos)
            };
        }
    }
    
    /**
     * 解析参数列表
     */
    private parseArgumentList(): Expression[] {
        const args: Expression[] = [];
        
        // 空参数列表
        if (this.check(TokenType.RPAREN)) {
            return args;
        }
        
        // 解析第一个参数
        args.push(this.parseExpression());
        
        // 解析剩余参数
        while (this.match(TokenType.COMMA)) {
            args.push(this.parseExpression());
        }
        
        return args;
    }
    
    /**
     * 解析表达式
     */
    private parseExpression(): Expression {
        const token = this.getCurrentToken();
        
        switch (token.type) {
            case TokenType.NUMBER:
                return this.parseNumberLiteral();
            case TokenType.STRING:
                return this.parseStringLiteral();
            case TokenType.BOOLEAN:
                return this.parseBooleanLiteral();
            case TokenType.LBRACE:
                return this.parseListLiteral();
            case TokenType.LPAREN:
                return this.parseTupleLiteralOrGroupedExpression();
            case TokenType.IDENTIFIER:
                return this.parseIdentifierOrFunctionCall();
            default:
                throw new ParseError(
                    `Unexpected token: ${token.type} (value: '${token.value}')`,
                    token.position
                );
        }
    }
    
    /**
     * 解析标识符或函数调用
     */
    private parseIdentifierOrFunctionCall(): Expression {
        const identifier = this.parseIdentifier();
        
        // 检查是否为函数调用
        if (this.check(TokenType.LPAREN)) {
            this.advance(); // 消费 '('
            
            const args = this.parseArgumentList();
            
            this.consume(TokenType.RPAREN, "Expected ')' after function arguments");
            
            const endPos = this.previous().position;
            
            return {
                type: 'FunctionCall',
                callee: identifier,
                arguments: args,
                loc: this.createLocation(identifier.loc.start, endPos)
            };
        }
        
        // 否则就是普通标识符
        return identifier;
    }
    
    /**
     * 解析标识符
     */
    private parseIdentifier(): Identifier {
        const token = this.consume(TokenType.IDENTIFIER, 'Expected identifier');
        
        return {
            type: 'Identifier',
            name: token.value,
            loc: this.createLocation(token.position, token.position)
        };
    }
    
    /**
     * 解析数字字面量
     */
    private parseNumberLiteral(): NumberLiteral {
        const token = this.consume(TokenType.NUMBER, 'Expected number');
        
        return {
            type: 'NumberLiteral',
            value: parseFloat(token.value),
            loc: this.createLocation(token.position, token.position)
        };
    }
    
    /**
     * 解析字符串字面量
     */
    private parseStringLiteral(): StringLiteral {
        const token = this.consume(TokenType.STRING, 'Expected string');
        
        return {
            type: 'StringLiteral',
            value: token.value,
            loc: this.createLocation(token.position, token.position)
        };
    }
    
    /**
     * 解析布尔字面量
     */
    private parseBooleanLiteral(): BooleanLiteral {
        const token = this.consume(TokenType.BOOLEAN, 'Expected boolean');
        
        return {
            type: 'BooleanLiteral',
            value: token.value === 'true',
            loc: this.createLocation(token.position, token.position)
        };
    }
    
    /**
     * 解析列表字面量
     * 格式: {elem1, elem2, ...}
     */
    private parseListLiteral(): ListLiteral {
        const startPos = this.getCurrentToken().position;
        
        this.consume(TokenType.LBRACE, "Expected '{'");
        
        const elements: Expression[] = [];
        
        // 空列表
        if (!this.check(TokenType.RBRACE)) {
            elements.push(this.parseExpression());
            
            while (this.match(TokenType.COMMA)) {
                elements.push(this.parseExpression());
            }
        }
        
        this.consume(TokenType.RBRACE, "Expected '}'");
        
        const endPos = this.previous().position;
        
        return {
            type: 'ListLiteral',
            elements,
            loc: this.createLocation(startPos, endPos)
        };
    }
    
    /**
     * 解析元组字面量或分组表达式
     * 格式: (elem1, elem2, ...) 或 (expr)
     * 
     * 如果圆括号内只有一个元素，返回该元素本身（分组表达式）
     * 如果有多个元素（用逗号分隔），返回 TupleLiteral
     */
    private parseTupleLiteralOrGroupedExpression(): Expression {
        const startPos = this.getCurrentToken().position;
        
        this.consume(TokenType.LPAREN, "Expected '('");
        
        const elements: Expression[] = [];
        
        // 空元组 - 虽然不太常见，但支持
        if (this.check(TokenType.RPAREN)) {
            this.consume(TokenType.RPAREN, "Expected ')'");
            const endPos = this.previous().position;
            
            return {
                type: 'TupleLiteral',
                elements: [],
                loc: this.createLocation(startPos, endPos)
            };
        }
        
        // 解析第一个元素
        elements.push(this.parseExpression());
        
        // 检查是否有更多元素（逗号分隔）
        if (this.check(TokenType.COMMA)) {
            // 这是一个元组
            while (this.match(TokenType.COMMA)) {
                // 允许尾随逗号，如 (1, 2, 3,)
                if (this.check(TokenType.RPAREN)) {
                    break;
                }
                elements.push(this.parseExpression());
            }
            
            this.consume(TokenType.RPAREN, "Expected ')'");
            const endPos = this.previous().position;
            
            return {
                type: 'TupleLiteral',
                elements,
                loc: this.createLocation(startPos, endPos)
            };
        }
        
        // 只有一个元素，这是分组表达式，直接返回该元素
        this.consume(TokenType.RPAREN, "Expected ')'");
        
        return elements[0];
    }
    
    /**
     * 匹配并消费 token
     */
    private match(...types: TokenType[]): boolean {
        for (const type of types) {
            if (this.check(type)) {
                this.advance();
                return true;
            }
        }
        return false;
    }
    
    /**
     * 检查当前 token 类型
     */
    private check(type: TokenType): boolean {
        if (this.isAtEnd()) {
            return false;
        }
        return this.getCurrentToken().type === type;
    }
    
    /**
     * 消费特定类型的 token
     */
    private consume(type: TokenType, message: string): Token {
        if (this.check(type)) {
            return this.advance();
        }
        
        const token = this.getCurrentToken();
        throw new ParseError(message, token.position);
    }
    
    /**
     * 前进到下一个 token
     */
    private advance(): Token {
        if (!this.isAtEnd()) {
            this.current++;
        }
        return this.previous();
    }
    
    /**
     * 是否到达末尾
     */
    private isAtEnd(): boolean {
        return this.getCurrentToken().type === TokenType.EOF;
    }
    
    /**
     * 获取当前 token
     */
    private getCurrentToken(): Token {
        return this.tokens[this.current];
    }
    
    /**
     * 获取前一个 token
     */
    private previous(): Token {
        return this.tokens[this.current - 1];
    }
    
    /**
     * 创建源位置信息
     */
    private createLocation(start: Position, end: Position): SourceLocation {
        return { start, end };
    }
    
    /**
     * 错误恢复：跳到下一个语句
     */
    private synchronize(): void {
        this.advance();
        
        while (!this.isAtEnd()) {
            if (this.previous().type === TokenType.NEWLINE || 
                this.previous().type === TokenType.SEMICOLON) {
                return;
            }
            
            this.advance();
        }
    }
}

/**
 * 便捷函数：解析 GeoGebra 脚本
 */
export function parseGeoGebraScript(source: string): Program {
    const parser = new Parser();
    return parser.parse(source);
}
