---
name: geogebra-commands
description: Generate GeoGebra commands and interactive HTML courseware from user descriptions. Use when the user asks to create geometry, functions, algebra visualizations, or interactive math courseware with GeoGebra.
---

# GeoGebra 指令生成专家

根据用户描述生成 GeoGebra 命令和网页版课件。

## 重要约束

- 只使用 GeoGebra（https://www.geogebra.org/）相关技术
- 绝对不要生成 SVG、Three.js、D3.js、Canvas、WebGL 或其他绘图方案
- 绝对不要在 HTML 中使用 `<canvas>`、Three.js CDN、D3.js CDN
- 交互式课件必须使用 GeoGebra 官方 API（deployggb.js）

## 输出规范

- 必须使用 ```geogebra 或 ```ggb 代码块输出 GeoGebra 命令
- 必须使用 ```html 代码块输出完整的 HTML
- 先给出简短的中文解释，说明生成的内容
- GeoGebra 命令间用换行分隔，每条命令独立一行
- 点坐标使用 (x, y) 或 (x, y, z) 格式
- 支持 2D 和 3D 几何
- **最终输出的 GeoGebra 命令代码块中不能包含任何注释**（不要出现 `//` 或 `#` 开头的注释行），只输出纯命令
- **一行一条指令**，不要将多条命令写在同一行

## 语法基础

### 变量赋值

GeoGebra 支持用 `=` 给对象命名：

```
A = (0, 0)           // 创建名为 A 的点
B = (3, 0)           // 创建名为 B 的点
f(x) = x^2           // 创建名为 f 的函数
s = Segment(A, B)    // 创建名为 s 的线段
list = {1, 2, 3}     // 创建列表
```

### 命令调用

命令使用 `CommandName(arg1, arg2, ...)` 语法，参数用逗号分隔：

```
Circle(A, 5)         // 以 A 为圆心，半径 5 画圆
Polygon(A, B, C)     // 过三点 A, B, C 画多边形
```

## 常用命令分类

### 几何作图

| 命令 | 签名 | 说明 |
|------|------|------|
| Point | Point(\<x\>, \<y\>) | 创建点 |
| Midpoint | Midpoint(\<Point\>, \<Point\>) | 中点 |
| Segment | Segment(\<Point\>, \<Point\>) | 线段 |
| Line | Line(\<Point\>, \<Point\>) | 直线 |
| Ray | Ray(\<Point\>, \<Point\>) | 射线 |
| Circle | Circle(\<Point\>, \<radius\>) | 圆心半径画圆 |
| Polygon | Polygon(\<Point\>, ...) | 多边形 |
| Polyline | Polyline(\<Point\>, ...) | 折线 |
| Angle | Angle(\<Point\>, \<Vertex\>, \<Point\>) | 角 |
| Intersect | Intersect(\<Object\>, \<Object\>) | 交点 |
| Tangent | Tangent(\<Point\>, \<Conic\>) | 切线 |
| AngularBisector | AngularBisector(\<Point\>, \<Vertex\>, \<Point\>) | 角平分线 |
| PerpendicularLine | PerpendicularLine(\<Point\>, \<Line\>) | 垂线 |
| ParallelLine | ParallelLine(\<Point\>, \<Line\>) | 平行线 |

### 变换

| 命令 | 签名 | 说明 |
|------|------|------|
| Reflect | Reflect(\<Object\>, \<Line\>) | 轴对称反射 |
| Reflect | Reflect(\<Object\>, \<Point\>) | 中心对称 |
| Rotate | Rotate(\<Object\>, \<Angle\>, \<Point\>) | 旋转 |
| Translate | Translate(\<Object\>, \<Vector\>) | 平移 |
| Dilate | Dilate(\<Object\>, \<Ratio\>, \<Point\>) | 缩放（位似） |

### 度量与计算

| 命令 | 签名 | 说明 |
|------|------|------|
| Distance | Distance(\<Point\>, \<Point\>) | 距离 |
| Area | Area(\<Polygon\>) | 面积 |
| Perimeter | Perimeter(\<Polygon\>) | 周长 |
| Length | Length(\<Segment\>) | 线段长度 |
| Slope | Slope(\<Line\>) | 斜率 |

### 函数与曲线

| 命令 | 签名 | 说明 |
|------|------|------|
| Function | Function(\<expression\>, \<x-min\>, \<x-max\>) | 函数 |
| Curve | Curve(\<x-expr\>, \<y-expr\>, \<t\>, \<t-min\>, \<t-max\>) | 参数曲线 |
| Derivative | Derivative(\<Function\>) | 导数 |
| Integral | Integral(\<Function\>, \<x-min\>, \<x-max\>) | 定积分 |

### 样式与交互

| 命令 | 签名 | 说明 |
|------|------|------|
| SetColor | SetColor(\<Object\>, \<"color"\>) | 设置颜色 |
| SetLineStyle | SetLineStyle(\<Object\>, \<n\>) | 线型 (0=实线, 10=虚线, 15=点线, 20=点划线) |
| SetLineThickness | SetLineThickness(\<Object\>, \<n\>) | 线宽 |
| ShowLabel | ShowLabel(\<Object\>, \<true/false\>) | 显示标签 |
| SetCaption | SetCaption(\<Object\>, "\<text\>") | 设置标题 |
| SetValue | SetValue(\<Object\>, \<value\>) | 设置值 |
| StartAnimation | StartAnimation(\<Object\>) | 开始动画 |
| Slider | Slider(\<min\>, \<max\>, \<increment\>) | 创建滑块 |

## 完整命令签名参考

`references/commandSignatures.json` 包含 509 个 GeoGebra 命令的完整签名、描述和示例。当需要使用上面未列出的命令时，请查阅此文件。

## 其他参考文件

- `references/commandSignatures.json` — 509 个命令的完整签名（GGB 命令生成时查阅）
- `references/html-template.md` — HTML 课件模板
- `references/examples.md` — 综合示例（函数、平面几何、立体几何、交互课件）
- `references/app-parameters.md` — GeoGebra Applet 初始化参数（**生成 HTML 课件时参考**）
- `references/apps-api.md` — GeoGebra Apps JavaScript API（**生成 HTML 课件时参考**）

## 常见错误

1. **命令名拼写错误**：GeoGebra 命令区分大小写，如 `Polygon` 不是 `polygon`
2. **参数数量不匹配**：每个命令有固定的参数签名，如 `Circle(A, 5)` 不能写成 `Circle(A)`
3. **使用非 GeoGebra 技术**：不要输出 SVG、Canvas、Three.js 等非 GeoGebra 绘图方案
4. **禁止使用不存在的命令**：以下命令**不存在**，不要使用：

| 错误命令 | 正确命令 | 说明 |
|---|---|---|
| `SetThickness` | `SetLineThickness` | 设置线宽 |
| `SetOpacity` | `SetFilling` | 设置填充透明度（0-1） |
| `SetBarThickness` | `SetLineThickness` | 柱状图线宽 |
| `DeleteAll` | 逐个 `Delete(<obj>)` | 清空所有对象（API 用 `getAllObjectNames` + `deleteObject`） |
| `SetLineWidth` | `SetLineThickness` | 线宽 |
| `SetFill` | `SetFilling` | 填充 |
| `SetTransparency` | `SetFilling` | 透明度 |

所有可用命令请查阅 `references/commandSignatures.json`，不要自行猜测命令名。

5. **使用纯 ASCII 数学符号**：GeoGebra 使用纯 ASCII 文本语法，不要使用 Unicode 符号：

| 错误写法 | 正确写法 | 说明 |
|---|---|---|
| `π` (Unicode pi) | `pi` | 圆周率，如 `cos(2 * pi * i / n)` |
| `∞` | `infinity` | 无穷大 |
| `θ` | `theta` | 角度变量 |
| `α`, `β`, `γ` | `alpha`, `beta`, `gamma` | 希腊字母变量名 |
| `²`、`³` | `^2`、`^3` | 指数，如 `x^2` |
| `√` | `sqrt` | 平方根，如 `sqrt(2)` |

注意：`cos(2π)` 这种隐式乘法不受支持，必须写 `cos(2 * pi)`。所有乘法必须使用 `*` 运算符。

## 角度指令使用注意事项

### 角度单位

GeoGebra 默认使用度（°）而非弧度。角度值后面必须加 `°` 符号：

```
// 正确
Rotate(tri, 90°, center)
Angle((1, 0), (0, 0), (0, 1))

// 错误 — 缺少 ° 符号
Rotate(tri, 90, center)
```

如果需要用弧度，必须显式标注：
```
Rotate(tri, π/2, center)    // 用 π 表示弧度
```

### Angle 命令的多重签名

`Angle` 命令有多种签名，注意参数顺序和含义：

| 签名 | 说明 |
|---|---|
| `Angle(<Object>)` | 圆锥曲线的旋转角 |
| `Angle(<Vector>, <Vector>)` | 两向量夹角 |
| `Angle(<Line>, <Line>)` | 两直线夹角 |
| `Angle(<Line>, <Plane>)` | 直线与平面夹角 |
| `Angle(<Plane>, <Plane>)` | 两平面夹角 |
| `Angle(<Point>, <Apex>, <Point>)` | 三点定义的角，**第二个点是顶点** |
| `Angle(<Point>, <Apex>, <Angle>)` | 从顶点出发指定大小的角 |
| `Angle(<Point>, <Point>, <Point>, <Direction>)` | 3D 中带方向的角度 |

### 三点定义角度的顶点位置

`Angle(A, B, C)` 中 **B 是顶点**，不是 A：

```
A = (1, 0)
B = (0, 0)    // B 是顶点
C = (0, 1)
Angle(A, B, C)    // 返回 90°
```

### 多边形角度的方向性

`Angle(<Polygon>)` 返回的内角/外角取决于多边形顶点的方向（顺时针/逆时针）：

```
// 逆时针 — 返回内角
poly1 = Polygon((4, 1), (2, 4), (1, 1))
Angle(poly1)

// 顺时针 — 可能返回外角
poly2 = Polygon((1, 1), (2, 4), (4, 1))
Angle(poly2)
```

### 3D 角度与方向

3D 中使用 `Direction` 参数可以绕过标准角度显示区间：

```
// 3D 中带方向的角度
Angle((1, -1, 0), (0, 0, 0), (-1, -1, 0), zAxis)
```

### Rotate 命令的角度

`Rotate` 旋转方向为**逆时针**，角度必须带 `°`：

```
// 逆时针旋转 90°
Rotate(tri, 90°, center)

// 3D 绕轴旋转
Rotate(tri, 45°, xAxis)
```

### 角平分线

`AngleBisector` 返回两条角平分线（内角平分线和外角平分线）：

```
// 两条直线的角平分线 — 返回两条线
AngleBisector(x + y = 1, x - y = 2)

// 三点定义的角平分线 — 第二个点是顶点
AngleBisector((1, 1), (4, 4), (7, 1))
```

### 希腊字母与角度变量

GeoGebra 中 `α` (alpha)、`β` (beta)、`γ` (gamma) 常用作角度变量名。注意 `Gamma` 命令是 Gamma 分布函数，不是角度命令：

```
// 角度变量
α = 45°
Rotate(tri, α, center)

// Gamma 分布 — 不要混淆
Gamma(2, 1, 0.5)
```
