import { Position } from './ast.js';

// --- Token 类型定义 ---
export enum TokenType {
    // 字面量
    IDENTIFIER = 'IDENTIFIER',
    NUMBER = 'NUMBER',
    STRING = 'STRING',
    BOOLEAN = 'BOOLEAN',
    
    // 符号
    LPAREN = 'LPAREN',          // (
    RPAREN = 'RPAREN',          // )
    LBRACE = 'LBRACE',          // {
    RBRACE = 'RBRACE',          // }
    COMMA = 'COMMA',            // ,
    EQUALS = 'EQUALS',          // =
    SEMICOLON = 'SEMICOLON',    // ;
    NEWLINE = 'NEWLINE',        // \n
    
    // 特殊
    EOF = 'EOF',
    UNKNOWN = 'UNKNOWN'
}

export interface Token {
    type: TokenType;
    value: string;
    position: Position;
}

// --- Lexer 类 ---
export class Lexer {
    private input: string;
    private position: number = 0;
    private line: number = 1;
    private column: number = 1;
    
    constructor(input: string) {
        this.input = input;
    }
    
    /**
     * 获取所有 token
     */
    public tokenize(): Token[] {
        const tokens: Token[] = [];
        let token: Token;
        
        while ((token = this.nextToken()).type !== TokenType.EOF) {
            // 跳过空白行的 NEWLINE token，保留有意义的换行
            if (token.type === TokenType.NEWLINE) {
                // 可选：保留换行符用于分隔命令
                tokens.push(token);
            } else if (token.type !== TokenType.UNKNOWN) {
                tokens.push(token);
            }
        }
        
        tokens.push(token); // 添加 EOF token
        return tokens;
    }
    
    /**
     * 获取下一个 token
     */
    private nextToken(): Token {
        this.skipWhitespace();
        
        if (this.isAtEnd()) {
            return this.createToken(TokenType.EOF, '');
        }
        
        const char = this.peek();
        const position = this.getCurrentPosition();
        
        // 注释 (// 开头)
        if (char === '/' && this.peekNext() === '/') {
            this.skipComment();
            return this.nextToken();
        }
        
        // 字符串 (双引号或单引号)
        if (char === '"' || char === "'") {
            return this.scanString();
        }
        
        // 数字
        if (this.isDigit(char) || (char === '-' && this.isDigit(this.peekNext()))) {
            return this.scanNumber();
        }
        
        // 标识符或布尔值
        if (this.isAlpha(char) || char === '_') {
            return this.scanIdentifier();
        }
        
        // 符号
        switch (char) {
            case '(':
                this.advance();
                return this.createToken(TokenType.LPAREN, '(', position);
            case ')':
                this.advance();
                return this.createToken(TokenType.RPAREN, ')', position);
            case '{':
                this.advance();
                return this.createToken(TokenType.LBRACE, '{', position);
            case '}':
                this.advance();
                return this.createToken(TokenType.RBRACE, '}', position);
            case ',':
                this.advance();
                return this.createToken(TokenType.COMMA, ',', position);
            case '=':
                this.advance();
                return this.createToken(TokenType.EQUALS, '=', position);
            case ';':
                this.advance();
                return this.createToken(TokenType.SEMICOLON, ';', position);
            case '\n':
                this.advance();
                this.line++;
                this.column = 1;
                return this.createToken(TokenType.NEWLINE, '\n', position);
            default:
                this.advance();
                return this.createToken(TokenType.UNKNOWN, char, position);
        }
    }
    
    /**
     * 扫描字符串字面量
     */
    private scanString(): Token {
        const position = this.getCurrentPosition();
        const quote = this.peek();
        this.advance(); // 跳过起始引号
        
        let value = '';
        while (!this.isAtEnd() && this.peek() !== quote) {
            if (this.peek() === '\\') {
                this.advance();
                // 处理转义字符
                if (!this.isAtEnd()) {
                    const escaped = this.peek();
                    switch (escaped) {
                        case 'n':
                            value += '\n';
                            break;
                        case 't':
                            value += '\t';
                            break;
                        case '\\':
                            value += '\\';
                            break;
                        case '"':
                            value += '"';
                            break;
                        case "'":
                            value += "'";
                            break;
                        default:
                            value += escaped;
                    }
                    this.advance();
                }
            } else {
                value += this.peek();
                this.advance();
            }
        }
        
        if (!this.isAtEnd()) {
            this.advance(); // 跳过结束引号
        }
        
        return this.createToken(TokenType.STRING, value, position);
    }
    
    /**
     * 扫描数字字面量
     */
    private scanNumber(): Token {
        const position = this.getCurrentPosition();
        let value = '';
        
        // 负号
        if (this.peek() === '-') {
            value += this.peek();
            this.advance();
        }
        
        // 整数部分
        while (!this.isAtEnd() && this.isDigit(this.peek())) {
            value += this.peek();
            this.advance();
        }
        
        // 小数部分
        if (!this.isAtEnd() && this.peek() === '.' && this.isDigit(this.peekNext())) {
            value += this.peek();
            this.advance();
            
            while (!this.isAtEnd() && this.isDigit(this.peek())) {
                value += this.peek();
                this.advance();
            }
        }
        
        // 科学计数法
        if (!this.isAtEnd() && (this.peek() === 'e' || this.peek() === 'E')) {
            value += this.peek();
            this.advance();
            
            if (!this.isAtEnd() && (this.peek() === '+' || this.peek() === '-')) {
                value += this.peek();
                this.advance();
            }
            
            while (!this.isAtEnd() && this.isDigit(this.peek())) {
                value += this.peek();
                this.advance();
            }
        }
        
        return this.createToken(TokenType.NUMBER, value, position);
    }
    
    /**
     * 扫描标识符或布尔值
     */
    private scanIdentifier(): Token {
        const position = this.getCurrentPosition();
        let value = '';
        
        while (!this.isAtEnd() && (this.isAlphaNumeric(this.peek()) || this.peek() === '_')) {
            value += this.peek();
            this.advance();
        }
        
        // 检查是否为布尔值
        if (value === 'true' || value === 'false') {
            return this.createToken(TokenType.BOOLEAN, value, position);
        }
        
        return this.createToken(TokenType.IDENTIFIER, value, position);
    }
    
    /**
     * 跳过空白字符 (不包括换行符)
     */
    private skipWhitespace(): void {
        while (!this.isAtEnd()) {
            const char = this.peek();
            if (char === ' ' || char === '\t' || char === '\r') {
                this.advance();
            } else {
                break;
            }
        }
    }
    
    /**
     * 跳过注释
     */
    private skipComment(): void {
        while (!this.isAtEnd() && this.peek() !== '\n') {
            this.advance();
        }
    }
    
    /**
     * 向前移动一个字符
     */
    private advance(): void {
        if (!this.isAtEnd()) {
            this.position++;
            this.column++;
        }
    }
    
    /**
     * 查看当前字符
     */
    private peek(): string {
        return this.isAtEnd() ? '\0' : this.input[this.position];
    }
    
    /**
     * 查看下一个字符
     */
    private peekNext(): string {
        return this.position + 1 >= this.input.length ? '\0' : this.input[this.position + 1];
    }
    
    /**
     * 是否到达末尾
     */
    private isAtEnd(): boolean {
        return this.position >= this.input.length;
    }
    
    /**
     * 判断是否为字母
     */
    private isAlpha(char: string): boolean {
        return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z');
    }
    
    /**
     * 判断是否为数字
     */
    private isDigit(char: string): boolean {
        return char >= '0' && char <= '9';
    }
    
    /**
     * 判断是否为字母或数字
     */
    private isAlphaNumeric(char: string): boolean {
        return this.isAlpha(char) || this.isDigit(char);
    }
    
    /**
     * 获取当前位置
     */
    private getCurrentPosition(): Position {
        return { line: this.line, column: this.column };
    }
    
    /**
     * 创建 token
     */
    private createToken(type: TokenType, value: string, position?: Position): Token {
        return {
            type,
            value,
            position: position || this.getCurrentPosition()
        };
    }
}
