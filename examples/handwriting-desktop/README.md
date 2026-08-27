# 手写文字生成器 — Handwriting Desktop

基于 Wails v2 的桌面应用，参考 [handwriting-web](https://github.com/14790897/handwriting-web) 设计，集成 pi-ai-go Agent 能力。

## 功能

- **手写文字渲染** — 在 A4 尺寸页面生成逼真的手写文字效果
- **参数调节** — 字号、行间距、边距、扰动（位置/旋转/墨水深浅）等丰富参数
- **涂改痕迹** — 随机删除线效果，模仿真实手写本的涂改
- **分页排版** — `---` 强制分页，`>>>` 右对齐
- **PDF 导出** — 文本可复制搜索的 PDF（嵌入 TTF 字体子集）
- **AI 助手** — 基于 pi-ai-go Agent，可帮助撰写/润色/翻译文本

## 技术栈

| 层次 | 技术 |
|------|------|
| 桌面框架 | [Wails v2](https://wails.io/) |
| 后端 | Go 1.23 |
| PDF 生成 | [gopdf](https://github.com/signintech/gopdf) |
| 字体渲染 | [golang.org/x/image/font/opentype](https://pkg.go.dev/golang.org/x/image/font/opentype) |
| AI Agent | pi-ai-go `agent.AgentLoopDetailed` |
| 前端 | React 18 + Vite 5 + Tailwind CSS 3 |

## 快速开始

### 前置条件

- Go 1.23+
- Node.js 18+ (推荐 pnpm)
- [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation)

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
# 或：npm install -g @wailsapp/cli
```

### 开发运行

```bash
cd examples/handwriting-desktop
# 构建前端
cd frontend && pnpm install && pnpm build && cd ..
# Wails 开发模式
wails dev
```

**注意**：首次构建时 Wails 可能需要下载 CGo 依赖（WebView2 等），请确保网络畅通。

### 构建安装包

```bash
cd examples/handwriting-desktop
wails build
```

构建产物在 `build/bin/` 目录。

## 使用指南

### 基本流程

1. 在左侧文本区输入内容
2. 选择合适的字体
3. 调整参数（字号、行间距、边距等）
4. 点击"预览"查看效果
5. 点击"生成图片"保存为 PNG 文件
6. 点击"导出 PDF"生成可复制文本的 PDF

### AI 助手

在底部 AI 聊天区输入自然语言指令，例如：

- "帮我写一封请假条"
- "把这段文字润色得更正式"
- "翻译成英文"
- "生成一段关于春天的散文"
- "然后生成手写" — Agent 会自动调用工具生成手写文件

### 排版标记

| 标记 | 效果 |
|------|------|
| `---` (独占一行) | 强制分页 |
| `>>>文本` (行首) | 该行右对齐 |

### 参数说明

| 参数 | 默认 | 说明 |
|------|------|------|
| 字号 | 100 | 字体大小(点),建议 70-150 |
| 行间距 | 150 | 两行基线距离 |
| 宽度/高度 | 2481/3507 | 页面尺寸(A4 像素) |
| X/Y 偏移扰动 | 3/3 | 每个字符随机偏移程度 |
| 旋转扰动 | 0.05 | 每个字符随机旋转程度 |
| 墨水深浅 | 30 | 笔画透明度扰动 |
| 涂改概率 | 0.005 | 每个字符产生删除线的概率 |

## 项目结构

```
handwriting-desktop/
├── main.go          # Wails 启动入口
├── app.go           # 应用结构/绑定方法(由 wails init 生成)
├── types.go         # 数据结构定义
├── fonts.go         # 内置字体加载
├── renderer.go      # 手写渲染核心(Go image 绘图)
├── generate.go      # 生成入口
├── pdf.go           # PDF 导出(gopdf + 字体子集嵌入)
├── agent.go         # Agent 集成(对话 + 工具)
├── models.go        # 模型/Provider 解析
├── settings.go      # 设置持久化
├── logger.go        # 日志器
├── frontend/        # React 前端
│   ├── src/
│   │   ├── App.jsx    # 主界面
│   │   ├── style.css  # 样式
│   │   └── main.jsx   # 入口
│   └── index.html
└── wails.json       # Wails 配置
```

## 许可

MIT
