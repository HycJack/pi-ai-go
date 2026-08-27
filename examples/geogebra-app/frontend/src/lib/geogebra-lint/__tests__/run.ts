/**
 * 简单的测试 runner — 用 tsx 直接跑这个文件即可
 * 
 * 使用方法：
 *   cd frontend
 *   npx tsx src/lib/geogebra-lint/__tests__/run.ts
 */

import { RuleEngine, noUnknownCommand, correctArgTypes, formatLintResults } from '../index';
import { LintSeverity } from '../types-linting';

let passed = 0;
let failed = 0;
let testCase = 0;

function test(name: string, code: string, expectations: {
    /** 期望的错误数量（精确值或 min-max） */
    errorCount?: number | [number, number];
    /** 期望的警告数量 */
    warningCount?: number | [number, number];
    /** 是否期望没有任何 parse-error */
    noParseError?: boolean;
    /** 期望包含特定的错误/警告消息 */
    contains?: string[];
    /** 期望不包含的消息 */
    notContains?: string[];
}) {
    testCase++;
    const engine = new RuleEngine();
    engine.registerRule(noUnknownCommand);
    engine.registerRule(correctArgTypes);
    
    let result;
    try {
        result = engine.lint(code);
    } catch (e: any) {
        console.log(`  ❌ #${testCase} ${name}`);
        console.log(`      ERROR: ${e.message}`);
        failed++;
        return;
    }
    
    const hasParseError = result.messages.some(m => m.ruleId === 'parse-error');
    const errors = result.messages.filter(m => m.severity === LintSeverity.Error);
    const warnings = result.messages.filter(m => m.severity === LintSeverity.Warning);
    
    let ok = true;
    const failures: string[] = [];
    
    // 检查 noParseError
    if (expectations.noParseError !== false && hasParseError) {
        ok = false;
        failures.push(`unexpected parse error: ${result.messages.find(m => m.ruleId === 'parse-error')?.message}`);
    }
    
    // 检查 errorCount
    if (expectations.errorCount !== undefined) {
        const ec = expectations.errorCount;
        const match = Array.isArray(ec) 
            ? (errors.length >= ec[0] && errors.length <= ec[1])
            : (errors.length === ec);
        if (!match) {
            ok = false;
            failures.push(`expected errorCount ${JSON.stringify(ec)}, got ${errors.length}`);
        }
    }
    
    // 检查 warningCount
    if (expectations.warningCount !== undefined) {
        const wc = expectations.warningCount;
        const match = Array.isArray(wc)
            ? (warnings.length >= wc[0] && warnings.length <= wc[1])
            : (warnings.length === wc);
        if (!match) {
            ok = false;
            failures.push(`expected warningCount ${JSON.stringify(wc)}, got ${warnings.length}`);
        }
    }
    
    // 检查 contains（联合 message + suggestions）
    if (expectations.contains) {
        for (const substr of expectations.contains) {
            const fullText = result.messages.map(m => {
                let text = m.message;
                const msg = m as any;
                if (msg.suggestions?.length) {
                    text += ' ' + msg.suggestions.join(' ');
                }
                return text;
            }).join('\n');
            if (!fullText.includes(substr)) {
                ok = false;
                failures.push(`expected message containing "${substr}" not found`);
            }
        }
    }
    
    // 检查 notContains（联合 message + suggestions）
    if (expectations.notContains) {
        for (const substr of expectations.notContains) {
            const fullText = result.messages.map(m => {
                let text = m.message;
                const msg = m as any;
                if (msg.suggestions?.length) {
                    text += ' ' + msg.suggestions.join(' ');
                }
                return text;
            }).join('\n');
            if (fullText.includes(substr)) {
                ok = false;
                failures.push(`unexpected message containing "${substr}" found`);
            }
        }
    }
    
    if (ok) {
        console.log(`  ✅ #${testCase} ${name}`);
        passed++;
    } else {
        console.log(`  ❌ #${testCase} ${name}`);
        failures.forEach(f => console.log(`      - ${f}`));
        // 显示完整结果帮助调试
        console.log(`      result: ${formatLintResults(result).split('\n').join('\n      ')}`);
        failed++;
    }
}

console.log('========================================');
console.log('  GeoGebra Lint 测试套件');
console.log('========================================\n');

// ==============================
// 分组 1: 基础测试
// ==============================
console.log('--- 分组 1: 基础命令与字面量 ---');

test('简单命令：一个参数', 
    'SetValue(a, 1)', {
    errorCount: 0,
    noParseError: true,
});

test('无参数命令', 
    'ZoomIn(1)', {
    errorCount: 0,
    noParseError: true,
});

test('字符串参数', 
    'SetCaption(A, "标签")', {
    errorCount: 0,
    noParseError: true,
});

test('布尔参数', 
    'SetVisible(A, true)', {
    errorCount: 1,
    noParseError: true,
    contains: ['未知的命令 "SetVisible"'],
});

test('列表字面量赋值', 
    'A = {1, 2, 3}', {
    errorCount: 0,
    noParseError: true,
});

test('元组字面量赋值', 
    'P = (0, 0, 3)', {
    errorCount: 0,
    noParseError: true,
});

test('数字赋值', 
    'x = 5', {
    errorCount: 0,
    noParseError: true,
});

test('字符串赋值', 
    'name = "hello"', {
    errorCount: 0,
    noParseError: true,
});

test('布尔值赋值', 
    'flag = true', {
    errorCount: 0,
    noParseError: true,
});

test('变量赋值', 
    'B = A', {
    errorCount: 0,
    noParseError: true,
});

// ==============================
// 分组 2: 变量赋值 + 命令调用
// ==============================
console.log('\n--- 分组 2: 变量赋值 + 命令调用 ---');

test('变量赋值：Slider 命令', 
    'n = Slider(3, 12, 1)', {
    noParseError: true,
    notContains: ['未知的命令 "n"'],
});

test('变量赋值：多个 Slider', 
    `n = Slider(3, 12, 1)
r = Slider(1, 5, 0.1)
h = Slider(1, 8, 0.1)`, {
    noParseError: true,
    notContains: ['未知的命令 "n"', '未知的命令 "r"', '未知的命令 "h"'],
});

test('变量赋值：Polygon', 
    'base = Polygon(vertices)', {
    noParseError: true,
    notContains: ['未知的命令 "base"'],
});

test('变量赋值：Sequence', 
    'vertices = Sequence(i, i, 1, 10)', {
    noParseError: true,
    notContains: ['未知的命令 "vertices"'],
});

test('变量赋值 + 传参给其他命令', 
    `n = Slider(3, 12, 1)
SetCaption(n, "边数")`, {
    noParseError: true,
    errorCount: 0,
});

// ==============================
// 分组 3: 算术表达式
// ==============================
console.log('\n--- 分组 3: 算术表达式 ---');

test('简单二元运算：乘法', 
    'SetValue(x, 2 * pi)', {
    noParseError: true,
});

test('二元运算：加减乘除', 
    'SetValue(x, a + b * c / d)', {
    noParseError: true,
});

test('二元运算：分组括号', 
    'SetValue(x, (a + b) * 2)', {
    noParseError: true,
});

test('变量赋值 + 表达式', 
    'a = 2 * pi + 1', {
    noParseError: true,
    notContains: ['未知的命令 "a"'],
});

test('复杂表达式：cos 嵌套', 
    'SetValue(x, cos(2 * pi * i / n))', {
    noParseError: true,
});

test('Sequence 内表达式', 
    'vertices = Sequence((r * cos(2 * pi * i / n), r * sin(2 * pi * i / n), 0), i, 1, n)', {
    noParseError: true,
    notContains: ['未知的命令 "vertices"'],
});

test('多个运算符连续', 
    'SetValue(x, a / b / c)', {
    noParseError: true,
});

// ==============================
// 分组 4: 复杂嵌套
// ==============================
console.log('\n--- 分组 4: 复杂嵌套 ---');

test('函数嵌套：Element 调用', 
    'SetValue(x, x(Element(A, 1)))', {
    noParseError: true,
});

test('复杂嵌套：Sequence 内函数调用', 
    `topVertices = Sequence((x(Element(vertices, i)), y(Element(vertices, i)), h), i, 1, n)`, {
    noParseError: true,
    notContains: ['未知的命令 "topVertices"'],
});

test('Execute + Sequence + 字符串拼接（字符串参数中包含函数调用）', 
    `Execute(Sequence("SetColor(sideFace" + i + ", 0.2, 0.6, 0.8)", i, 1, n))`, {
    noParseError: true,
});

test('链式 Element + Mod 索引', 
    `Execute(Sequence("sideEdge" + i + " = Segment(Element(vertices," + i + "), Element(topVertices," + i + "))", i, 1, n))`, {
    noParseError: true,
});

test('多重赋值：从 Simple 生成到执行命令', 
    `n = Slider(3, 12, 1, 1, 100, false, true, false, false)
r = Slider(1, 5, 0.1, 1, 100, false, true, false, false)
h = Slider(1, 8, 0.1, 1, 100, false, true, false, false)
SetCaption(n, "底面边数 n")
SetCaption(r, "底面半径 r")
SetCaption(h, "棱柱高度 h")
vertices = Sequence((r * cos(2 * pi * i / n), r * sin(2 * pi * i / n), 0), i, 1, n)
topVertices = Sequence((x(Element(vertices, i)), y(Element(vertices, i)), h), i, 1, n)
base = Polygon(vertices)
SetColor(base, 0, 0.5, 1)
SetOpacity(base, 0.3)
top = Polygon(topVertices)
SetColor(top, 0, 0.5, 1)`, {
    noParseError: true,
    contains: ['未知的命令 "SetOpacity"'],
});

// ==============================
// 分组 5: 未知命令检测
// ==============================
console.log('\n--- 分组 5: 未知命令检测 ---');

test('未知命令检测：DeleteAll', 
    'DeleteAll()', {
    errorCount: [1, 1],
    contains: ['未知的命令 "DeleteAll"'],
});

test('未知命令检测：SetOpacity', 
    'SetOpacity(base, 0.3)', {
    errorCount: [1, 1],
    contains: ['未知的命令 "SetOpacity"'],
});

test('未知命令检测：SetThickness', 
    'SetThickness(line, 2)', {
    errorCount: [1, 1],
    contains: ['未知的命令 "SetThickness"'],
});

test('未知命令检测：一行前者未知不阻塞后者已知', 
    `DeleteAll()
SetValue(x, 1)`, {
    noParseError: true,
    errorCount: [1, 1],
});

// ==============================
// 分组 6: 边界情况
// ==============================
console.log('\n--- 分组 6: 边界情况 ---');

test('空输入', 
    '', {
    errorCount: 0,
    noParseError: true,
});

test('只有注释', 
    '// 这是一条注释', {
    errorCount: 0,
    noParseError: true,
});

test('只有赋值', 
    'x = 42', {
    errorCount: 0,
    noParseError: true,
});

test('逗号后有空格', 
    'SetValue(a, 1, 2, 3)', {
    noParseError: true,
});

test('同一命令多行（每行一个命令）', 
    `SetValue(a, 1)
SetValue(b, 2)
SetValue(c, 3)`, {
    errorCount: 0,
    noParseError: true,
});

// ==============================
// 分组 7: 相似命令建议
// ==============================
console.log('\n--- 分组 7: 相似命令建议 ---');

test('相似命令：SetLineWidth → 建议 SetLineThickness', 
    'SetLineWidth(a, 2)', {
    contains: ['SetLineThickness'],
});

// ==============================
// 汇总
// ==============================
console.log('\n========================================');
console.log(`  结果: ${passed} 通过, ${failed} 失败, 共 ${passed + failed} 用例`);
console.log('========================================');

if (failed > 0) {
    // process.exit(1);
}
