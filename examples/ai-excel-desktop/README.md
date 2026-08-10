# AI Excel 工作台 (Wails + Go + React)

基于 Wails + Go 的 AI 操作 Excel 桌面应用。前端使用 React，AI 对话区采用 **ai-elements** 风格组件 + **streamdown** 流式 Markdown 渲染，AI 助手为**可收起/展开的侧边栏**。

## 技术栈

### 后端 (Go)
| 库 | 用途 |
|---|---|
| [wailsapp/wails/v2](https://github.com/wailsapp/wails) v2.12.0 | 桌面应用框架 |
| [xuri/excelize/v2](https://github.com/xuri/excelize) v2.11.0 | Excel 读写 |

### 前端 (React + Vite)
| 库 | 用途 |
|---|---|
| **streamdown** 2.5 | AI 流式 Markdown 渲染（react-markdown 替代品，支持 GFM/代码高亮/Mermaid） |
| **ai-elements** 风格组件 | 手写 Conversation / Message / PromptInput（参考 shadcn/ui 设计令牌） |
| lucide-react | 图标 |
| ag-grid-community 31 | 虚拟滚动数据表格 |
| chart.js 3.9 | 图表渲染 |
| tailwindcss 3.4 | 原子化 CSS（shadcn 设计令牌模式） |

## 界面布局

```
┌──────────────────────────────────────────────────────┬──────────────┐
│ Header: 标题 │ 文件名 │ 打开文件 │ [✨ 切换侧边栏]   │              │
├──────────────────────────────────────────────────────┤   AI 侧边栏  │
│ 统计卡片: 行数 │ 列数 │ 数值列 │ 文本列                │  (可收起)    │
│ ┌────────────────────────────────────────────────┐   │              │
│ │ 工作表选择 + 导出按钮                            │   │ 对话消息流   │
│ ├────────────────────────────────────────────────┤   │ (streamdown  │
│ │                                                │   │  渲染)       │
│ │           AG-Grid 数据表格                      │   │              │
│ │           (虚拟滚动)                            │   │              │
│ │                                                │   │              │
│ ├────────────────────────────────────────────────┤   │              │
│ │ Chart.js 图表区                                 │   │ ┌──────────┐ │
│ │ (柱/线/饼/散点/直方/面积)                         │   │ │ 输入框    │ │
│ └────────────────────────────────────────────────┘   │ └──────────┘ │
└──────────────────────────────────────────────────────┴──────────────┘
```

### AI 侧边栏特性
- **可收起/展开**：顶部 ✨ 按钮或侧边栏头部按钮切换；收起后右侧浮动一个 ✨ 圆形按钮重新展开
- **对话流**：用户消息为气泡，AI 回复用 **streamdown** 渲染 Markdown（标题/列表/代码/引用块）
- **自动吸底**：滚动到底部按钮
- **快捷建议**：根据是否加载数据显示示例指令芯片
- **消息操作**：最后一条 AI 消息支持复制 / 重新生成
- **空状态**：未对话时显示引导

## AI 回复示例（streamdown 渲染的 Markdown）

```markdown
### 已生成 折线图

**配置：**
- **X 轴**：`月份`
- **Y 轴**：`销售额、利润`
- **采样**：5,000 行（共 50,000 行）

> 已识别 line 图表：X轴=月份，Y轴=销售额、利润

图表已渲染到右侧画布，可下载 PNG。
```

## 开发

```bash
cd C:/Users/huangyicao/Downloads/ai-excel-desktop
wails dev      # 热重载开发
wails build    # 生成 build/bin/ai-excel-desktop.exe
```

## 文件结构

```
ai-excel-desktop/
├── main.go              # Wails 入口
├── app.go               # Go 后端（Excel 解析 + AI 图表配置）
├── wails.json
└── frontend/
    ├── index.html
    ├── package.json
    ├── tailwind.config.js   # shadcn 设计令牌
    ├── postcss.config.js
    ├── vite.config.js
    └── src/
        ├── main.jsx          # React 入口
        ├── App.jsx           # 主应用（布局 + 状态）
        ├── style.css        # 全局样式 + streamdown + shadcn 令牌
        ├── lib/utils.js     # Wails 绑定 + 工具函数
        └── components/
            ├── AISidebar.jsx   # 可收起 AI 侧边栏
            ├── Chat.jsx        # ai-elements 风格对话组件
            ├── DataTable.jsx   # AG-Grid 封装
            └── ChartView.jsx   # Chart.js 封装
```

## 组件 API

### AISidebar
```jsx
<AISidebar
  open={bool}              // 展开/收起
  onToggle={fn}            // 切换
  messages={[{id, role, content, chart?}]}
  onSend={fn(text)}
  onClear={fn}
  onCopyLast={fn}
  onRegenerate={fn}
  isLoading={bool}
  summary={string}
  dataLoaded={bool}
/>
```

### ai-elements 风格组件（Chat.jsx）
- `Conversation` — 滚动容器，自动吸底，含"滚动到底部"按钮
- `Message` — 消息项，区分 user / assistant 角色
- `MessageContent` — 用户气泡 / AI 用 `<Streamdown>` 渲染 Markdown
- `MessageActions` / `MessageAction` — 复制、重新生成按钮
- `ConversationEmptyState` — 空状态
- `PromptInput` — 输入框 + 快捷建议芯片 + 发送按钮

## 后端 API (Go)

| 方法 | 说明 |
|---|---|
| `OpenFileDialog()` | 文件选择对话框 |
| `LoadFile(path)` | 加载文件元信息 |
| `LoadSheet(name)` | 加载工作表数据 |
| `AnalyzeColumns()` | 列统计分析 |
| `GenerateChartConfig(prompt)` | 自然语言 → ChartConfig |
| `GetDataSummary()` | 数据摘要 |
| `ExportExcel(headers, rows)` | 导出 Excel |
| `ExportCSV(headers, rows)` | 导出 CSV |
| `SampleData(maxRows)` | 大数据采样 |
