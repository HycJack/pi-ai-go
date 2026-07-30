# GeoGebra Apps API

> 来源: https://geogebra.github.io/docs/reference/en/GeoGebra_Apps_API/

## 获取 API 对象

通过 `appletOnLoad` 回调获取：

```javascript
const params = {
  appletOnLoad: (api) => {
    // api 就是 API 对象
    api.evalCommand('A = (1, 2)');
  },
};
```

## 创建对象

| 方法 | 说明 |
|---|---|
| `boolean evalCommand(String cmdString)` | 执行命令字符串（同输入栏），返回是否成功。可用 `\n` 分隔多条命令。**必须用英文命令名** |
| `String evalCommandGetLabels(String cmdString)` | 同 `evalCommand`，但返回创建对象的标签列表（逗号分隔，如 `"A,B,C"`） |
| `boolean evalLaTex(String input)` | 解析 LaTeX 字符串为构造元素（如 `x^{2}`、`\frac`） |
| `String evalCommandCAS(String string)` | 传给 CAS，返回结果字符串 |
| `void insertEmbed(String type, String uri)` | 插入嵌入元素（Notes） |

## 设置对象状态

### 通用方法

| 方法 | 说明 |
|---|---|
| `void deleteObject(String objName)` | 删除对象 |
| `void setValue(String objName, double value)` | 设置数值（boolean: 1=true） |
| `void setTextValue(String objName, String value)` | 设置文本值 |
| `void setListValue(String objName, int i, double value)` | 设置列表元素值 |
| `void setCoords(String objName, double x, double y)` | 设置坐标（2D） |
| `void setCoords(String objName, double x, double y, double z)` | 设置坐标（3D） |
| `void setCaption(String objName, String caption)` | 设置标题 |
| `void setColor(String objName, int red, int green, int blue)` | 设置颜色（RGB 0-255） |
| `void setVisible(String objName, boolean visible)` | 显示/隐藏 |
| `void setLabelVisible(String objName, boolean visible)` | 显示/隐藏标签 |
| `void setLabelStyle(String objName, int style)` | 标签样式: NAME=0, NAME_VALUE=1, VALUE=2, CAPTION=3 |
| `void setFixed(String objName, boolean fixed, boolean selectionAllowed)` | 固定对象 |
| `void setTrace(String objName, boolean flag)` | 开关轨迹 |
| `boolean renameObject(String oldObjName, String newObjName)` | 重命名 |
| `void setLayer(String objName, int layer)` | 设置图层 |
| `void setLayerVisible(int layer, boolean visible)` | 图层可见性 |
| `void setLineStyle(String objName, int style)` | 线型 (0-4) |
| `void setLineThickness(String objName, int thickness)` | 线宽 (1-13, -1=默认) |
| `void setPointStyle(String objName, int style)` | 点样式 (-1=默认, 0=实心圆, 1=十字, 2=圆, 3=加号, 4=实心菱形...) |
| `void setPointSize(String objName, int size)` | 点大小 (1-9) |
| `void setDisplayStyle(String objName, String style)` | 显示样式: `"parametric"`, `"explicit"`, `"implicit"`, `"specific"` |
| `void setFilling(String objName, double filling)` | 填充 (0-1) |
| `void setAuxiliary(geo, boolean)` | 设置辅助对象 |
| `void showAllObjects()` | 调整视图使所有对象可见 |

### 动画

| 方法 | 说明 |
|---|---|
| `void setAnimating(String objName, boolean animate)` | 设置动画标志 |
| `void setAnimationSpeed(String objName, double speed)` | 设置动画速度 |
| `void startAnimation()` | 开始动画 |
| `void stopAnimation()` | 停止动画 |
| `boolean isAnimationRunning()` | 动画是否运行中 |

## 获取对象状态

| 方法 | 说明 |
|---|---|
| `double getXcoord(String objName)` | x 坐标 |
| `double getYcoord(String objName)` | y 坐标 |
| `double getZcoord(String objName)` | z 坐标 |
| `double getValue(String objName)` | 数值（线段长度、多边形面积等，boolean: true=1） |
| `double getListValue(String objName, Integer index)` | 列表元素值 |
| `String getColor(String objName)` | 颜色 hex 字符串（如 `"#FF0000"`） |
| `boolean getVisible(String objName)` | 是否可见 |
| `boolean getVisible(String objName, int view)` | 指定视图是否可见 (1 或 2) |
| `String getValueString(String objName [, boolean useLocalizedInput])` | 值字符串 |
| `String getDefinitionString(String objName)` | 定义描述 |
| `String getCommandString(String objName [, boolean useLocalizedInput])` | 命令字符串 |
| `String getLaTeXString(String objName)` | LaTeX 语法值 |
| `String getLaTeXBase64(String objName, boolean value)` | LaTeX 的 base64 PNG |
| `String getObjectType(String objName)` | 类型（如 `"point"`, `"line"`, `"circle"`） |
| `boolean exists(String objName)` | 对象是否存在 |
| `boolean isDefined(String objName)` | 值是否有效 |
| `String[] getAllObjectNames([String type])` | 所有对象名（可按类型过滤） |
| `int getObjectNumber()` | 对象数量 |
| `String getObjectName(int i)` | 第 n 个对象名 |
| `String getLayer(String objName)` | 图层 |
| `int getLineStyle(String objName)` | 线型 (0-4) |
| `int getLineThickness(String objName)` | 线宽 (1-13) |
| `int getPointStyle(String objName)` | 点样式 |
| `int getPointSize(String objName)` | 点大小 (1-9) |
| `double getFilling(String objName)` | 填充 (0-1) |
| `String getCaption(String objectName, boolean substitutePlaceholders)` | 标题 |
| `int getLabelStyle(String objectName)` | 标签样式 |
| `boolean getLabelVisible()` | 标签是否可见 |
| `boolean isInteractive(String objName)` | 是否可通过 TAB 选中 |
| `boolean isIndependent(String objName)` | 是否独立 |
| `boolean isMoveable(String objName)` | 是否可移动 |

## 构造与界面

| 方法 | 说明 |
|---|---|
| `void setMode(int mode)` | 设置鼠标模式（工具） |
| `int getMode()` | 获取当前工具模式 |
| `void reset()` | 重载初始构造 |
| `void newConstruction()` | 清空所有对象 |
| `void refreshViews()` | 刷新所有视图（清除轨迹） |
| `void setOnTheFlyPointCreationActive(boolean flag)` | 是否允许即时创建点 |
| `void setPointCapture(view, mode)` | 点捕获模式。view: 1=graphics, 2=graphics2, -1=3D。mode: 0=无, 1=吸附网格, 2=固定网格, 3=自动 |
| `void setRounding(string round)` | 设置小数位数。`"10s"`=10有效数字, `"5"`=5小数位 |
| `void hideCursorWhenDragging(boolean flag)` | 拖拽时隐藏鼠标 |
| `void setRepaintingActive(boolean flag)` | 开关重绘（批量操作时用） |
| `void setErrorDialogsActive(boolean flag)` | 开关错误对话框 |
| `void setCoordSystem(double xmin, double xmax, double ymin, double ymax)` | 设置坐标系范围 |
| `void setCoordSystem(double xmin, double xmax, double ymin, double ymax, double zmin, double zmax)` | 设置 3D 坐标系范围 |
| `void setAxesVisible(boolean x, boolean y, boolean z)` | 显示/隐藏坐标轴 |
| `void setGridVisible(boolean flag)` | 显示/隐藏网格 |
| `void setCaption(String objName, String caption)` | 设置标题 |

## 导出

| 方法 | 说明 |
|---|---|
| `String getPNGBase64(double exportScale, boolean transparent, double DPI)` | 导出 PNG base64 |
| `void exportSVG(function callback)` | 导出 SVG（3D 返回 null） |
| `void exportPDF(double scale, function callback, String sliderLabel)` | 导出 PDF |
| `void getScreenshotBase64(function callback)` | 截图 base64 |
| `boolean writePNGtoFile(String filename, double exportScale, boolean transparent, double DPI)` | 写入 PNG 文件 |

## 事件监听

| 方法 | 说明 |
|---|---|
| `void registerAddListener(function listener)` | 添加对象时触发 |
| `void registerRemoveListener(function listener)` | 删除对象时触发 |
| `void registerUpdateListener(function listener)` | 更新对象时触发 |
| `void registerRenameListener(function listener)` | 重命名时触发 |
| `void registerClearListener(function listener)` | 清空构造时触发 |
| `void registerClientListener(function listener)` | 客户端事件（如 `setMode`、`undo`、`redo`、`selectNode`） |
| `void unregisterAddListener(function listener)` | 移除添加监听 |
| `void unregisterRemoveListener(function listener)` | 移除删除监听 |
| `void unregisterUpdateListener(function listener)` | 移除更新监听 |
| `void unregisterRenameListener(function listener)` | 移除重命名监听 |
| `void unregisterClearListener(function listener)` | 移除清空监听 |
| `void unregisterClientListener(function listener)` | 移除客户端监听 |

## 文件格式

| 方法 | 说明 |
|---|---|
| `String getXML()` | 获取完整构造的 XML |
| `String getXML(String objName)` | 获取指定对象的 XML |
| `void setXML(String xml)` | 设置构造的 XML |
| `String getBase64()` | 获取 base64 编码的 .ggb |
| `void setBase64(String base64)` | 从 base64 加载 |
| `String getFileJSON()` | 获取 JSON 格式 |
| `void setFileJSON(String json)` | 从 JSON 加载 |

## 本项目使用的 API

本项目主要使用以下方法：

```typescript
// 执行 GeoGebra 命令
api.evalCommand('A = (1, 2)');

// 获取所有对象名（用于 reset）
const names = api.getAllObjectNames();
for (const name of names) {
  api.deleteObject(name);
}

// 批量操作时关闭重绘
api.setRepaintingActive(false);
// ... 多个操作
api.setRepaintingActive(true);

// 关闭错误对话框
api.setErrorDialogsActive(false);
```
