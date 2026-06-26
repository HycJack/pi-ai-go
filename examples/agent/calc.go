package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// calc 支持的函数（变量名 → 参数个数）
var calcFuncs = map[string]int{
	"sqrt":  1,
	"sin":   1,
	"cos":   1,
	"tan":   1,
	"log":   1, // log10
	"ln":    1, // 自然对数
	"abs":   1,
	"floor": 1,
	"ceil":  1,
	"round": 1,
	"pow":   2, // pow(x, y) = x^y
}

// evaluateExpression 求值数学表达式，支持：
//   - 四则运算 + - * /
//   - 括号 ()
//   - 一元负号 -
//   - 函数 sqrt/sin/cos/tan/log/ln/abs/floor/ceil/round/pow
//   - 数字字面量
//
// sin/cos/tan 接收角度（degrees）。
func evaluateExpression(expr string) (float64, error) {
	p := &parser{input: strings.ReplaceAll(expr, " ", "")}
	p.next()
	v, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	if p.tok.kind != tEOF {
		return 0, fmt.Errorf("表达式末尾有多余字符: %q", p.tok.lit)
	}
	return v, nil
}

// ---- tokenizer ----

type tokenKind int

const (
	tEOF tokenKind = iota
	tNum
	tIdent
	tPlus
	tMinus
	tStar
	tSlash
	tCaret
	tLParen
	tRParen
	tComma
)

type token struct {
	kind tokenKind
	lit  string
	num  float64
}

type parser struct {
	input string
	pos   int
	tok   token
}

func (p *parser) next() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
	if p.pos >= len(p.input) {
		p.tok = token{kind: tEOF}
		return
	}
	c := p.input[p.pos]

	// 数字
	if c >= '0' && c <= '9' || c == '.' {
		start := p.pos
		dot := false
		for p.pos < len(p.input) {
			ch := p.input[p.pos]
			if ch >= '0' && ch <= '9' {
				p.pos++
			} else if ch == '.' && !dot {
				dot = true
				p.pos++
			} else {
				break
			}
		}
		v, err := strconv.ParseFloat(p.input[start:p.pos], 64)
		if err != nil {
			p.tok = token{kind: tNum, lit: p.input[start:p.pos]}
		} else {
			p.tok = token{kind: tNum, lit: p.input[start:p.pos], num: v}
		}
		return
	}

	// 标识符（函数名）
	if unicode.IsLetter(rune(c)) {
		start := p.pos
		for p.pos < len(p.input) && (unicode.IsLetter(rune(p.input[p.pos])) || unicode.IsDigit(rune(p.input[p.pos]))) {
			p.pos++
		}
		p.tok = token{kind: tIdent, lit: strings.ToLower(p.input[start:p.pos])}
		return
	}

	p.pos++
	switch c {
	case '+':
		p.tok = token{kind: tPlus, lit: "+"}
	case '-':
		p.tok = token{kind: tMinus, lit: "-"}
	case '*':
		p.tok = token{kind: tStar, lit: "*"}
	case '/':
		p.tok = token{kind: tSlash, lit: "/"}
	case '^':
		p.tok = token{kind: tCaret, lit: "^"}
	case '(':
		p.tok = token{kind: tLParen, lit: "("}
	case ')':
		p.tok = token{kind: tRParen, lit: ")"}
	case ',':
		p.tok = token{kind: tComma, lit: ","}
	default:
		p.tok = token{kind: tEOF, lit: string(c)}
	}
}

// ---- 递归下降 ----

// expr = term (('+'|'-') term)*
func (p *parser) parseExpr() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for p.tok.kind == tPlus || p.tok.kind == tMinus {
		op := p.tok.kind
		p.next()
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if op == tPlus {
			left += right
		} else {
			left -= right
		}
	}
	return left, nil
}

// term = factor (('*'|'/') factor)*
func (p *parser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for p.tok.kind == tStar || p.tok.kind == tSlash {
		op := p.tok.kind
		p.next()
		right, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		if op == tStar {
			left *= right
		} else {
			if right == 0 {
				return 0, fmt.Errorf("除数不能为 0")
			}
			left /= right
		}
	}
	return left, nil
}

// factor = unary ('^' factor)?     // 右结合
func (p *parser) parseFactor() (float64, error) {
	left, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	if p.tok.kind == tCaret {
		p.next()
		right, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		left = math.Pow(left, right)
	}
	return left, nil
}

// unary = '-' unary | primary
func (p *parser) parseUnary() (float64, error) {
	if p.tok.kind == tMinus {
		p.next()
		v, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		return -v, nil
	}
	if p.tok.kind == tPlus {
		p.next()
		return p.parseUnary()
	}
	return p.parsePrimary()
}

// primary = number | ident '(' args ')' | '(' expr ')'
func (p *parser) parsePrimary() (float64, error) {
	switch p.tok.kind {
	case tNum:
		v := p.tok.num
		p.next()
		return v, nil
	case tLParen:
		p.next()
		v, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if p.tok.kind != tRParen {
			return 0, fmt.Errorf("缺少右括号 ')'")
		}
		p.next()
		return v, nil
	case tIdent:
		name := p.tok.lit
		p.next()
		if p.tok.kind != tLParen {
			return 0, fmt.Errorf("未知标识符 %q", name)
		}
		p.next()
		args, err := p.parseArgs()
		if err != nil {
			return 0, err
		}
		if p.tok.kind != tRParen {
			return 0, fmt.Errorf("函数 %s 缺少右括号", name)
		}
		p.next()
		return callFunc(name, args)
	}
	return 0, fmt.Errorf("意外字符: %q", p.tok.lit)
}

// args = expr (',' expr)*
func (p *parser) parseArgs() ([]float64, error) {
	var args []float64
	if p.tok.kind == tRParen {
		return args, nil
	}
	v, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	args = append(args, v)
	for p.tok.kind == tComma {
		p.next()
		v, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, v)
	}
	return args, nil
}

func callFunc(name string, args []float64) (float64, error) {
	want, ok := calcFuncs[name]
	if !ok {
		return 0, fmt.Errorf("未知函数: %s", name)
	}
	if len(args) != want {
		return 0, fmt.Errorf("函数 %s 需要 %d 个参数，得到 %d 个", name, want, len(args))
	}
	switch name {
	case "sqrt":
		if args[0] < 0 {
			return 0, fmt.Errorf("sqrt 参数不能为负")
		}
		return math.Sqrt(args[0]), nil
	case "sin":
		return math.Sin(args[0] * math.Pi / 180), nil
	case "cos":
		return math.Cos(args[0] * math.Pi / 180), nil
	case "tan":
		rad := args[0] * math.Pi / 180
		if math.Abs(math.Cos(rad)) < 1e-12 {
			return 0, fmt.Errorf("tan(%.4g) 未定义", args[0])
		}
		return math.Tan(rad), nil
	case "log":
		if args[0] <= 0 {
			return 0, fmt.Errorf("log 参数必须大于 0")
		}
		return math.Log10(args[0]), nil
	case "ln":
		if args[0] <= 0 {
			return 0, fmt.Errorf("ln 参数必须大于 0")
		}
		return math.Log(args[0]), nil
	case "abs":
		return math.Abs(args[0]), nil
	case "floor":
		return math.Floor(args[0]), nil
	case "ceil":
		return math.Ceil(args[0]), nil
	case "round":
		return math.Round(args[0]), nil
	case "pow":
		return math.Pow(args[0], args[1]), nil
	}
	return 0, fmt.Errorf("内部错误：未实现的函数 %s", name)
}
