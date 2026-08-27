import { RuleEngine, noUnknownCommand, correctArgTypes, formatLintResults } from './lib/geogebra-lint/index';

const code = `n = Slider(3, 12, 1, 1, 100, false, true, false, false)
r = Slider(1, 5, 0.1, 1, 100, false, true, false, false)
h = Slider(1, 8, 0.1, 1, 100, false, true, false, false)
SetCaption(n, "底面边数 n")
SetCaption(r, "底面半径 r")
SetCaption(h, "棱柱高度 h")
vertices = Sequence((r * cos(2 * pi * i / n), r * sin(2 * pi * i / n), 0), i, 1, n)
topVertices = Sequence((x(Element(vertices, i)), y(Element(vertices, i)), h), i, 1, n)`;

const engine = new RuleEngine();
engine.registerRule(noUnknownCommand);
engine.registerRule(correctArgTypes);
const result = engine.lint(code);
console.log(formatLintResults(result));
