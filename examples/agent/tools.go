package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	piai "pi-ai-go"
	"pi-ai-go/agent"
)

// errJSON 工具错误响应的辅助函数。
func errJSON(format string, args ...any) (agent.AgentToolResult, error) {
	return agent.AgentToolResult{
		Content: []piai.ContentBlock{piai.TextContent{
			Type: "text",
			Text: fmt.Sprintf(format, args...),
		}},
		IsError: true,
	}, nil
}

// textJSON 工具成功响应的辅助函数。
func textJSON(v any) (agent.AgentToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errJSON("序列化结果失败: %v", err)
	}
	return agent.AgentToolResult{
		Content: []piai.ContentBlock{piai.TextContent{
			Type: "text",
			Text: string(b),
		}},
	}, nil
}

// calculatorTool 数学计算器（支持运算符优先级、括号、函数）。
func calculatorTool() agent.AgentTool {
	return agent.AgentTool{
		Name:        "calculator",
		Description: "执行数学计算。支持 + - * / ^ 运算符、括号、以及 sqrt/sin/cos/tan/log/ln/abs/floor/ceil/round/pow 函数（sin/cos/tan 输入为角度）。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"expression": {
					"type": "string",
					"description": "数学表达式，例如 '2+3*4'、'(1+2)^3'、'sqrt(16)'"
				}
			},
			"required": ["expression"]
		}`),
		Execute: func(_ context.Context, _ string, params json.RawMessage, _ func(json.RawMessage)) (agent.AgentToolResult, error) {
			var args struct {
				Expression string `json:"expression"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return errJSON("参数解析错误: %v", err)
			}
			if args.Expression == "" {
				return errJSON("expression 不能为空")
			}
			v, err := evaluateExpression(args.Expression)
			if err != nil {
				return errJSON("计算错误: %v", err)
			}
			// 整数用整数格式，否则保留 6 位小数
			if v == math.Trunc(v) && math.Abs(v) < 1e15 {
				return textJSON(map[string]any{
					"expression": args.Expression,
					"result":     v,
					"formatted":  fmt.Sprintf("%g", v),
				})
			}
			return textJSON(map[string]any{
				"expression": args.Expression,
				"result":     v,
				"formatted":  fmt.Sprintf("%.6f", v),
			})
		},
	}
}

// weatherTool 模拟天气查询。
func weatherTool() agent.AgentTool {
	return agent.AgentTool{
		Name:        "weather",
		Description: "查询指定城市的天气信息（模拟数据）。",
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
		Execute: func(_ context.Context, _ string, params json.RawMessage, _ func(json.RawMessage)) (agent.AgentToolResult, error) {
			var args struct {
				City string `json:"city"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return errJSON("参数解析错误: %v", err)
			}
			if args.City == "" {
				return errJSON("city 不能为空")
			}
			return textJSON(map[string]any{
				"city":        args.City,
				"temperature": "25°C",
				"condition":   "晴朗",
				"humidity":    "45%",
				"wind_speed":  "10km/h",
				"updated_at":  time.Now().Format("2006-01-02 15:04:05"),
			})
		},
	}
}

// databaseQueryTool 模拟数据库查询。
func databaseQueryTool() agent.AgentTool {
	return agent.AgentTool{
		Name:        "database_query",
		Description: "查询数据库（模拟数据）。支持 user_info / order_list / product_info 三种类型。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query_type": {
					"type": "string",
					"enum": ["user_info", "order_list", "product_info"],
					"description": "查询类型"
				},
				"params": {
					"type": "object",
					"description": "查询参数（product_info 时传 product_id）"
				}
			},
			"required": ["query_type"]
		}`),
		Execute: func(_ context.Context, _ string, params json.RawMessage, _ func(json.RawMessage)) (agent.AgentToolResult, error) {
			var args struct {
				QueryType string                 `json:"query_type"`
				Params    map[string]interface{} `json:"params"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return errJSON("参数解析错误: %v", err)
			}
			switch args.QueryType {
			case "user_info":
				return textJSON(map[string]any{
					"id":     1001,
					"name":   "张三",
					"email":  "zhangsan@example.com",
					"phone":  "13800138000",
					"status": "活跃",
				})
			case "order_list":
				return textJSON(map[string]any{
					"orders": []map[string]any{
						{"id": "ORD001", "amount": 299.00, "status": "已完成", "date": "2024-01-15"},
						{"id": "ORD002", "amount": 599.00, "status": "配送中", "date": "2024-01-20"},
						{"id": "ORD003", "amount": 149.00, "status": "待支付", "date": "2024-01-22"},
					},
					"total": 3,
				})
			case "product_info":
				productID, _ := args.Params["product_id"].(string)
				if productID == "" {
					productID = "unknown"
				}
				return textJSON(map[string]any{
					"id":          productID,
					"name":        "示例商品",
					"price":       99.99,
					"stock":       150,
					"description": "这是一个示例商品描述",
					"category":    "电子产品",
				})
			default:
				return errJSON("不支持的查询类型: %s", args.QueryType)
			}
		},
	}
}

// searchTool 模拟搜索。
func searchTool() agent.AgentTool {
	return agent.AgentTool{
		Name:        "search",
		Description: "在网络上搜索信息（模拟数据）。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "搜索关键词"
				},
				"limit": {
					"type": "integer",
					"description": "返回结果数量，默认为 5"
				}
			},
			"required": ["query"]
		}`),
		Execute: func(_ context.Context, _ string, params json.RawMessage, _ func(json.RawMessage)) (agent.AgentToolResult, error) {
			var args struct {
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return errJSON("参数解析错误: %v", err)
			}
			if args.Query == "" {
				return errJSON("query 不能为空")
			}
			if args.Limit <= 0 {
				args.Limit = 5
			}
			all := []map[string]any{
				{
					"title":   fmt.Sprintf("关于 '%s' 的文章", args.Query),
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
			if args.Limit < len(all) {
				all = all[:args.Limit]
			}
			return textJSON(map[string]any{
				"query":   args.Query,
				"total":   len(all),
				"results": all,
			})
		},
	}
}

// defaultTools 返回所有内置工具。
func defaultTools() []agent.AgentTool {
	return []agent.AgentTool{
		calculatorTool(),
		weatherTool(),
		databaseQueryTool(),
		searchTool(),
	}
}

// 默认系统提示。
const defaultSystemPrompt = `你是一个智能助手，可以使用各种工具来帮助用户。

可用工具：
1. calculator - 执行数学计算
2. weather - 查询天气信息
3. database_query - 查询数据库信息
4. search - 搜索网络信息

注意事项：
- 使用工具时，请确保参数正确
- 如果工具执行失败，请向用户解释原因
- 尽量提供准确、有用的信息
- 回答简洁明了，避免冗长
`
