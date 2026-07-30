# GeoGebra App 初始化参数

> 来源: https://geogebra.github.io/docs/reference/en/GeoGebra_App_Parameters/

## 应用类型

| 参数 | 值 | 说明 |
|---|---|---|
| `appName` | `"graphing"` | GeoGebra Graphing Calculator |
| | `"geometry"` | GeoGebra Geometry |
| | `"3d"` | GeoGebra 3D Graphing Calculator |
| | `"classic"` | GeoGebra Classic（默认） |
| | `"suite"` | GeoGebra Calculator Suite |
| | `"evaluator"` | Equation Editor |
| | `"scientific"` | Scientific Calculator |
| | `"notes"` | GeoGebra Notes |

## 尺寸与缩放

| 参数 | 值 | 说明 | 默认 |
|---|---|---|---|
| `width` | `800` | Applet 宽度（必填，除非用 `scaleContainerClass`） | - |
| `height` | `600` | Applet 高度（必填，除非用 `scaleContainerClass`） | - |
| `scale` | `2` | 缩放比例 | `1` |
| `scaleContainerClass` | `"my-container"` | 容器 CSS 类名，applet 自动缩放适配容器 | - |
| `autoHeight` | `true/false` | 自动计算高度 | - |
| `allowUpscale` | `true/false` | 是否允许放大 | `false` |

## 文件加载

| 参数 | 值 | 说明 |
|---|---|---|
| `material_id` | `"RHYH3UQ8"` | GeoGebra Materials id |
| `filename` | `"myFile.ggb"` | 要加载的文件 URL |
| `ggbBase64` | base64 字符串 | base64 编码的 .ggb 文件 |

## 界面控制

| 参数 | 值 | 说明 | 默认 |
|---|---|---|---|
| `showMenuBar` | `true/false` | 显示菜单栏 | `false` |
| `showToolBar` | `true/false` | 显示工具栏 | `false` |
| `showToolBarHelp` | `true/false` | 显示工具栏帮助文字 | - |
| `customToolBar` | `"0 1 2 \| 3 4"` | 自定义工具栏（Toolbar Mode Values） | - |
| `showAlgebraInput` | `true/false` | 显示代数输入栏 | `false` |
| `showResetIcon` | `true/false` | 显示重置图标 | `false` |
| `showZoomButtons` | `true/false` | 显示缩放按钮 | `false` |
| `showFullscreenButton` | `true/false` | 显示全屏按钮 | - |
| `showAnimationButton` | `true/false` | 显示动画按钮 | - |
| `showSuggestionButtons` | `true/false` | 显示建议按钮 | - |
| `allowStyleBar` | `true/false` | 允许样式栏 | `false` |
| `algebraInputPosition` | `"algebra"` / `"top"` / `"bottom"` | 输入栏位置 | 取决于文件 |
| `perspective` | 字符串 | 视图布局，参考 SetPerspective 命令 | - |

## 交互控制

| 参数 | 值 | 说明 | 默认 |
|---|---|---|---|
| `enableRightClick` | `true/false` | 右键菜单、属性对话框、右键缩放 | `true` |
| `enableLabelDrags` | `true/false` | 标签可拖拽 | `true` |
| `enableShiftDragZoom` | `true/false` | Shift+拖拽移动、Shift+滚轮缩放 | `true` |
| `enableUndoRedo` | `true/false` | 撤销/重做图标（需工具栏可见） | `true` |
| `enable3d` | `true/false` | 启用 3D（考试模式） | - |
| `enableCAS` | `true/false` | 启用 CAS（考试模式） | - |
| `enableFileFeatures` | `true/false` | 文件保存/加载、登录（需菜单栏可见） | `true` |
| `preventFocus` | `true/false` | 阻止自动获取焦点 | `false` |
| `disableJavaScript` | `true/false` | 禁用 material 文件中的 JavaScript | - |

## 外观

| 参数 | 值 | 说明 | 默认 |
|---|---|---|---|
| `borderColor` | `"#FFFFFF"` | 边框颜色 | 灰色 |
| `borderRadius` | `10` | 边框圆角（像素） | - |
| `transparentGraphics` | `true/false` | Graphics View 透明背景 | - |
| `buttonShadows` | `true/false` | 按钮阴影 | - |
| `buttonRounding` | `0` - `0.9` | 按钮圆角比例 | `0.2` |
| `buttonBorderColor` | `"#RRGGBB"` | 按钮边框颜色 | 黑色/深色 |
| `editorBackgroundColor` | `"#RRGGBB"` | 编辑器背景色（evaluator） | - |
| `editorForegroundColor` | `"#RRGGBB"` | 编辑器前景色（evaluator） | - |

## 语言与地区

| 参数 | 值 | 说明 |
|---|---|---|
| `language` | `"zh"` | ISO 语言代码 |
| `country` | `"CN"` | ISO 国家代码 |

## 随机数

| 参数 | 值 | 说明 | 默认 |
|---|---|---|---|
| `randomize` | `true/false` | 加载文件时是否随机化 | `true` |
| `randomSeed` | `33` | 随机数种子 | - |

## 其他

| 参数 | 值 | 说明 | 默认 |
|---|---|---|---|
| `id` | `"applet2"` | 传给 `ggbOnInit()` 的参数 | - |
| `appletOnLoad` | `function(api){...}` | 初始化完成回调 | - |
| `useBrowserForJS` | `true/false` | 是否从 HTML 运行 ggbOnInit | `false` |
| `showLog` | `true/false` | 浏览器控制台显示日志 | `false` |
| `capturingThreshold` | `3` | 对象选择灵敏度 | `3` |
| `rounding` | `"10"` / `"10s"` | 小数位数（`s` = 有效数字） | - |
| `playButton` | `true/false` | 显示预览图和播放按钮 | `false` |
| `showStartTooltip` | `true/false` | 显示欢迎提示 | - |
| `textmode` | `true/false` | 编辑器文本模式（evaluator） | `false` |
| `showKeyboardOnFocus` | `"auto"` / `true` / `false` | 聚焦时显示键盘 | `auto` |
| `keyboardType` | `"scientific"` / `"normal"` / `"notes"` | 键盘类型（evaluator） | - |
| `detachKeyboard` | `"auto"` / `true` / `false` | 键盘是否分离 | `auto` |
| `detachedKeyboardParent` | 字符串 | 键盘挂载目标选择器 | - |

## 本项目使用的参数

```typescript
const params = {
  appName: 'classic',
  width: 800,
  height: 500,
  showMenuBar: true,
  showToolBar: true,
  showAlgebraInput: true,
  enableRightClick: true,
  enableShiftDragZoom: true,
  showZoomButtons: true,
  enableUndoRedo: true,
  appletOnLoad: (api) => { /* ... */ },
};
```
