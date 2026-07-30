# GeoGebra 综合示例

## 一、函数与代数

### 示例 1: 二次函数与导数

```geogebra
f(x) = x^2 - 4x + 3
g(x) = Derivative(f)
h(x) = Integral(f, 0, 3)
root1 = Root(f, 0)
root2 = Root(f, 3)
vertex = Extremum(f)
ShowLabel(root1, true)
ShowLabel(root2, true)
ShowLabel(vertex, true)
```

### 示例 2: 三角函数与周期

```geogebra
a = Slider(0.5, 3, 0.1)
f(x) = a * sin(x)
g(x) = a * cos(x)
h(x) = a * tan(x)
Integral(f, 0, 2π)
roots = Root(f, 0, 2π)
```

### 示例 3: 极限与渐近线

```geogebra
f(x) = (x^2 - 1) / (x - 1)
g(x) = Limit(f, 1)
asym = Asymptote(f)
ShowLabel(f, true)
ShowLabel(g, true)
```

### 示例 4: 序列与求和

```geogebra
seq = Sequence(1/n, n, 1, 10)
s = Sum(Sequence(n^2, n, 1, 10))
p = Product(Sequence(n, n, 1, 5))
```

## 二、平面几何

### 示例 5: 勾股定理

```geogebra
A = (0, 0)
B = (3, 0)
C = (0, 4)
AB = Segment(A, B)
BC = Segment(B, C)
AC = Segment(A, C)
a = Distance(B, C)
b = Distance(A, C)
c = Distance(A, B)
ShowLabel(A, true)
ShowLabel(B, true)
ShowLabel(C, true)
SetColor(AB, "red")
SetColor(BC, "blue")
SetColor(AC, "green")
```

### 示例 6: 三角形的外接圆与内切圆

```geogebra
A = (0, 0)
B = (4, 0)
C = (2, 3)
tri = Polygon(A, B, C)
circum = Circumcircle(A, B, C)
inc = Incircle(A, B, C)
center_c = Center(circum)
center_i = Center(inc)
ShowLabel(center_c, true)
ShowLabel(center_i, true)
```

### 示例 7: 圆锥曲线

```geogebra
c1 = Circle((0, 0), 3)
e1 = Ellipse((−4, 0), (4, 0), 2)
p1 = Parabola((0, 0), 1)
h1 = Hyperbola((−3, 0), (3, 0), 2)
t1 = Tangent((3, 0), e1)
a1 = Asymptote(h1)
```

### 示例 8: 角平分线与垂直平分线

```geogebra
A = (0, 0)
B = (4, 0)
C = (2, 3)
ab = AngleBisector(A, B, C)
mid = Midpoint(A, B)
perp = PerpendicularLine(mid, Segment(A, B))
```

### 示例 9: 几何变换

```geogebra
A = (1, 1)
B = (4, 1)
C = (2, 4)
tri = Polygon(A, B, C)
// 轴对称
mirror = Reflect(tri, xAxis)
// 中心对称
center = (0, 0)
rotated = Rotate(tri, 90°, center)
// 平移
translated = Translate(tri, Vector((2, 3)))
// 位似
dilated = Dilate(tri, 2, center)
```

## 三、立体几何

### 示例 10: 正四面体

```geogebra
A = (0, 0, 0)
B = (4, 0, 0)
tet = Tetrahedron(A, B)
```

### 示例 11: 正方体（棱柱）

```geogebra
A = (0, 0, 0)
B = (4, 0, 0)
C = (4, 4, 0)
D = (0, 4, 0)
prism = Prism(A, B, C, (0, 0, 4))
```

### 示例 12: 圆柱与圆锥

```geogebra
A = (0, 0, 0)
B = (0, 0, 5)
cyl = Cylinder(A, B, 3)
C = (0, 0, 6)
cone = Cone(A, C, 3)
```

### 示例 13: 球体

```geogebra
center = (0, 0, 0)
sph = Sphere(center, 5)
// 球面上的点
P = Point(sph)
```

### 示例 14: 平面与截面

```geogebra
A = (0, 0, 0)
B = (4, 0, 0)
C = (4, 4, 0)
D = (0, 4, 0)
E = (0, 0, 4)
prism = Prism(A, B, C, E)
// 创建一个截面
plane = Plane((2, 0, 0), (2, 4, 0), (2, 2, 4))
cross = IntersectPath(plane, prism)
SetColor(cross, "red")
```

### 示例 15: 正多面体

```geogebra
A = (0, 0, 0)
B = (3, 0, 0)
tet = Tetrahedron(A, B)
octa = Octahedron(A, B)
dodec = Dodecahedron(A, B)
icosa = Icosahedron(A, B)
```

## 四、交互式课件

### 示例 16: 滑块驱动动画

```geogebra
a = Slider(1, 5, 0.1)
f(x) = a * x^2
g(x) = a * sin(x)
StartAnimation(a)
```

### 示例 17: 动态颜色

```geogebra
A = (0, 0)
B = (3, 0)
C = (1.5, 2)
tri = Polygon(A, B, C)
area = Area(tri)
SetDynamicColor(tri, area, 10 - area, 5)
```
