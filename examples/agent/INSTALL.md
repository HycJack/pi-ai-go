# Agent Demo 安装和使用指南

## 前置要求

### 1. 安装 Go 语言环境

#### Linux/macOS:
```bash
# 下载 Go (以 1.23.0 为例)
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz

# 解压到 /usr/local
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz

# 添加到 PATH (添加到 ~/.bashrc 或 ~/.zshrc)
export PATH=$PATH:/usr/local/go/bin

# 验证安装
go version
```

#### Windows:
1. 访问 https://go.dev/dl/
2. 下载 Windows 安装程序
3. 运行安装程序并按照提示操作
4. 打开命令提示符，运行 `go version` 验证

### 2. 配置 API 密钥

在项目根目录创建 `.env` 文件：

```bash
cd /mnt/workspace/pi-ai-go
cat > .env << 'ENVEOF'
# SiliconFlow API 配置
SILICONFLOW_API_KEY=your_api_key_here
SILICONFLOW_BASE_URL=https://api.siliconflow.cn/v1
SILICONFLOW_MODEL=Qwen/Qwen2.5-7B-Instruct
ENVEOF
```

**注意**: 将 `your_api_key_here` 替换为你的实际 API 密钥。

## 快速开始

### 方法 1: 使用运行脚本（推荐）

```bash
cd /mnt/workspace/pi-ai-go/test/agent

# 添加执行权限
chmod +x run.sh

# 运行脚本
./run.sh
```

脚本会显示一个交互式菜单，让你选择不同的测试模式。

### 方法 2: 直接运行交互式对话

```bash
cd /mnt/workspace/pi-ai-go/test/agent

# 运行交互式对话
go run main.go
```

### 方法 3: 运行自动化测试

```bash
cd /mnt/workspace/pi-ai-go/test/agent

# 运行所有测试
go test -v -run "TestRunAll" -timeout 300s

# 运行特定测试
go test -v -run "TestCalculatorTool" -timeout 60s
go test -v -run "TestWeatherTool" -timeout 60s
go test -v -run "TestMultipleTools" -timeout 60s
```

## 功能演示

### 交互式对话示例

运行 `go run main.go` 后，可以尝试以下对话：

```
👤 你: 计算 123 + 456 * 789
🤖 [Agent 开始运行]
🔧 [开始执行工具] calculator
   参数: {"expression":"123+456*789"}
   ✅ [工具执行完成] 359907.00
💬 [助手回复]: 计算结果是 359,907

👤 你: 查询北京的天气
🔧 [开始执行工具] weather
   参数: {"city":"北京"}
   ✅ [工具执行完成] {...}
💬 [助手回复]: 北京今天天气晴朗，温度25°C...

👤 你: 查询用户信息
🔧 [开始执行工具] database_query
   参数: {"query_type":"user_info"}
   ✅ [工具执行完成] {...}
💬 [助手回复]: 用户信息如下...

👤 你: 搜索人工智能的最新进展
🔧 [开始执行工具] search
   参数: {"query":"人工智能最新进展"}
   ✅ [工具执行完成] {...}
💬 [助手回复]: 搜索结果显示...

👤 你: quit
👋 再见！
```

## 工具说明

### 1. 计算器 (calculator)

支持的运算：
- 基本运算: `+`, `-`, `*`, `/`
- 数学函数: `sqrt`, `sin`, `cos`, `tan`, `log`

示例：
- `计算 2 + 3 * 4`
- `计算 sqrt(16)`
- `计算 sin(30)`
- `计算 log(100)`

### 2. 天气查询 (weather)

查询指定城市的天气信息，包括：
- 温度
- 天气状况
- 湿度
- 风速

示例：
- `查询北京的天气`
- `上海今天天气怎么样`

### 3. 数据库查询 (database_query)

支持的查询类型：
- `user_info` - 查询用户信息
- `order_list` - 查询订单列表
- `product_info` - 查询商品信息

示例：
- `查询用户信息`
- `查看订单列表`
- `查询商品详情`

### 4. 搜索工具 (search)

在网络上搜索信息，返回相关文章列表。

示例：
- `搜索机器学习的最新进展`
- `查找 Go 语言教程`

## 代码结构说明

```
pi-ai-go/test/agent/
├── main.go              # 主程序（交互式对话）
├── auto_test.go         # 自动化测试文件
├── example_usage.go     # 使用示例代码
├── run.sh              # 运行脚本
├── go.mod              # Go 模块文件
├── README.md           # 项目说明
├── INSTALL.md          # 安装指南（本文件）
└── USAGE_EXAMPLES.md   # 使用示例文档
```

## 扩展开发

### 添加新工具

1. 在 `main.go` 中创建新的工具函数：

```go
func myNewTool() agent.AgentTool {
    return agent.AgentTool{
        Name:        "my_tool",
        Description: "工具描述",
        Parameters: json.RawMessage(`{
            "type": "object",
            "properties": {
                "param1": {
                    "type": "string",
                    "description": "参数说明"
                }
            },
            "required": ["param1"]
        }`),
        Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
            // 实现工具逻辑
            return agent.AgentToolResult{
                Content: []piai.ContentBlock{piai.TextContent{
                    Type: "text",
                    Text: "工具执行结果",
                }},
            }, nil
        },
    }
}
```

2. 在 `main()` 函数中添加工具：

```go
tools := []agent.AgentTool{
    calculatorTool(),
    weatherTool(),
    databaseQueryTool(),
    searchTool(),
    myNewTool(),  // 添加新工具
}
```

### 自定义事件处理

修改 `printEvent()` 函数来自定义事件处理逻辑：

```go
func printEvent(evt agent.AgentEvent) {
    switch e := evt.(type) {
    case agent.EventToolExecStart:
        // 自定义工具开始执行的处理
        log.Printf("Tool started: %s", e.ToolName)
        
    case agent.EventToolExecEnd:
        // 自定义工具执行完成的处理
        if e.IsError {
            log.Printf("Tool failed: %s", e.ToolName)
        } else {
            log.Printf("Tool completed: %s", e.ToolName)
        }
    }
}
```

## 常见问题

### Q: 编译错误 "package pi-ai-go is not in GOROOT"

A: 确保你在正确的目录下运行命令：
```bash
cd /mnt/workspace/pi-ai-go/test/agent
go run main.go
```

### Q: API 调用失败

A: 检查以下几点：
1. `.env` 文件中的 API 密钥是否正确
2. API 基础 URL 是否正确
3. 模型 ID 是否有效
4. 网络连接是否正常

### Q: 工具没有被调用

A: 可能的原因：
1. System Prompt 没有明确指示使用工具
2. 工具描述不够清晰
3. 用户问题与工具功能不匹配

尝试改进 System Prompt，明确说明可用的工具。

### Q: 如何更换 LLM 提供商？

A: 修改 `main.go` 中的模型配置：

```go
model := piai.Model{
    ID:            "your-model-id",
    API:           piai.APIOpenAICompletions,
    Provider:      piai.ProviderDeepSeek,  // 或其他提供商
    BaseURL:       "https://your-api-endpoint.com/v1",
    Input:         []piai.Modality{piai.ModalityText},
    ContextWindow: 64000,
    MaxTokens:     4096,
}
```

同时更新 `.env` 文件中的 API 密钥。

## 性能优化建议

1. **并行工具执行**: 设置 `ToolExecution: agent.ToolExecParallel` 可以并行执行多个工具
2. **超时控制**: 合理设置 context 超时时间
3. **错误重试**: 在工具执行函数中添加重试逻辑
4. **缓存**: 对频繁查询的结果进行缓存

## 调试技巧

1. **启用详细日志**: 订阅所有事件并打印详细信息
2. **单独测试工具**: 使用测试文件单独测试每个工具
3. **查看消息历史**: 使用 `aiAgent.Messages()` 查看完整对话历史
4. **使用 context**: 传递有意义的 context 以便于调试

## 相关资源

- [pi-ai-go 项目文档](../../README.md)
- [Agent 包源码](../../agent/)
- [Agent 测试文件](../../agent/agent_test.go)
- [Go 语言官方文档](https://go.dev/doc/)
