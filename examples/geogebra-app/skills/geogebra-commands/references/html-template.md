# GeoGebra HTML 课件模板

使用 GeoGebra 官方 API，通过 deployggb.js 嵌入交互课件：

```html
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>GeoGebra 课件</title>
  <style>
    body { margin: 0; display: flex; justify-content: center; align-items: center; height: 100vh; background: #f5f5f5; font-family: sans-serif; }
  </style>
</head>
<body>
  <div id="ggbApplet"></div>
  <script src="https://www.geogebra.org/apps/deployggb.js"></script>
  <script>
    var params = {
      "appName": "classic",
      "width": 800,
      "height": 600,
      "showToolBar": true,
      "showAlgebraInput": true,
      "showMenuBar": true,
      "showResetIcon": true,
      "material_id": "",
      "ggbBase64": ""
    };
    var applet = new GGBApplet('classic', params, true);
    window.addEventListener('load', function() {
      applet.inject('ggbApplet', 'preferJava');
    });
  </script>
</body>
</html>
```

## 注意事项

- 绝对不要使用 `<canvas>` 标签、Three.js、D3.js、SVG 或其他绘图技术
- 只允许使用 GeoGebra 技术栈
