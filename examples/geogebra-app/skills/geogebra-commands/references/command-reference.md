# GeoGebra 命令参考

GeoGebra 命令采用类似英文的语法。

## 几何作图

- Point(<x>, <y>) — 创建点
- Point(<Object>) — 在对象上创建点
- Midpoint(<Point>, <Point>) — 中点
- Segment(<Point>, <Point>) — 线段
- Line(<Point>, <Point>) — 直线
- Ray(<Point>, <Point>) — 射线
- Circle(<Point>, <radius>) — 圆心半径画圆
- Circle(<Point>, <Point>) — 两点决定圆心和圆周上一点
- Polygon(<Point>, <Point>, <Point>, ...) — 多边形
- Polyline(<Point>, <Point>, ...) — 折线
- Angle(<Point>, <Vertex>, <Point>) — 角
- Intersect(<Object>, <Object>) — 交点
- Tangent(<Point>, <Conic>) — 切线
- AngularBisector(<Point>, <Vertex>, <Point>) — 角平分线
- PerpendicularLine(<Point>, <Line>) — 垂线
- ParallelLine(<Point>, <Line>) — 平行线

## 变换

- Reflect(<Object>, <Line>) — 轴对称反射
- Reflect(<Object>, <Point>) — 中心对称
- Rotate(<Object>, <Angle>, <Point>) — 旋转
- Translate(<Object>, <Vector>) — 平移
- Dilate(<Object>, <Ratio>, <Point>) — 缩放（位似）

## 度量与计算

- Distance(<Point>, <Point>) — 距离
- Area(<Polygon>) — 面积
- Perimeter(<Polygon>) — 周长
- Length(<Segment>) — 线段长度
- Angle(<Point>, <Vertex>, <Point>) — 角度
- Slope(<Line>) — 斜率

## 函数与曲线

- Function(<expression>, <x-min>, <x-max>) — 函数
- Curve(<x-expr>, <y-expr>, <t>, <t-min>, <t-max>) — 参数曲线
- Derivative(<Function>) — 导数
- Integral(<Function>, <x-min>, <x-max>) — 定积分

## 样式与交互

- SetColor(<Object>, <"color-name">) — 设置颜色
- SetLineStyle(<Object>, <n>) — 线型 (0=实线, 10=虚线, 15=点线, 20=点划线)
- SetLineThickness(<Object>, <n>) — 线宽
- ShowLabel(<Object>, <true/false>) — 显示标签
- SetCaption(<Object>, "<text>") — 设置标题
- SetValue(<Object>, <value>) — 设置值
- StartAnimation(<Object>) / StartAnimation() — 开始动画
- Slider(<min>, <max>, <increment>) — 创建滑块
