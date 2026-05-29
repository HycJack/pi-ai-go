package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	piai "pi-ai-go"
	"pi-ai-go/agent"
	_ "pi-ai-go/providers"
)

// loadEnv 从 .env 文件加载环境变量
func loadEnv(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析 KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// 只在环境变量未设置时才加载
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

// calculatorTool 创建一个计算器工具
func calculatorTool() agent.AgentTool {
	return agent.AgentTool{
		Name:        "calculator",
		Description: "执行数学计算。支持基本运算（+, -, *, /）和数学函数（sqrt, pow, sin, cos, tan, log等）",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"expression": {
					"type": "string",
					"description": "数学表达式，例如 '2+3*4' 或 'sqrt(16)'"
				}
			},
			"required": ["expression"]
		}`),
		Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
			// 解析参数
			var args struct {
				Expression string `json:"expression"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: fmt.Sprintf("参数解析错误: %v", err),
					}},
					IsError: true,
				}, nil
			}

			// 模拟计算（简化版本，实际应用中可以使用更复杂的表达式解析器）
			result, err := evaluateExpression(args.Expression)
			if err != nil {
				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: fmt.Sprintf("计算错误: %v", err),
					}},
					IsError: true,
				}, nil
			}

			return agent.AgentToolResult{
				Content: []piai.ContentBlock{piai.TextContent{
					Type: "text",
					Text: fmt.Sprintf("%.2f", result),
				}},
			}, nil
		},
	}
}

// evaluateExpression 简化的表达式求值
func evaluateExpression(expr string) (float64, error) {
	// 移除空格
	expr = strings.ReplaceAll(expr, " ", "")
	
	// 处理一些基本的数学函数
	if strings.HasPrefix(expr, "sqrt(") && strings.HasSuffix(expr, ")") {
		inner := expr[5 : len(expr)-1]
		val, err := strconv.ParseFloat(inner, 64)
		if err != nil {
			return 0, fmt.Errorf("无效的数值: %s", inner)
		}
		if val < 0 {
			return 0, fmt.Errorf("不能对负数开平方")
		}
		return math.Sqrt(val), nil
	}
	
	if strings.HasPrefix(expr, "sin(") && strings.HasSuffix(expr, ")") {
		inner := expr[4 : len(expr)-1]
		val, err := strconv.ParseFloat(inner, 64)
		if err != nil {
			return 0, fmt.Errorf("无效的数值: %s", inner)
		}
		return math.Sin(val * math.Pi / 180), nil // 假设输入是角度
	}
	
	if strings.HasPrefix(expr, "cos(") && strings.HasSuffix(expr, ")") {
		inner := expr[4 : len(expr)-1]
		val, err := strconv.ParseFloat(inner, 64)
		if err != nil {
			return 0, fmt.Errorf("无效的数值: %s", inner)
		}
		return math.Cos(val * math.Pi / 180), nil
	}
	
	if strings.HasPrefix(expr, "tan(") && strings.HasSuffix(expr, ")") {
		inner := expr[4 : len(expr)-1]
		val, err := strconv.ParseFloat(inner, 64)
		if err != nil {
			return 0, fmt.Errorf("无效的数值: %s", inner)
		}
		rad := val * math.Pi / 180
		if math.Abs(math.Cos(rad)) < 1e-10 {
			return 0, fmt.Errorf("tan(%.0f) 未定义", val)
		}
		return math.Tan(rad), nil
	}
	
	if strings.HasPrefix(expr, "log(") && strings.HasSuffix(expr, ")") {
		inner := expr[4 : len(expr)-1]
		val, err := strconv.ParseFloat(inner, 64)
		if err != nil {
			return 0, fmt.Errorf("无效的数值: %s", inner)
		}
		if val <= 0 {
			return 0, fmt.Errorf("log 的参数必须大于 0")
		}
		return math.Log10(val), nil
	}

	// 简单的四则运算处理
	// 查找运算符（+、-、*、/）
	ops := []string{"+", "-", "*", "/"}
	for _, op := range ops {
		idx := strings.LastIndex(expr, op)
		if idx > 0 { // 确保不是第一个字符（负号）
			left := expr[:idx]
			right := expr[idx+1:]
			
			leftVal, err := strconv.ParseFloat(left, 64)
			if err != nil {
				return 0, fmt.Errorf("无效的左操作数: %s", left)
			}
			
			rightVal, err := strconv.ParseFloat(right, 64)
			if err != nil {
				return 0, fmt.Errorf("无效的右操作数: %s", right)
			}
			
			switch op {
			case "+":
				return leftVal + rightVal, nil
			case "-":
				return leftVal - rightVal, nil
			case "*":
				return leftVal * rightVal, nil
			case "/":
				if rightVal == 0 {
					return 0, fmt.Errorf("除数不能为 0")
				}
				return leftVal / rightVal, nil
			}
		}
	}
	
	// 如果没有运算符，尝试直接解析为数字
	val, err := strconv.ParseFloat(expr, 64)
	if err != nil {
		return 0, fmt.Errorf("无法解析表达式: %s", expr)
	}
	return val, nil
}

// weatherTool 创建一个天气查询工具
func weatherTool() agent.AgentTool {
	return agent.AgentTool{
		Name:        "weather",
		Description: "查询指定城市的天气信息",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"city": {
					"type": "string",
					"description": "城市名称，例如 '北京'、'上海'、'深圳'"
				}
			},
			"required": ["city"]
		}`),
		Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
			var args struct {
				City string `json:"city"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: fmt.Sprintf("参数解析错误: %v", err),
					}},
					IsError: true,
				}, nil
			}

			// 模拟天气数据
			weatherData := map[string]interface{}{
				"城市":     args.City,
				"温度":     "25°C",
				"天气":     "晴朗",
				"湿度":     "45%",
				"风速":     "10km/h",
				"更新时间": time.Now().Format("2006-01-02 15:04:05"),
			}

			result, _ := json.MarshalIndent(weatherData, "", "  ")

			return agent.AgentToolResult{
				Content: []piai.ContentBlock{piai.TextContent{
					Type: "text",
					Text: string(result),
				}},
			}, nil
		},
	}
}

// databaseQueryTool 创建一个数据库查询工具
func databaseQueryTool() agent.AgentTool {
	return agent.AgentTool{
		Name:        "database_query",
		Description: "查询数据库中的信息。支持查询用户信息、订单记录等。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query_type": {
					"type": "string",
					"description": "查询类型",
					"enum": ["user_info", "order_list", "product_info"]
				},
				"params": {
					"type": "object",
					"description": "查询参数"
				}
			},
			"required": ["query_type"]
		}`),
		Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
			var args struct {
				QueryType string                 `json:"query_type"`
				Params    map[string]interface{} `json:"params"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: fmt.Sprintf("参数解析错误: %v", err),
					}},
					IsError: true,
				}, nil
			}

			// 模拟数据库查询
			var result map[string]interface{}
			switch args.QueryType {
			case "user_info":
				result = map[string]interface{}{
					"id":     1001,
					"name":   "张三",
					"email":  "zhangsan@example.com",
					"phone":  "13800138000",
					"status": "活跃",
				}
			case "order_list":
				result = map[string]interface{}{
					"orders": []map[string]interface{}{
						{"id": "ORD001", "amount": 299.00, "status": "已完成", "date": "2024-01-15"},
						{"id": "ORD002", "amount": 599.00, "status": "配送中", "date": "2024-01-20"},
						{"id": "ORD003", "amount": 149.00, "status": "待支付", "date": "2024-01-22"},
					},
					"total": 3,
				}
			case "product_info":
				productID, _ := args.Params["product_id"].(string)
				if productID == "" {
					productID = "unknown"
				}
				result = map[string]interface{}{
					"id":          productID,
					"name":        "示例商品",
					"price":       99.99,
					"stock":       150,
					"description": "这是一个示例商品描述",
					"category":    "电子产品",
				}
			default:
				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: fmt.Sprintf("不支持的查询类型: %s", args.QueryType),
					}},
					IsError: true,
				}, nil
			}

			resultJSON, _ := json.MarshalIndent(result, "", "  ")

			return agent.AgentToolResult{
				Content: []piai.ContentBlock{piai.TextContent{
					Type: "text",
					Text: string(resultJSON),
				}},
			}, nil
		},
	}
}

// searchTool 创建一个搜索工具
func searchTool() agent.AgentTool {
	return agent.AgentTool{
		Name:        "search",
		Description: "在网络上搜索信息",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "搜索关键词"
				},
				"limit": {
					"type": "integer",
					"description": "返回结果数量，默认为5"
				}
			},
			"required": ["query"]
		}`),
		Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
			var args struct {
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: fmt.Sprintf("参数解析错误: %v", err),
					}},
					IsError: true,
				}, nil
			}

			if args.Limit == 0 {
				args.Limit = 5
			}

			// 模拟搜索结果
			results := []map[string]interface{}{
				{
					"title":   fmt.Sprintf("关于 '%s' 的文章1", args.Query),
					"url":     "https://example.com/article1",
					"snippet": fmt.Sprintf("这篇文章详细介绍了 %s 的相关内容...", args.Query),
				},
				{
					"title":   fmt.Sprintf("'%s' 最新研究", args.Query),
					"url":     "https://example.com/research",
					"snippet": fmt.Sprintf("最新的研究表明 %s 在多个领域都有应用...", args.Query),
				},
				{
					"title":   fmt.Sprintf("如何理解 %s", args.Query),
					"url":     "https://example.com/guide",
					"snippet": fmt.Sprintf("本文将帮助您深入理解 %s 的核心概念...", args.Query),
				},
			}

			if args.Limit < len(results) {
				results = results[:args.Limit]
			}

			result := map[string]interface{}{
				"query":   args.Query,
				"total":   len(results),
				"results": results,
			}

			resultJSON, _ := json.MarshalIndent(result, "", "  ")

			return agent.AgentToolResult{
				Content: []piai.ContentBlock{piai.TextContent{
					Type: "text",
					Text: string(resultJSON),
				}},
			}, nil
		},
	}
}

// printEvent 打印事件信息
func printEvent(evt agent.AgentEvent) {
	switch e := evt.(type) {
	case agent.EventAgentStart:
		fmt.Println("\n🤖 [Agent 开始运行]")
	case agent.EventAgentEnd:
		fmt.Printf("\n✅ [Agent 运行结束，共 %d 条消息]\n", len(e.Messages))
	case agent.EventTurnStart:
		fmt.Println("\n🔄 [新一轮对话开始]")
	case agent.EventTurnEnd:
		fmt.Printf("🔄 [对话轮次结束，工具调用数: %d]\n", len(e.ToolResults))
	case agent.EventMessageStart:
		fmt.Print("💬 [助手回复]: ")
	case agent.EventMessageUpdate:
		// 只打印文本增量（从 AssistantEvent 中提取）
		switch evt := e.AssistantEvent.(type) {
		case piai.EventTextDelta:
			fmt.Print(evt.Delta)
		}
	case agent.EventMessageEnd:
		fmt.Println()
	case agent.EventToolExecStart:
		fmt.Printf("\n🔧 [开始执行工具] %s (ID: %s)\n", e.ToolName, e.ToolCallID)
		fmt.Printf("   参数: %s\n", string(e.Args))
	case agent.EventToolExecUpdate:
		fmt.Printf("   ⏳ [工具执行中] 部分结果: %s\n", string(e.PartialResult))
	case agent.EventToolExecEnd:
		if e.IsError {
			fmt.Printf("   ❌ [工具执行失败] %s\n", string(e.Result))
		} else {
			fmt.Printf("   ✅ [工具执行完成] %s\n", string(e.Result))
		}
	}
}

func main() {
	// 加载 .env 文件
	if err := loadEnv("../../.env"); err != nil {
		fmt.Printf("警告: 无法加载 .env 文件: %v\n", err)
	}

	// 从环境变量获取配置
	apiKey := os.Getenv("XIAOMI_API_KEY")
	baseURL := os.Getenv("XIAOMI_BASE_URL")
	modelID := os.Getenv("XIAOMI_MODEL")

	if apiKey == "" {
		fmt.Println("错误: 请在 .env 文件中设置 API_KEY")
		os.Exit(1)
	}
	if baseURL == "" {
		baseURL = "https://api.siliconflow.cn/v1"
	}
	if modelID == "" {
		modelID = "Qwen/Qwen2.5-7B-Instruct"
	}

	// 创建模型
	model := piai.Model{
		ID:            modelID,
		API:           piai.APIOpenAICompletions,
		Provider:      piai.ProviderDeepSeek,
		BaseURL:       baseURL,
		Input:         []piai.Modality{piai.ModalityText},
		ContextWindow: 64000,
		MaxTokens:     4096,
		Cost: piai.Cost{
			Input:  0.14,
			Output: 0.28,
		},
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("🤖 Agent 工具调用测试 Demo")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("模型: %s\n", modelID)
	fmt.Printf("API: %s\n\n", baseURL)

	// 创建工具列表
	tools := []agent.AgentTool{
		calculatorTool(),
		weatherTool(),
		databaseQueryTool(),
		searchTool(),
	}

	// 创建 Agent
	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model:  model,
			SystemPrompt: `你是一个智能助手，可以使用各种工具来帮助用户。

你拥有以下工具：
1. calculator - 执行数学计算
2. weather - 查询天气信息
3. database_query - 查询数据库信息
4. search - 搜索网络信息

请根据用户的问题，选择合适的工具来回答。如果不需要使用工具，可以直接回答。

注意事项：
- 使用工具时，请确保参数正确
- 如果工具执行失败，请向用户解释原因
- 尽量提供准确、有用的信息`,
			Tools: tools,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: apiKey,
				},
			},
		},
	})

	// 订阅事件
	aiAgent.Subscribe(printEvent)

	// 交互式对话循环
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("💬 开始对话（输入 'quit' 或 'exit' 退出）")
	fmt.Println("📝 示例问题:")
	fmt.Println("   - 计算 123 + 456 * 789")
	fmt.Println("   - 查询北京的天气")
	fmt.Println("   - 查询用户信息")
	fmt.Println("   - 搜索人工智能的最新进展")
	fmt.Println()

	for {
		fmt.Print("\n👤 你: ")
		if !scanner.Scan() {
			break
		}

		input := "杭州天气如何"//"strings.TrimSpace(scanner.Text())"
		if input == "" {
			continue
		}

		if strings.ToLower(input) == "quit" || strings.ToLower(input) == "exit" {
			fmt.Println("\n👋 再见！")
			break
		}

		// 运行 Agent
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		_, err := aiAgent.Run(ctx, piai.UserMessage{
			Role:    "user",
			Content: input,
		})
		cancel()

		if err != nil {
			fmt.Printf("\n❌ 错误: %v\n", err)
		}

		// 显示消息历史统计
		messages := aiAgent.Messages()
		fmt.Printf("\n📊 [消息历史: %d 条消息]\n", len(messages))
	}
}
