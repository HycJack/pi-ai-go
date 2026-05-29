package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

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

func main() {
	if err := loadEnv("../../.env"); err != nil {
		fmt.Printf("警告: 无法加载 .env 文件: %v\n", err)
	}

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

	fmt.Printf("模型: %s\n", modelID)
	fmt.Printf("API: %s\n\n", baseURL)
	fmt.Println("=== 流式输出测试 ===")
	fmt.Println("问题: 请用3句话介绍Go语言的优势")
	fmt.Println("---")

	stream, err := piai.StreamSimple(context.Background(), model, []piai.Message{
		piai.UserMessage{Content: "请用3句话介绍Go语言的优势"},
	}, piai.SimpleStreamOptions{
		StreamOptions: piai.StreamOptions{
			APIKey: apiKey,
		},
	})
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		return
	}

	// 使用 ForEach 逐个处理事件，实现流式输出
	_, err = stream.ForEach(context.Background(), func(evt piai.AssistantMessageEvent) error {
		switch e := evt.(type) {
		case piai.EventStart:
			fmt.Printf("[开始] 模型: %s, 提供者: %s\n", e.Model, e.Provider)
		case piai.EventTextDelta:
			// 流式输出文本片段，不换行
			fmt.Print(e.Delta)
		case piai.EventTextEnd:
			fmt.Println() // 文本块结束时换行
		case piai.EventThinkingDelta:
			// 思考过程（如果有的话）
			fmt.Printf("[思考] %s", e.Delta)
		case piai.EventDone:
			fmt.Printf("\n[完成] Token 用量: 输入=%d, 输出=%d\n",
				e.Message.Usage.Input, e.Message.Usage.Output)
			fmt.Printf("[完成] 停止原因: %s\n", e.Message.StopReason)
		case piai.EventError:
			fmt.Printf("\n[错误] %v\n", e.Error)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("\n流式处理错误: %v\n", err)
	}
}
