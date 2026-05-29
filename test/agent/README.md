# Agent 工具调用测试 Demo

这个demo展示了如何使用 `pi-ai-go/agent` 包创建一个能够调用工具的智能Agent。

## 功能特性

### 支持的工具

1. **计算器 (calculator)**
   - 支持基本四则运算：+, -, *, /
   - 支持数学函数：sqrt, sin, cos, tan, log
   - 示例：`计算 sqrt(16) + 3 * 2`

2. **天气查询 (weather)**
   - 查询指定城市的天气信息
   - 返回温度、天气、湿度、风速等信息
   - 示例：`查询北京的天气`

3. **数据库查询 (database_query)**
   - 支持多种查询类型：
     - `user_info` - 查询用户信息
     - `order_list` - 查询订单列表
     - `product_info` - 查询商品信息
   - 示例：`查询用户信息`

4. **搜索工具 (search)**
   - 在网络上搜索信息
   - 返回相关文章列表
   - 示例：`搜索人工智能的最新进展`

### Agent 特性

- 🔄 **工具自动调用**：Agent会根据用户问题自动选择合适的工具
- 📡 **事件订阅**：实时接收和处理Agent的各种事件
- 💬 **交互式对话**：支持多轮对话
- ⏱️ **超时控制**：每个请求都有超时保护
- 📊 **消息历史**：维护完整的对话历史

## 使用方法

### 1. 配置环境

确保在项目根目录的 `.env` 文件中配置了API密钥：

```bash
SILICONFLOW_API_KEY=your_api_key_here
SILICONFLOW_BASE_URL=https://api.siliconflow.cn/v1
SILICONFLOW_MODEL=Qwen/Qwen2.5-7B-Instruct
```

### 2. 运行Demo

```bash
cd /mnt/workspace/pi-ai-go/test/agent
go run main.go
```

### 3. 开始对话

运行后，你可以输入各种问题来测试不同的工具：

```
👤 你: 计算 123 + 456 * 789
🤖 [Agent 开始运行]
💬 [助手回复]: 我来帮你计算这个表达式。
🔧 [开始执行工具] calculator
   参数: {"expression":"123+456*789"}
   ✅ [工具执行完成] 359907.00
💬 [助手回复]: 计算结果是 359,907

👤 你: 查询上海的天气
🔧 [开始执行工具] weather
   参数: {"city":"上海"}
   ✅ [工具执行完成] {...}
```

## 代码结构

```
main.go
├── 工具定义
│   ├── calculatorTool()      - 计算器工具
│   ├── weatherTool()         - 天气查询工具
│   ├── databaseQueryTool()   - 数据库查询工具
│   └── searchTool()          - 搜索工具
├── 辅助函数
│   ├── evaluateExpression()  - 表达式求值
│   ├── printEvent()          - 事件打印
│   └── loadEnv()             - 环境变量加载
└── main()
    ├── 环境配置
    ├── Agent创建
    ├── 事件订阅
    └── 交互式对话循环
```

## 扩展指南

### 添加新工具

1. 创建工具函数：

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
            // 解析参数
            var args struct {
                Param1 string `json:"param1"`
            }
            json.Unmarshal(params, &args)
            
            // 执行工具逻辑
            result := doSomething(args.Param1)
            
            // 返回结果
            return agent.AgentToolResult{
                Content: []piai.ContentBlock{piai.TextContent{
                    Type: "text",
                    Text: result,
                }},
            }, nil
        },
    }
}
```

2. 在 `main()` 中添加到工具列表：

```go
tools := []agent.AgentTool{
    calculatorTool(),
    weatherTool(),
    databaseQueryTool(),
    searchTool(),
    myNewTool(),  // 新工具
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

## 注意事项

1. **API限制**：请确保API调用频率不超过提供商的限制
2. **超时设置**：默认60秒超时，可根据需要调整
3. **错误处理**：工具执行失败时会返回错误信息给用户
4. **并发安全**：Agent内部使用锁保证并发安全

## 相关文档

- [pi-ai-go 包文档](../../README.md)
- [Agent 包文档](../../agent/README.md)
- [测试文件](../../agent/agent_test.go)
