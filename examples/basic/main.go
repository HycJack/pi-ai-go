package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"pi-ai-go/core"
	"pi-ai-go/llm"
	_ "pi-ai-go/providers"
)

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
	if err := loadEnv("../.env"); err != nil {
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

	model := core.Model{
		ID:            modelID,
		API:           core.APIOpenAICompletions,
		Provider:      core.ProviderDeepSeek,
		BaseURL:       baseURL,
		Input:         []core.Modality{core.ModalityText},
		ContextWindow: 64000,
		MaxTokens:     4096,
		Cost: core.Cost{
			Input:  0.14,
			Output: 0.28,
		},
	}

	fmt.Printf("模型: %s\n", modelID)
	fmt.Printf("API: %s\n\n", baseURL)

	fmt.Println("=== 测试普通请求 ===")
	msg, err := llm.CompleteSimple(context.Background(), model, []core.Message{
		core.UserMessage{Content: "你好，请用一句话介绍自己"},
	}, core.SimpleStreamOptions{
		StreamOptions: core.StreamOptions{
			APIKey: apiKey,
		},
	})
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		return
	}

	for _, block := range msg.Content {
		if text, ok := block.(core.TextContent); ok {
			fmt.Println(text.Text)
		}
	}

	fmt.Printf("\nToken 用量: 输入=%d, 输出=%d\n", msg.Usage.Input, msg.Usage.Output)
}
