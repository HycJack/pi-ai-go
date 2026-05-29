package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	piai "pi-ai-go"
	"pi-ai-go/agent"
	_ "pi-ai-go/providers"
)

func getTestConfig() (piai.Model, string) {
	// 加载环境变量
	loadEnv("../.env")

	apiKey := os.Getenv("SILICONFLOW_API_KEY")
	baseURL := os.Getenv("SILICONFLOW_BASE_URL")
	modelID := os.Getenv("SILICONFLOW_MODEL")

	if apiKey == "" {
		fmt.Println("错误: 请在 .env 文件中设置 SILICONFLOW_API_KEY")
		os.Exit(1)
	}
	if baseURL == "" {
		baseURL = "https://api.siliconflow.cn/v1"
	}
	if modelID == "" {
		modelID = "Qwen/Qwen2.5-7B-Instruct"
	}

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

	return model, apiKey
}

func TestCalculatorTool(t *testing.T) {
	model, apiKey := getTestConfig()

	tools := []agent.AgentTool{
		calculatorTool(),
	}

	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model:  model,
			SystemPrompt: `你是一个数学计算助手。使用 calculator 工具来计算数学表达式。`,
			Tools: tools,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: apiKey,
				},
			},
		},
	})

	// 订阅事件
	aiAgent.Subscribe(func(evt agent.AgentEvent) {
		switch e := evt.(type) {
		case agent.EventToolExecStart:
			fmt.Printf("🔧 工具开始: %s\n", e.ToolName)
		case agent.EventToolExecEnd:
			fmt.Printf("✅ 工具完成: %s\n", e.ToolName)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("\n测试计算器工具...")
	result, err := aiAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "计算 123 + 456 * 789",
	})

	if err != nil {
		t.Fatalf("运行失败: %v", err)
	}

	// 打印最终回复
	fmt.Println("\n最终回复:")
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

func TestWeatherTool(t *testing.T) {
	model, apiKey := getTestConfig()

	tools := []agent.AgentTool{
		weatherTool(),
	}

	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model:  model,
			SystemPrompt: `你是一个天气查询助手。使用 weather 工具来查询天气信息。`,
			Tools: tools,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: apiKey,
				},
			},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("\n测试天气查询工具...")
	result, err := aiAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "查询北京的天气",
	})

	if err != nil {
		t.Fatalf("运行失败: %v", err)
	}

	// 打印最终回复
	fmt.Println("\n最终回复:")
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

func TestMultipleTools(t *testing.T) {
	model, apiKey := getTestConfig()

	tools := []agent.AgentTool{
		calculatorTool(),
		weatherTool(),
		databaseQueryTool(),
	}

	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model:  model,
			SystemPrompt: `你是一个多功能助手。可以使用以下工具：
1. calculator - 数学计算
2. weather - 天气查询
3. database_query - 数据库查询

根据用户问题选择合适的工具。`,
			Tools: tools,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: apiKey,
				},
			},
		},
	})

	// 记录工具调用
	toolCalls := []string{}
	aiAgent.Subscribe(func(evt agent.AgentEvent) {
		if e, ok := evt.(agent.EventToolExecStart); ok {
			toolCalls = append(toolCalls, e.ToolName)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("\n测试多工具调用...")
	result, err := aiAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "帮我计算 2 的 10 次方，然后查询上海的天气",
	})

	if err != nil {
		t.Fatalf("运行失败: %v", err)
	}

	// 验证工具调用
	fmt.Printf("\n工具调用统计: %v\n", toolCalls)

	// 打印最终回复
	fmt.Println("\n最终回复:")
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

func TestDatabaseQuery(t *testing.T) {
	model, apiKey := getTestConfig()

	tools := []agent.AgentTool{
		databaseQueryTool(),
	}

	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model:  model,
			SystemPrompt: `你是一个数据库查询助手。使用 database_query 工具来查询数据。`,
			Tools: tools,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: apiKey,
				},
			},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("\n测试数据库查询工具...")
	result, err := aiAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "查询用户信息",
	})

	if err != nil {
		t.Fatalf("运行失败: %v", err)
	}

	// 打印最终回复
	fmt.Println("\n最终回复:")
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

func TestSearchTool(t *testing.T) {
	model, apiKey := getTestConfig()

	tools := []agent.AgentTool{
		searchTool(),
	}

	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model:  model,
			SystemPrompt: `你是一个搜索助手。使用 search 工具来搜索信息。`,
			Tools: tools,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: apiKey,
				},
			},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("\n测试搜索工具...")
	result, err := aiAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "搜索机器学习的最新进展",
	})

	if err != nil {
		t.Fatalf("运行失败: %v", err)
	}

	// 打印最终回复
	fmt.Println("\n最终回复:")
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

func TestToolErrorHandling(t *testing.T) {
	model, apiKey := getTestConfig()

	// 创建一个会失败的工具
	failingTool := agent.AgentTool{
		Name:        "failing_tool",
		Description: "一个会失败的工具",
		Parameters: json.RawMessage(`{}`),
		Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
			return agent.AgentToolResult{}, fmt.Errorf("工具执行失败")
		},
	}

	tools := []agent.AgentTool{failingTool}

	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model:  model,
			SystemPrompt: `使用 failing_tool 工具。`,
			Tools: tools,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: apiKey,
				},
			},
		},
	})

	// 记录错误
	var toolError bool
	aiAgent.Subscribe(func(evt agent.AgentEvent) {
		if e, ok := evt.(agent.EventToolExecEnd); ok {
			if e.IsError {
				toolError = true
				fmt.Printf("❌ 捕获到工具错误: %s\n", string(e.Result))
			}
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("\n测试工具错误处理...")
	result, err := aiAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "使用工具",
	})

	if err != nil {
		t.Fatalf("运行失败: %v", err)
	}

	if !toolError {
		t.Error("应该捕获到工具错误")
	}

	// 打印最终回复
	fmt.Println("\n最终回复:")
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

func TestAgentStateManagement(t *testing.T) {
	model, apiKey := getTestConfig()

	tools := []agent.AgentTool{
		calculatorTool(),
	}

	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model:  model,
			SystemPrompt: `你是一个数学助手。`,
			Tools: tools,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: apiKey,
				},
			},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("\n测试Agent状态管理...")

	// 第一轮对话
	fmt.Println("第一轮对话:")
	result1, err := aiAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "计算 10 + 20",
	})

	if err != nil {
		t.Fatalf("第一轮运行失败: %v", err)
	}

	// 打印消息历史
	messages1 := aiAgent.Messages()
	fmt.Printf("消息数量: %d\n", len(messages1))

	// 第二轮对话
	fmt.Println("\n第二轮对话:")
	result2, err := aiAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "把结果乘以3",
	})

	if err != nil {
		t.Fatalf("第二轮运行失败: %v", err)
	}

	// 打印消息历史
	messages2 := aiAgent.Messages()
	fmt.Printf("消息数量: %d\n", len(messages2))

	// 打印最终回复
	fmt.Println("\n最终回复:")
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

func TestParallelToolExecution(t *testing.T) {
	model, apiKey := getTestConfig()

	tools := []agent.AgentTool{
		calculatorTool(),
		weatherTool(),
	}

	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model:  model,
			SystemPrompt: `你是一个多功能助手。可以同时使用多个工具。`,
			Tools: tools,
			ToolExecution: agent.ToolExecParallel,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: apiKey,
				},
			},
		},
	})

	// 记录工具调用时间
	toolTimes := map[string]time.Time{}
	aiAgent.Subscribe(func(evt agent.AgentEvent) {
		switch e := evt.(type) {
		case agent.EventToolExecStart:
			toolTimes[e.ToolName] = time.Now()
		case agent.EventToolExecEnd:
			startTime := toolTimes[e.ToolName]
			fmt.Printf("🔧 工具 %s 耗时: %v\n", e.ToolName, time.Since(startTime))
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("\n测试并行工具执行...")
	result, err := aiAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "同时查询北京天气和计算 2 的 10 次方",
	})

	if err != nil {
		t.Fatalf("运行失败: %v", err)
	}

	// 打印最终回复
	fmt.Println("\n最终回复:")
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

// TestRunAll 运行所有测试
func TestRunAll(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过完整测试")
	}

	tests := []struct {
		name string
		fn   func(*testing.T)
	}{
		{"Calculator", TestCalculatorTool},
		{"Weather", TestWeatherTool},
		{"MultipleTools", TestMultipleTools},
		{"DatabaseQuery", TestDatabaseQuery},
		{"Search", TestSearchTool},
		{"ToolError", TestToolErrorHandling},
		{"StateManagement", TestAgentStateManagement},
		{"ParallelExecution", TestParallelToolExecution},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fmt.Printf("\n%s\n", strings.Repeat("=", 60))
			fmt.Printf("测试: %s\n", tt.name)
			fmt.Printf("%s\n", strings.Repeat("=", 60))
			tt.fn(t)
		})
	}
}
