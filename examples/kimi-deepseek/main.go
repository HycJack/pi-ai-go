/*
 * 功能说明：Kimi (Moonshot) 和 DeepSeek API 调用示例
 *
 * 测试内容：
 * 1. Kimi API 流式调用（Stream）
 * 2. Kimi API 非流式调用（Complete）
 * 3. DeepSeek API 流式调用（Stream）
 * 4. DeepSeek API 非流式调用（Complete）
 *
 * 环境变量配置（.env 文件）：
 *   KIMI_API_KEY      - Moonshot Kimi 的 API Key
 *   KIMI_BASE_URL     - Kimi API 端点（默认 https://api.moonshot.ai/v1）
 *   KIMI_MODEL        - Kimi 模型名称（默认 moonshot-v1-8k）
 *   DEEPSEEK_API_KEY  - DeepSeek 的 API Key
 *   DEEPSEEK_BASE_URL - DeepSeek API 端点（默认 https://api.deepseek.com/v1）
 *   DEEPSEEK_MODEL    - DeepSeek 模型名称（默认 deepseek-chat）
 *
 * 使用方法：
 *   go run main.go
 */
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	piai "pi-ai-go"
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
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

// printSection 打印章节标题
func printSection(title string) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  %s\n", title)
	fmt.Println(strings.Repeat("=", 60))
}

// printSubsection 打印小节标题
func printSubsection(title string) {
	fmt.Println()
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("  %s\n", title)
	fmt.Println(strings.Repeat("-", 50))
}

// buildKimiModel 构建 Kimi 模型配置
func buildKimiModel() piai.Model {
	baseURL := os.Getenv("KIMI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.moonshot.cn/v1"
	}
	modelID := os.Getenv("KIMI_MODEL")
	if modelID == "" {
		modelID = "kimi-k2.6"
	}

	return piai.Model{
		ID:            modelID,
		API:           piai.APIOpenAICompletions, // Kimi 使用 OpenAI 兼容协议
		Provider:      piai.ProviderKimi,
		BaseURL:       baseURL,
		Input:         []piai.Modality{piai.ModalityText},
		ContextWindow: 8000,
		MaxTokens:     2048,
		Cost: piai.Cost{
			Input:  1.0,
			Output: 1.0,
		},
	}
}

// buildDeepSeekModel 构建 DeepSeek 模型配置
func buildDeepSeekModel() piai.Model {
	baseURL := os.Getenv("DEEPSEEK_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}
	modelID := os.Getenv("DEEPSEEK_MODEL")
	if modelID == "" {
		modelID = "deepseek-v4-flash"
	}

	return piai.Model{
		ID:            modelID,
		API:           piai.APIOpenAICompletions, // DeepSeek 使用 OpenAI 兼容协议
		Provider:      piai.ProviderDeepSeek,
		BaseURL:       baseURL,
		Input:         []piai.Modality{piai.ModalityText},
		ContextWindow: 64000,
		MaxTokens:     4096,
		Cost: piai.Cost{
			Input:  0.14, // 缓存未命中价格（单位：元/百万 token）
			Output: 0.28,
		},
	}
}

// buildXiaomiModel 构建小米 Mimo 模型配置
func buildXiaomiModel() piai.Model {
	baseURL := os.Getenv("XIAOMI_BASE_URL")
	if baseURL == "" {
		// baseURL = "https://api.xiaomimimo.com/v1"
		baseURL = "https://token-plan-cn.xiaomimimo.com/v1"
	}
	modelID := os.Getenv("XIAOMI_MODEL")
	if modelID == "" {
		modelID = "mimo-v2.5-pro"
	}

	return piai.Model{
		ID:            modelID,
		API:           piai.APIOpenAICompletions, // 小米使用 OpenAI 兼容协议
		Provider:      piai.ProviderXiaomi,
		BaseURL:       baseURL,
		Input:         []piai.Modality{piai.ModalityText},
		ContextWindow: 128000,
		MaxTokens:     4096,
		Cost: piai.Cost{
			Input:  0.1, // 单位：元/百万 token（示例价格）
			Output: 0.2,
		},
	}
}

// testStream 测试流式调用
func testStream(ctx context.Context, name string, model piai.Model, apiKey, prompt string) error {
	printSubsection(fmt.Sprintf("%s - 流式调用", name))
	fmt.Printf("模型: %s\n", model.ID)
	fmt.Printf("API: %s\n", model.BaseURL)
	fmt.Printf("提示: %s\n", prompt)
	fmt.Println()
	fmt.Print("回复: ")

	start := time.Now()

	// 调试：记录原始响应
	var rawResponses []string

	stream, err := piai.StreamSimple(ctx, model, []piai.Message{
		piai.UserMessage{Content: prompt},
	}, piai.SimpleStreamOptions{
		StreamOptions: piai.StreamOptions{
			APIKey: apiKey,
			OnResponse: func(data any) {
				if s, ok := data.(string); ok {
					rawResponses = append(rawResponses, s)
				}
			},
		},
	})
	if err != nil {
		return fmt.Errorf("创建流失败: %w", err)
	}

	var fullText strings.Builder
	var thinkingPrinted bool // 标记思考信息是否已打印

	// 使用 ForEach 逐个处理事件，实现流式输出
	_, err = stream.ForEach(ctx, func(evt piai.AssistantMessageEvent) error {
		switch e := evt.(type) {
		case piai.EventStart:
			fmt.Printf("\n[开始] 模型: %s, 提供者: %s\n", e.Model, e.Provider)
		case piai.EventTextDelta:
			// 流式输出文本片段
			fmt.Print(e.Delta)
			fullText.WriteString(e.Delta)
		case piai.EventTextEnd:
			fmt.Println() // 文本块结束换行
		case piai.EventThinkingDelta:
			// 思考过程（DeepSeek-R1 会触发），只打印一次
			if !thinkingPrinted {
				fmt.Printf("[思考] %s", e.Delta)
				thinkingPrinted = true
			} else {
				fmt.Printf("%s", e.Delta)
			}
		case piai.EventDone:
			elapsed := time.Since(start)
			fmt.Printf("\n[完成] Token 用量: 输入=%d, 输出=%d\n",
				e.Message.Usage.Input, e.Message.Usage.Output)
			fmt.Printf("[完成] 费用: ¥%.6f\n", e.Message.Usage.Cost.Total)
			fmt.Printf("[完成] 停止原因: %s\n", e.Message.StopReason)
			fmt.Printf("[完成] 耗时: %v\n", elapsed)
			fmt.Printf("[完成] 完整内容长度: %d 字符\n", fullText.Len())

			// 调试：显示原始响应中的 usage
			fmt.Println("\n[调试] 原始响应中的 usage 信息:")
			for i, resp := range rawResponses {
				if strings.Contains(resp, "usage") {
					fmt.Printf("  [%d] %s\n", i, resp)
				}
			}
		case piai.EventError:
			fmt.Printf("\n[错误] %v\n", e.Error)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("流处理错误: %w", err)
	}
	return nil
}

// testComplete 测试非流式调用
func testComplete(ctx context.Context, name string, model piai.Model, apiKey, prompt string) error {
	printSubsection(fmt.Sprintf("%s - 非流式调用", name))
	fmt.Printf("模型: %s\n", model.ID)
	fmt.Printf("API: %s\n", model.BaseURL)
	fmt.Printf("提示: %s\n", prompt)
	fmt.Println()

	start := time.Now()

	msg, err := piai.CompleteSimple(ctx, model, []piai.Message{
		piai.UserMessage{Content: prompt},
	}, piai.SimpleStreamOptions{
		StreamOptions: piai.StreamOptions{
			APIKey: apiKey,
		},
	})
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	elapsed := time.Since(start)

	fmt.Println("回复:")
	for _, block := range msg.Content {
		switch b := block.(type) {
		case piai.TextContent:
			fmt.Println(b.Text)
		case piai.ThinkingContent:
			fmt.Printf("[思考] %s\n", b.Thinking)
		}
	}

	fmt.Println()
	fmt.Printf("Token 用量: 输入=%d, 输出=%d\n", msg.Usage.Input, msg.Usage.Output)
	fmt.Printf("费用: ¥%.6f\n", msg.Usage.Cost.Total)
	fmt.Printf("停止原因: %s\n", msg.StopReason)
	fmt.Printf("耗时: %v\n", elapsed)

	return nil
}

func main() {
	// 加载 .env 文件
	if err := loadEnv(".env"); err != nil {
		fmt.Printf("警告: 无法加载 .env 文件: %v\n", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// ==================== Kimi 测试 ====================
	printSection("Kimi (Moonshot) API 测试")

	kimiAPIKey := os.Getenv("KIMI_API_KEY")
	if kimiAPIKey == "" {
		fmt.Println("⚠ 跳过 Kimi 测试: 未设置 KIMI_API_KEY")
	} else {
		kimiModel := buildKimiModel()
		kimiPrompt := "请用2句话介绍Moonshot的Kimi模型。"

		// 测试 1: Kimi 流式调用
		if err := testStream(ctx, "Kimi", kimiModel, kimiAPIKey, kimiPrompt); err != nil {
			fmt.Printf("Kimi 流式调用失败: %v\n", err)
		}

		// 测试 2: Kimi 非流式调用
		if err := testComplete(ctx, "Kimi", kimiModel, kimiAPIKey, kimiPrompt); err != nil {
			fmt.Printf("Kimi 非流式调用失败: %v\n", err)
		}
	}

	// ==================== DeepSeek 测试 ====================
	printSection("DeepSeek API 测试")

	deepseekAPIKey := os.Getenv("DEEPSEEK_API_KEY")
	if deepseekAPIKey == "" {
		fmt.Println("⚠ 跳过 DeepSeek 测试: 未设置 DEEPSEEK_API_KEY")
	} else {
		deepseekModel := buildDeepSeekModel()
		deepseekPrompt := "请用2句话介绍DeepSeek的优势。"

		// 测试 3: DeepSeek 流式调用
		if err := testStream(ctx, "DeepSeek", deepseekModel, deepseekAPIKey, deepseekPrompt); err != nil {
			fmt.Printf("DeepSeek 流式调用失败: %v\n", err)
		}

		// 测试 4: DeepSeek 非流式调用
		if err := testComplete(ctx, "DeepSeek", deepseekModel, deepseekAPIKey, deepseekPrompt); err != nil {
			fmt.Printf("DeepSeek 非流式调用失败: %v\n", err)
		}

		// 测试 5: DeepSeek 多轮对话
		printSubsection("DeepSeek - 多轮对话测试")
		fmt.Println("第一轮: 你好")
		fmt.Println("第二轮: 1+1等于几？")
		fmt.Println("第三轮: 那个结果乘以2呢？")
		fmt.Println()

		msg1, err := piai.CompleteSimple(ctx, deepseekModel, []piai.Message{
			piai.UserMessage{Content: "你好"},
		}, piai.SimpleStreamOptions{
			StreamOptions: piai.StreamOptions{
				APIKey: deepseekAPIKey,
			},
		})
		if err != nil {
			fmt.Printf("第一轮失败: %v\n", err)
		} else {
			for _, block := range msg1.Content {
				if text, ok := block.(piai.TextContent); ok {
					fmt.Printf("助手: %s\n", text.Text)
				}
			}
		}

		msg2, err := piai.CompleteSimple(ctx, deepseekModel, []piai.Message{
			piai.UserMessage{Content: "你好"},
			piai.AssistantMessage{
				Role:    "assistant",
				Content: msg1.Content,
			},
			piai.UserMessage{Content: "1+1等于几？"},
		}, piai.SimpleStreamOptions{
			StreamOptions: piai.StreamOptions{
				APIKey: deepseekAPIKey,
			},
		})
		if err != nil {
			fmt.Printf("第二轮失败: %v\n", err)
		} else {
			for _, block := range msg2.Content {
				if text, ok := block.(piai.TextContent); ok {
					fmt.Printf("助手: %s\n", text.Text)
				}
			}
		}
	}

	// ==================== 小米 Mimo 测试 ====================
	printSection("小米 Mimo API 测试")

	xiaomiAPIKey := os.Getenv("XIAOMI_API_KEY")
	if xiaomiAPIKey == "" {
		fmt.Println("⚠ 跳过小米测试: 未设置 XIAOMI_API_KEY")
	} else {
		xiaomiModel := buildXiaomiModel()
		xiaomiPrompt := "请用2句话介绍小米Mimo模型。"

		// 测试 6: 小米 Mimo 流式调用
		if err := testStream(ctx, "小米 Mimo", xiaomiModel, xiaomiAPIKey, xiaomiPrompt); err != nil {
			fmt.Printf("小米 Mimo 流式调用失败: %v\n", err)
		}

		// 测试 7: 小米 Mimo 非流式调用
		if err := testComplete(ctx, "小米 Mimo", xiaomiModel, xiaomiAPIKey, xiaomiPrompt); err != nil {
			fmt.Printf("小米 Mimo 非流式调用失败: %v\n", err)
		}
	}

	// ==================== 总结 ====================
	printSection("测试完成")
	fmt.Println("所有测试已执行完毕。")
	fmt.Println()
	fmt.Println("配置提示:")
	fmt.Println("  在项目根目录的 .env 文件中设置以下环境变量：")
	fmt.Println("    KIMI_API_KEY=your-kimi-key")
	fmt.Println("    DEEPSEEK_API_KEY=your-deepseek-key")
	fmt.Println("    XIAOMI_API_KEY=your-xiaomi-key")
	fmt.Println()
	fmt.Println("可选配置:")
	fmt.Println("    KIMI_MODEL=kimi-k2.6 (默认)")
	fmt.Println("    DEEPSEEK_MODEL=deepseek-v4-flash (默认)")
	fmt.Println("    XIAOMI_MODEL=mimo-v2.5-pro (默认)")
}
