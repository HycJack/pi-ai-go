// 这个文件展示了如何在其他Go项目中使用 Agent 工具调用功能
// 可以作为参考代码复制到你的项目中

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

// ExampleBasicUsage 基本使用示例
func ExampleBasicUsage() {
	// 1. 配置模型
	model := piai.Model{
		ID:            "Qwen/Qwen2.5-7B-Instruct",
		API:           piai.APIOpenAICompletions,
		Provider:      piai.ProviderDeepSeek,
		BaseURL:       "https://api.siliconflow.cn/v1",
		Input:         []piai.Modality{piai.ModalityText},
		ContextWindow: 64000,
		MaxTokens:     4096,
	}

	// 2. 定义工具
	tools := []agent.AgentTool{
		{
			Name:        "get_time",
			Description: "获取当前时间",
			Parameters:  json.RawMessage(`{}`),
			Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
				currentTime := time.Now().Format("2006-01-02 15:04:05")
				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: fmt.Sprintf("当前时间: %s", currentTime),
					}},
				}, nil
			},
		},
	}

	// 3. 创建 Agent
	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model:        model,
			SystemPrompt: "你是一个助手，可以告诉用户当前时间。",
			Tools:        tools,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: "your-api-key-here",
				},
			},
		},
	})

	// 4. 订阅事件（可选）
	aiAgent.Subscribe(func(evt agent.AgentEvent) {
		switch e := evt.(type) {
		case agent.EventToolExecStart:
			log.Printf("工具开始: %s", e.ToolName)
		case agent.EventToolExecEnd:
			log.Printf("工具完成: %s (错误: %v)", e.ToolName, e.IsError)
		}
	})

	// 5. 运行 Agent
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := aiAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "现在几点了？",
	})

	if err != nil {
		log.Fatalf("运行失败: %v", err)
	}

	// 6. 处理结果
	for _, msg := range result {
		if am, ok := msg.(piai.AssistantMessage); ok {
			for _, block := range am.Content {
				if text, ok := block.(piai.TextContent); ok {
					fmt.Println("回复:", text.Text)
				}
			}
		}
	}
}

// ExampleMultipleTurns 多轮对话示例
func ExampleMultipleTurns() {
	model := piai.Model{
		ID:            "Qwen/Qwen2.5-7B-Instruct",
		API:           piai.APIOpenAICompletions,
		Provider:      piai.ProviderDeepSeek,
		BaseURL:       "https://api.siliconflow.cn/v1",
		Input:         []piai.Modality{piai.ModalityText},
		ContextWindow: 64000,
		MaxTokens:     4096,
	}

	// 记忆工具 - 存储和检索信息
	memory := map[string]string{}
	
	tools := []agent.AgentTool{
		{
			Name:        "save_memory",
			Description: "保存信息到记忆",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"key": {"type": "string", "description": "记忆键"},
					"value": {"type": "string", "description": "记忆值"}
				},
				"required": ["key", "value"]
			}`),
			Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
				var args struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				}
				json.Unmarshal(params, &args)
				memory[args.Key] = args.Value
				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: fmt.Sprintf("已保存: %s = %s", args.Key, args.Value),
					}},
				}, nil
			},
		},
		{
			Name:        "get_memory",
			Description: "从记忆中检索信息",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"key": {"type": "string", "description": "记忆键"}
				},
				"required": ["key"]
			}`),
			Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
				var args struct {
					Key string `json:"key"`
				}
				json.Unmarshal(params, &args)
				value, exists := memory[args.Key]
				if !exists {
					return agent.AgentToolResult{
						Content: []piai.ContentBlock{piai.TextContent{
							Type: "text",
							Text: fmt.Sprintf("未找到键 '%s' 的记忆", args.Key),
						}},
					}, nil
				}
				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: fmt.Sprintf("%s = %s", args.Key, value),
					}},
				}, nil
			},
		},
	}

	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model:        model,
			SystemPrompt: `你是一个有记忆功能的助手。使用 save_memory 保存信息，使用 get_memory 检索信息。`,
			Tools:        tools,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: "your-api-key-here",
				},
			},
		},
	})

	ctx := context.Background()

	// 第一轮：保存信息
	fmt.Println("第一轮对话:")
	result1, _ := aiAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "记住我的名字是张三",
	})

	for _, msg := range result1 {
		if am, ok := msg.(piai.AssistantMessage); ok {
			for _, block := range am.Content {
				if text, ok := block.(piai.TextContent); ok {
					fmt.Println("助手:", text.Text)
				}
			}
		}
	}

	// 第二轮：检索信息
	fmt.Println("\n第二轮对话:")
	result2, _ := aiAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "我叫什么名字？",
	})

	for _, msg := range result2 {
		if am, ok := msg.(piai.AssistantMessage); ok {
			for _, block := range am.Content {
				if text, ok := block.(piai.TextContent); ok {
					fmt.Println("助手:", text.Text)
				}
			}
		}
	}
}

// ExampleToolWithHooks 带钩子的工具示例
func ExampleToolWithHooks() {
	model := piai.Model{
		ID:            "Qwen/Qwen2.5-7B-Instruct",
		API:           piai.APIOpenAICompletions,
		Provider:      piai.ProviderDeepSeek,
		BaseURL:       "https://api.siliconflow.cn/v1",
		Input:         []piai.Modality{piai.ModalityText},
		ContextWindow: 64000,
		MaxTokens:     4096,
	}

	tools := []agent.AgentTool{
		{
			Name:        "sensitive_operation",
			Description: "一个需要权限验证的敏感操作",
			Parameters:  json.RawMessage(`{"type": "object", "properties": {"action": {"type": "string"}}}`),
			Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: "操作已执行",
					}},
				}, nil
			},
		},
	}

	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model:        model,
			SystemPrompt: "你是一个助手。",
			Tools:        tools,
			// BeforeToolCall 钩子 - 在工具执行前验证
			BeforeToolCall: func(ctx agent.BeforeToolCallContext) *agent.ToolCallBlock {
				log.Printf("即将执行工具: %s", ctx.ToolCall.Name)
				
				// 示例：阻止敏感操作
				if ctx.ToolCall.Name == "sensitive_operation" {
					// 可以在这里添加权限检查逻辑
					log.Println("⚠️ 敏感操作被拦截")
					return &agent.ToolCallBlock{
						Block:  true,
						Reason: "敏感操作需要管理员权限",
					}
				}
				
				return nil // 允许执行
			},
			// AfterToolCall 钩子 - 在工具执行后处理
			AfterToolCall: func(ctx agent.AfterToolCallContext) *agent.ToolCallOverride {
				log.Printf("工具 %s 执行完成", ctx.ToolCall.Name)
				
				// 可以在这里修改工具结果
				if ctx.IsError {
					log.Printf("工具执行失败: %v", ctx.Result.Content)
				}
				
				return nil // 不修改结果
			},
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: "your-api-key-here",
				},
			},
		},
	})

	ctx := context.Background()
	result, _ := aiAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "执行敏感操作",
	})

	for _, msg := range result {
		if am, ok := msg.(piai.AssistantMessage); ok {
			for _, block := range am.Content {
				if text, ok := block.(piai.TextContent); ok {
					fmt.Println("回复:", text.Text)
				}
			}
		}
	}
}

// ExampleStreamingEvents 流式事件处理示例
func ExampleStreamingEvents() {
	model := piai.Model{
		ID:            "Qwen/Qwen2.5-7B-Instruct",
		API:           piai.APIOpenAICompletions,
		Provider:      piai.ProviderDeepSeek,
		BaseURL:       "https://api.siliconflow.cn/v1",
		Input:         []piai.Modality{piai.ModalityText},
		ContextWindow: 64000,
		MaxTokens:     4096,
	}

	tools := []agent.AgentTool{
		{
			Name:        "long_running_task",
			Description: "一个长时间运行的任务",
			Parameters:  json.RawMessage(`{}`),
			Execute: func(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (agent.AgentToolResult, error) {
				// 模拟长时间运行的任务，定期发送更新
				for i := 0; i < 5; i++ {
					time.Sleep(500 * time.Millisecond)
					
					// 发送进度更新
					progress := map[string]interface{}{
						"progress": (i + 1) * 20,
						"status":   fmt.Sprintf("处理中... %d%%", (i+1)*20),
					}
					progressJSON, _ := json.Marshal(progress)
					onUpdate(progressJSON)
				}

				return agent.AgentToolResult{
					Content: []piai.ContentBlock{piai.TextContent{
						Type: "text",
						Text: "任务完成！",
					}},
				}, nil
			},
		},
	}

	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model:        model,
			SystemPrompt: "你是一个助手，可以执行长时间任务。",
			Tools:        tools,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey: "your-api-key-here",
				},
			},
		},
	})

	// 订阅所有事件
	aiAgent.Subscribe(func(evt agent.AgentEvent) {
		switch e := evt.(type) {
		case agent.EventAgentStart:
			fmt.Println("🤖 Agent 开始")
		case agent.EventAgentEnd:
			fmt.Println("✅ Agent 结束")
		case agent.EventTurnStart:
			fmt.Println("🔄 新轮次开始")
		case agent.EventMessageStart:
			fmt.Print("💬 助手: ")
		case agent.EventMessageUpdate:
			for _, block := range e.Message.Content {
				if text, ok := block.(piai.TextContent); ok {
					fmt.Print(text.Text)
				}
			}
		case agent.EventMessageEnd:
			fmt.Println()
		case agent.EventToolExecStart:
			fmt.Printf("🔧 工具开始: %s\n", e.ToolName)
		case agent.EventToolExecUpdate:
			fmt.Printf("   ⏳ 进度: %s\n", string(e.PartialResult))
		case agent.EventToolExecEnd:
			fmt.Printf("   ✅ 工具完成: %s\n", e.ToolName)
		}
	})

	ctx := context.Background()
	_, err := aiAgent.Run(ctx, piai.UserMessage{
		Role:    "user",
		Content: "执行长时间任务",
	})

	if err != nil {
		log.Fatalf("运行失败: %v", err)
	}
}

func init() {
	// 这些函数仅作为示例，不会在main中运行
	_ = ExampleBasicUsage
	_ = ExampleMultipleTurns
	_ = ExampleToolWithHooks
	_ = ExampleStreamingEvents
}
