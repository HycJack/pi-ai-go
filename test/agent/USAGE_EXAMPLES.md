# Agent 使用示例集合

本文档提供了多个实际使用场景的示例代码，帮助你快速上手 Agent 工具调用功能。

## 目录

1. [基础示例](#基础示例)
2. [客服系统](#客服系统)
3. [数据分析助手](#数据分析助手)
4. [任务管理](#任务管理)
5. [代码助手](#代码助手)
6. [文件处理](#文件处理)
7. [API集成](#api集成)
8. [多模态应用](#多模态应用)

---

## 基础示例

### 最简单的 Agent

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	piai "pi-ai-go"
	"pi-ai-go/agent"
	_ "pi-ai-go/providers"
)

func main() {
	model := piai.Model{
		ID:       "Qwen/Qwen2.5-7B-Instruct",
		API:      piai.APIOpenAICompletions,
		Provider: piai.ProviderDeepSeek,
		BaseURL:  "https://api.siliconflow.cn/v1",
		Input:    []piai.Modality{piai.ModalityText},
	}

	// 定义一个简单的工具
	greetTool := agent.AgentTool{
		Name:        "greet",
		Description: "向用户打招呼",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {"type": "string", "description": "用户名称"}
			},
			"required": ["name"]
		}`),
		Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
			var args struct {
				Name string `json:"name"`
			}
			json.Unmarshal(params, &args)
			
			return agent.AgentToolResult{
				Content: []piai.ContentBlock{piai.TextContent{
					Type: "text",
					Text: fmt.Sprintf("你好，%s！欢迎使用我们的服务！", args.Name),
				}},
			}, nil
		},
	}

	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model:        model,
			SystemPrompt: "你是一个友好的助手。使用 greet 工具向用户打招呼。",
			Tools:        []agent.AgentTool{greetTool},
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: "your-api-key",
				},
			},
		},
	})

	result, err := aiAgent.Run(context.Background(), piai.UserMessage{
		Role:    "user",
		Content: "我是张三，请向我打招呼",
	})

	if err != nil {
		log.Fatal(err)
	}

	// 打印结果
	for _, msg := range result {
		if am, ok := msg.(piai.AssistantMessage); ok {
			for _, block := range am.Content {
				if text, ok := block.(piai.TextContent); ok {
					fmt.Println(text.Text)
				}
			}
		}
	}
}
```

---

## 客服系统

### 智能客服 Agent

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	piai "pi-ai-go"
	"pi-ai-go/agent"
	_ "pi-ai-go/providers"
)

// CustomerServiceAgent 客服Agent
type CustomerServiceAgent struct {
	agent   *agent.Agent
	tickets map[string]map[string]interface{}
}

func NewCustomerServiceAgent(apiKey string) *CustomerServiceAgent {
	model := piai.Model{
		ID:       "Qwen/Qwen2.5-7B-Instruct",
		API:      piai.APIOpenAICompletions,
		Provider: piai.ProviderDeepSeek,
		BaseURL:  "https://api.siliconflow.cn/v1",
		Input:    []piai.Modality{piai.ModalityText},
	}

	tickets := map[string]map[string]interface{}{}

	tools := []agent.AgentTool{
		{
			Name:        "create_ticket",
			Description: "创建工单",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"title": {"type": "string", "description": "工单标题"},
					"description": {"type": "string", "description": "问题描述"},
					"priority": {"type": "string", "enum": ["low", "medium", "high", "urgent"], "description": "优先级"}
				},
				"required": ["title", "description"]
			}`),
			Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
				var args struct {
					Title       string `json:"title"`
					Description string `json:"description"`
					Priority    string `json:"priority"`
				}
				json.Unmarshal(params, &args)

				if args.Priority == "" {
					args.Priority = "medium"
				}

				ticketID := fmt.Sprintf("TICKET-%d", len(tickets)+1)
				tickets[ticketID] = map[string]interface{}{
					"id":          ticketID,
					"title":       args.Title,
					"description": args.Description,
					"priority":    args.Priority,
					"status":      "open",
				}

				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: fmt.Sprintf("工单已创建：\nID: %s\n标题: %s\n优先级: %s\n状态: 已开启", ticketID, args.Title, args.Priority),
					}},
				}, nil
			},
		},
		{
			Name:        "get_ticket",
			Description: "查询工单状态",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"ticket_id": {"type": "string", "description": "工单ID"}
				},
				"required": ["ticket_id"]
			}`),
			Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
				var args struct {
					TicketID string `json:"ticket_id"`
				}
				json.Unmarshal(params, &args)

				ticket, exists := tickets[args.TicketID]
				if !exists {
					return agent.AgentToolResult{
						Content: []piai.ContentBlock{piai.TextContent{
							Type: "text",
							Text: fmt.Sprintf("未找到工单：%s", args.TicketID),
						}},
						IsError: true,
					}, nil
				}

				result := fmt.Sprintf("工单信息：\nID: %s\n标题: %s\n描述: %s\n优先级: %s\n状态: %s",
					ticket["id"], ticket["title"], ticket["description"],
					ticket["priority"], ticket["status"])

				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: result,
					}},
				}, nil
			},
		},
		{
			Name:        "update_ticket",
			Description: "更新工单状态",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"ticket_id": {"type": "string", "description": "工单ID"},
					"status": {"type": "string", "enum": ["open", "in_progress", "resolved", "closed"], "description": "新状态"}
				},
				"required": ["ticket_id", "status"]
			}`),
			Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
				var args struct {
					TicketID string `json:"ticket_id"`
					Status   string `json:"status"`
				}
				json.Unmarshal(params, &args)

				ticket, exists := tickets[args.TicketID]
				if !exists {
					return agent.AgentToolResult{
						Content: []piai.ContentBlock{piai.TextContent{
							Type: "text",
							Text: fmt.Sprintf("未找到工单：%s", args.TicketID),
						}},
						IsError: true,
					}, nil
				}

				ticket["status"] = args.Status

				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: fmt.Sprintf("工单 %s 状态已更新为：%s", args.TicketID, args.Status),
					}},
				}, nil
			},
		},
	}

	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model: model,
			SystemPrompt: `你是一个专业的客服助手。你可以帮助用户：
1. 创建工单 - 使用 create_ticket
2. 查询工单状态 - 使用 get_ticket
3. 更新工单状态 - 使用 update_ticket

请用友好、专业的语气回答用户问题。如果用户描述了一个问题，请主动创建工单。`,
			Tools: tools,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: apiKey,
				},
			},
		},
	})

	return &CustomerServiceAgent{
		agent:   aiAgent,
		tickets: tickets,
	}
}

func (cs *CustomerServiceAgent) Chat(message string) (string, error) {
	result, err := cs.agent.Run(context.Background(), piai.UserMessage{
		Role:    "user",
		Content: message,
	})

	if err != nil {
		return "", err
	}

	// 提取最后的助手回复
	for i := len(result) - 1; i >= 0; i-- {
		if am, ok := result[i].(piai.AssistantMessage); ok {
			var texts []string
			for _, block := range am.Content {
				if text, ok := block.(piai.TextContent); ok {
					texts = append(texts, text.Text)
				}
			}
			return strings.Join(texts, ""), nil
		}
	}

	return "", fmt.Errorf("没有收到回复")
}

func main() {
	cs := NewCustomerServiceAgent("your-api-key")

	// 模拟客服对话
	conversations := []string{
		"你好，我遇到了一个登录问题",
		"我无法登录到我的账户，一直显示密码错误",
		"我的账户是 user@example.com",
		"请帮我创建一个工单",
		"工单状态是什么？",
	}

	for _, msg := range conversations {
		fmt.Printf("\n👤 用户: %s\n", msg)
		reply, err := cs.Chat(msg)
		if err != nil {
			log.Printf("错误: %v", err)
			continue
		}
		fmt.Printf("🤖 客服: %s\n", reply)
	}
}
```

---

## 数据分析助手

### 数据查询和分析 Agent

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	piai "pi-ai-go"
	"pi-ai-go/agent"
	_ "pi-ai-go/providers"
)

func createDataAnalysisAgent(apiKey string) *agent.Agent {
	model := piai.Model{
		ID:       "Qwen/Qwen2.5-7B-Instruct",
		API:      piai.APIOpenAICompletions,
		Provider: piai.ProviderDeepSeek,
		BaseURL:  "https://api.siliconflow.cn/v1",
		Input:    []piai.Modality{piai.ModalityText},
	}

	// 模拟数据存储
	salesData := []map[string]interface{}{
		{"date": "2024-01", "product": "产品A", "amount": 15000, "quantity": 100},
		{"date": "2024-01", "product": "产品B", "amount": 22000, "quantity": 150},
		{"date": "2024-02", "product": "产品A", "amount": 18000, "quantity": 120},
		{"date": "2024-02", "product": "产品B", "amount": 25000, "quantity": 170},
		{"date": "2024-03", "product": "产品A", "amount": 20000, "quantity": 130},
		{"date": "2024-03", "product": "产品B", "amount": 28000, "quantity": 190},
	}

	tools := []agent.AgentTool{
		{
			Name:        "query_sales",
			Description: "查询销售数据",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"product": {"type": "string", "description": "产品名称（可选）"},
					"start_date": {"type": "string", "description": "开始日期（可选）"},
					"end_date": {"type": "string", "description": "结束日期（可选）"}
				}
			}`),
			Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
				var args struct {
					Product   string `json:"product"`
					StartDate string `json:"start_date"`
					EndDate   string `json:"end_date"`
				}
				json.Unmarshal(params, &args)

				filtered := []map[string]interface{}{}
				for _, record := range salesData {
					if args.Product != "" && record["product"] != args.Product {
						continue
					}
					if args.StartDate != "" && record["date"].(string) < args.StartDate {
						continue
					}
					if args.EndDate != "" && record["date"].(string) > args.EndDate {
						continue
					}
					filtered = append(filtered, record)
				}

				result, _ := json.MarshalIndent(filtered, "", "  ")
				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: string(result),
					}},
				}, nil
			},
		},
		{
			Name:        "calculate_statistics",
			Description: "计算统计数据",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"data": {
						"type": "array",
						"items": {"type": "number"},
						"description": "数据数组"
					},
					"operation": {
						"type": "string",
						"enum": ["sum", "average", "max", "min", "stddev"],
						"description": "统计操作"
					}
				},
				"required": ["data", "operation"]
			}`),
			Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
				var args struct {
					Data      []float64 `json:"data"`
					Operation string    `json:"operation"`
				}
				json.Unmarshal(params, &args)

				if len(args.Data) == 0 {
					return agent.AgentToolResult{
						Content: []piai.ContentBlock{piai.TextContent{
							Type: "text",
							Text: "数据为空",
						}},
						IsError: true,
					}, nil
				}

				var result float64
				switch args.Operation {
				case "sum":
					for _, v := range args.Data {
						result += v
					}
				case "average":
					sum := 0.0
					for _, v := range args.Data {
						sum += v
					}
					result = sum / float64(len(args.Data))
				case "max":
					result = args.Data[0]
					for _, v := range args.Data[1:] {
						if v > result {
							result = v
						}
					}
				case "min":
					result = args.Data[0]
					for _, v := range args.Data[1:] {
						if v < result {
							result = v
						}
					}
				case "stddev":
					avg := 0.0
					for _, v := range args.Data {
						avg += v
					}
					avg /= float64(len(args.Data))
					
					variance := 0.0
					for _, v := range args.Data {
						variance += (v - avg) * (v - avg)
					}
					variance /= float64(len(args.Data))
					result = math.Sqrt(variance)
				}

				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: fmt.Sprintf("%s 结果: %.2f", args.Operation, result),
					}},
				}, nil
			},
		},
	}

	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model: model,
			SystemPrompt: `你是一个数据分析助手。你可以：
1. 查询销售数据 - 使用 query_sales
2. 计算统计指标 - 使用 calculate_statistics

请帮助用户分析数据，提供有价值的洞察。`,
			Tools: tools,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: apiKey,
				},
			},
		},
	})

	return aiAgent
}

func main() {
	dataAgent := createDataAnalysisAgent("your-api-key")

	ctx := context.Background()

	// 示例查询
	queries := []string{
		"查询所有产品A的销售数据",
		"计算2024年第一季度的总销售额",
		"哪个产品的平均销售额更高？",
	}

	for _, query := range queries {
		fmt.Printf("\n📊 查询: %s\n", query)
		fmt.Println(strings.Repeat("-", 50))

		result, err := dataAgent.Run(ctx, piai.UserMessage{
			Role:    "user",
			Content: query,
		})

		if err != nil {
			log.Printf("错误: %v", err)
			continue
		}

		for _, msg := range result {
			if am, ok := msg.(piai.AssistantMessage); ok {
				for _, block := range am.Content {
					if text, ok := block.(piai.TextContent); ok {
						fmt.Println(text.Text)
					}
				}
			}
		}
	}
}
```

---

## 任务管理

### 任务管理 Agent

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	piai "pi-ai-go"
	"pi-ai-go/agent"
	_ "pi-ai-go/providers"
)

type Task struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	DueDate     string    `json:"due_date"`
	CreatedAt   time.Time `json:"created_at"`
}

func createTaskManagerAgent(apiKey string) (*agent.Agent, *[]Task) {
	model := piai.Model{
		ID:       "Qwen/Qwen2.5-7B-Instruct",
		API:      piai.APIOpenAICompletions,
		Provider: piai.ProviderDeepSeek,
		BaseURL:  "https://api.siliconflow.cn/v1",
		Input:    []piai.Modality{piai.ModalityText},
	}

	tasks := []Task{}

	tools := []agent.AgentTool{
		{
			Name:        "create_task",
			Description: "创建新任务",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"title": {"type": "string", "description": "任务标题"},
					"description": {"type": "string", "description": "任务描述"},
					"priority": {"type": "string", "enum": ["low", "medium", "high"], "description": "优先级"},
					"due_date": {"type": "string", "description": "截止日期 (YYYY-MM-DD)"}
				},
				"required": ["title"]
			}`),
			Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
				var args struct {
					Title       string `json:"title"`
					Description string `json:"description"`
					Priority    string `json:"priority"`
					DueDate     string `json:"due_date"`
				}
				json.Unmarshal(params, &args)

				if args.Priority == "" {
					args.Priority = "medium"
				}

				task := Task{
					ID:          fmt.Sprintf("TASK-%d", len(tasks)+1),
					Title:       args.Title,
					Description: args.Description,
					Status:      "todo",
					Priority:    args.Priority,
					DueDate:     args.DueDate,
					CreatedAt:   time.Now(),
				}

				tasks = append(tasks, task)

				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: fmt.Sprintf("任务已创建：\nID: %s\n标题: %s\n优先级: %s\n状态: 待办",
							task.ID, task.Title, task.Priority),
					}},
				}, nil
			},
		},
		{
			Name:        "list_tasks",
			Description: "列出任务",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"status": {"type": "string", "enum": ["todo", "in_progress", "done", "all"], "description": "按状态筛选"},
					"priority": {"type": "string", "enum": ["low", "medium", "high"], "description": "按优先级筛选"}
				}
			}`),
			Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
				var args struct {
					Status   string `json:"status"`
					Priority string `json:"priority"`
				}
				json.Unmarshal(params, &args)

				filtered := []Task{}
				for _, task := range tasks {
					if args.Status != "" && args.Status != "all" && task.Status != args.Status {
						continue
					}
					if args.Priority != "" && task.Priority != args.Priority {
						continue
					}
					filtered = append(filtered, task)
				}

				if len(filtered) == 0 {
					return agent.AgentToolResult{
						Content: []piai.ContentBlock{piai.TextContent{
							Type: "text",
							Text: "没有找到匹配的任务",
						}},
					}, nil
				}

				result := "任务列表：\n"
				for _, task := range filtered {
					result += fmt.Sprintf("\n- [%s] %s (优先级: %s, 状态: %s)",
						task.ID, task.Title, task.Priority, task.Status)
				}

				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: result,
					}},
				}, nil
			},
		},
		{
			Name:        "update_task_status",
			Description: "更新任务状态",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"task_id": {"type": "string", "description": "任务ID"},
					"status": {"type": "string", "enum": ["todo", "in_progress", "done"], "description": "新状态"}
				},
				"required": ["task_id", "status"]
			}`),
			Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
				var args struct {
					TaskID string `json:"task_id"`
					Status string `json:"status"`
				}
				json.Unmarshal(params, &args)

				for i, task := range tasks {
					if task.ID == args.TaskID {
						tasks[i].Status = args.Status
						return agent.AgentToolResult{
							Content: []piai.ContentBlock{piai.TextContent{
								Type: "text",
								Text: fmt.Sprintf("任务 %s 状态已更新为：%s", args.TaskID, args.Status),
							}},
						}, nil
					}
				}

				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: fmt.Sprintf("未找到任务：%s", args.TaskID),
					}},
					IsError: true,
				}, nil
			},
		},
	}

	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model: model,
			SystemPrompt: `你是一个任务管理助手。你可以帮助用户：
1. 创建任务 - 使用 create_task
2. 查看任务列表 - 使用 list_tasks
3. 更新任务状态 - 使用 update_task_status

请帮助用户高效管理他们的任务。`,
			Tools: tools,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: apiKey,
				},
			},
		},
	})

	return aiAgent, &tasks
}

func main() {
	taskAgent, tasks := createTaskManagerAgent("your-api-key")

	ctx := context.Background()

	// 创建一些任务
	fmt.Println("📝 创建任务...")
	taskAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "创建一个高优先级任务：完成项目报告，截止日期是明天",
	})

	taskAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "创建任务：回复客户邮件",
	})

	taskAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "创建任务：代码审查",
	})

	// 查看任务列表
	fmt.Println("\n📋 查看任务列表...")
	result, _ := taskAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "显示所有任务",
	})

	for _, msg := range result {
		if am, ok := msg.(piai.AssistantMessage); ok {
			for _, block := range am.Content {
				if text, ok := block.(piai.TextContent); ok {
					fmt.Println(text.Text)
				}
			}
		}
	}

	// 更新任务状态
	fmt.Println("\n✅ 更新任务状态...")
	taskAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "把第一个任务标记为进行中",
	})

	// 显示所有任务
	fmt.Printf("\n📊 所有任务 (%d 个):\n", len(*tasks))
	for _, task := range *tasks {
		fmt.Printf("- [%s] %s (状态: %s, 优先级: %s)\n",
			task.ID, task.Title, task.Status, task.Priority)
	}
}
```

---

## 代码助手

### 代码生成和解释 Agent

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	piai "pi-ai-go"
	"pi-ai-go/agent"
	_ "pi-ai-go/providers"
)

func createCodeAssistantAgent(apiKey string) *agent.Agent {
	model := piai.Model{
		ID:       "Qwen/Qwen2.5-7B-Instruct",
		API:      piai.APIOpenAICompletions,
		Provider: piai.ProviderDeepSeek,
		BaseURL:  "https://api.siliconflow.cn/v1",
		Input:    []piai.Modality{piai.ModalityText},
	}

	tools := []agent.AgentTool{
		{
			Name:        "generate_code",
			Description: "生成代码",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"language": {"type": "string", "description": "编程语言"},
					"description": {"type": "string", "description": "代码功能描述"},
					"complexity": {"type": "string", "enum": ["simple", "medium", "complex"], "description": "代码复杂度"}
				},
				"required": ["language", "description"]
			}`),
			Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
				var args struct {
					Language    string `json:"language"`
					Description string `json:"description"`
					Complexity  string `json:"complexity"`
				}
				json.Unmarshal(params, &args)

				// 模拟代码生成
				code := fmt.Sprintf("// %s 代码\n// 功能: %s\n// 语言: %s\n\n// TODO: 实现代码逻辑",
					args.Complexity, args.Description, args.Language)

				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: fmt.Sprintf("生成的代码：\n\n```%s\n%s\n```", args.Language, code),
					}},
				}, nil
			},
		},
		{
			Name:        "explain_code",
			Description: "解释代码",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"code": {"type": "string", "description": "要解释的代码"},
					"language": {"type": "string", "description": "代码语言"}
				},
				"required": ["code"]
			}`),
			Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
				var args struct {
					Code     string `json:"code"`
					Language string `json:"language"`
				}
				json.Unmarshal(params, &args)

				explanation := fmt.Sprintf("代码解释：\n\n这段代码使用 %s 语言编写。\n\n主要功能：...\n关键概念：...\n\n建议改进：...",
					args.Language)

				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: explanation,
					}},
				}, nil
			},
		},
		{
			Name:        "review_code",
			Description: "代码审查",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"code": {"type": "string", "description": "要审查的代码"},
					"focus": {"type": "string", "enum": ["performance", "security", "readability", "all"], "description": "审查重点"}
				},
				"required": ["code"]
			}`),
			Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
				var args struct {
					Code  string `json:"code"`
					Focus string `json:"focus"`
				}
				json.Unmarshal(params, &args)

				if args.Focus == "" {
					args.Focus = "all"
				}

				review := fmt.Sprintf("代码审查报告\n\n审查重点: %s\n\n优点：\n- ...\n\n问题：\n- ...\n\n改进建议：\n- ...",
					args.Focus)

				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: review,
					}},
				}, nil
			},
		},
	}

	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model: model,
			SystemPrompt: `你是一个专业的代码助手。你可以帮助用户：
1. 生成代码 - 使用 generate_code
2. 解释代码 - 使用 explain_code
3. 代码审查 - 使用 review_code

请提供高质量的代码建议和解释。`,
			Tools: tools,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: apiKey,
				},
			},
		},
	})

	return aiAgent
}

func main() {
	codeAgent := createCodeAssistantAgent("your-api-key")

	ctx := context.Background()

	// 代码生成
	fmt.Println("💻 生成代码...")
	result1, _ := codeAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "用 Python 写一个快速排序算法",
	})

	for _, msg := range result1 {
		if am, ok := msg.(piai.AssistantMessage); ok {
			for _, block := range am.Content {
				if text, ok := block.(piai.TextContent); ok {
					fmt.Println(text.Text)
				}
			}
		}
	}

	// 代码解释
	fmt.Println("\n📖 解释代码...")
	result2, _ := codeAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: `解释这段代码：
func fibonacci(n int) int {
    if n <= 1 {
        return n
    }
    return fibonacci(n-1) + fibonacci(n-2)
}`,
	})

	for _, msg := range result2 {
		if am, ok := msg.(piai.AssistantMessage); ok {
			for _, block := range am.Content {
				if text, ok := block.(piai.TextContent); ok {
					fmt.Println(text.Text)
				}
			}
		}
	}
}
```

---

## 最佳实践

### 1. 工具设计原则

```go
// ✅ 好的工具设计
goodTool := agent.AgentTool{
    Name:        "get_weather",           // 清晰的名称
    Description: "查询指定城市的当前天气信息，包括温度、湿度、风速等",  // 详细的描述
    Parameters: json.RawMessage(`{
        "type": "object",
        "properties": {
            "city": {
                "type": "string",
                "description": "城市名称，例如：北京、上海、深圳"
            }
        },
        "required": ["city"]
    }`),                                  // 完整的参数定义
    Execute: func(...) (...) {
        // 实现逻辑
    },
}

// ❌ 不好的工具设计
badTool := agent.AgentTool{
    Name:        "tool1",                 // 不清晰的名称
    Description: "查询天气",               // 太简短
    Parameters: json.RawMessage(`{}`),    // 没有参数定义
    Execute: func(...) (...) {
        // 实现逻辑
    },
}
```

### 2. 错误处理

```go
Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
    // 解析参数
    var args MyArgs
    if err := json.Unmarshal(params, &args); err != nil {
        return agent.AgentToolResult{
            Content: []piai.ContentBlock{piai.TextContent{
                Type: "text",
                Text: fmt.Sprintf("参数错误: %v", err),
            }},
            IsError: true,  // 标记为错误
        }, nil  // 返回 nil error，而不是 err
    }

    // 执行逻辑
    result, err := doSomething(args)
    if err != nil {
        return agent.AgentToolResult{
            Content: []piai.ContentBlock{piai.TextContent{
                Type: "text",
                Text: fmt.Sprintf("执行失败: %v", err),
            }},
            IsError: true,
        }, nil
    }

    return agent.AgentToolResult{
        Content: []piai.ContentBlock{piai.TextContent{
            Type: "text",
            Text: result,
        }},
    }, nil
}
```

### 3. 进度更新

```go
Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
    // 长时间运行的任务
    for i := 0; i < 100; i++ {
        // 检查是否被取消
        select {
        case <-ctx.Done():
            return agent.AgentToolResult{
                Content: []piai.ContentBlock{piai.TextContent{
                    Type: "text",
                    Text: "任务被取消",
                }},
                IsError: true,
            }, nil
        default:
        }

        // 发送进度更新
        progress := map[string]interface{}{
            "progress": i,
            "status":   "处理中...",
        }
        progressJSON, _ := json.Marshal(progress)
        onUpdate(progressJSON)

        // 执行任务
        time.Sleep(100 * time.Millisecond)
    }

    return agent.AgentToolResult{
        Content: []piai.ContentBlock{piai.TextContent{
            Type: "text",
            Text: "任务完成",
        }},
    }, nil
}
```

### 4. 上下文管理

```go
// 设置合理的超时
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()

// 在工具中检查上下文
Execute: func(ctx context.Context, ...) (...) {
    select {
    case <-ctx.Done():
        return agent.AgentToolResult{
            Content: []piai.ContentBlock{piai.TextContent{
                Type: "text",
                Text: "操作超时",
            }},
            IsError: true,
        }, nil
    default:
        // 继续执行
    }
    // ...
}
```

---

## 故障排除

### 常见问题

1. **工具没有被调用**
   - 检查 System Prompt 是否明确提到了工具
   - 确保工具描述清晰、准确
   - 验证参数定义是否完整

2. **工具执行超时**
   - 增加 context 超时时间
   - 优化工具执行逻辑
   - 添加进度更新

3. **参数解析失败**
   - 确保 Parameters JSON Schema 正确
   - 提供清晰的参数描述
   - 设置合理的默认值

4. **内存泄漏**
   - 及时释放资源
   - 使用 context 控制生命周期
   - 避免在工具中持有过多状态

---

## 更多资源

- [pi-ai-go 官方文档](../../README.md)
- [Agent 包 API 参考](../../agent/README.md)
- [测试用例](../../agent/agent_test.go)
- [Go 语言最佳实践](https://go.dev/doc/effective_go)
