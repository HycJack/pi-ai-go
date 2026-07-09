---
name: calculator-tips
description: 使用 calculator 工具时的最佳实践
---

# Calculator Tips

调用 `calculator` 工具时:

- **始终传完整表达式**，不要拆分。如 `2*(3+4)/5`。
- **支持函数**: `sqrt`, `sin`, `cos`, `tan`, `log`, `ln`, `abs`, `floor`, `ceil`, `round`, `pow`。
- **三角函数输入为角度**，需要弧度时手写换算。
- 整数结果以 `g` 格式返回，浮点保留 6 位小数。

## 常见用例

- 复利: `principal * (1 + rate) ^ years`
- 标准差: `sqrt(sum((x - mean)^2) / n)`