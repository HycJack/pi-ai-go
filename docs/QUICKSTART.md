# 快速开始指南

本指南帮助你在 5 分钟内开始使用 pi-ai-go。

## 前置条件

- Go 1.23 或更高版本
- 任一 AI 服务商的 API Key

## 安装

```bash
go get pi-ai-go
```

## 配置环境变量

### 方式一：使用 .env 文件（推荐）

```bash
# 复制配置模板
cp .env.example .env

# 编辑配置文件，填入你的 API Key
vim .env
```

`.env` 文件示例：
```bash
# OpenAI
OPENAI_API_KEY=sk-xxx

# 或者使用 SiliconFlow（国内访问友好）
SILICONFLOW_API_KEY=sk-xxx
SILICONFLOW_BASE_URL=https://api.siliconflow.cn/v1
SILICONFLOW_MODEL=Qwen/Qwen2.5-7B-Instruct
```

### 方式二：设置环境变量

```bash
export OPENAI_API_KEY="sk-xxx"
```

## 第一个程序

创建 `main.go`：

```go
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	piai "pi-ai-go"
	_ "pi-ai-go/providers" // 注册内置服务商
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
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if os.Getenv(key) == "" {
				os.Setenv(key, value)
			}
		}
	}
	return scanner.Err()
}

func main() {
	// 加载 .env 文件
	loadEnv("../.env")

	// 获取 API Key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("SILICONFLOW_API_KEY")
	}

	// 选择模型
	model, _ := piai.GetModel(piai.ProviderOpenAI, "gpt-4o")
	
	// 或者使用自定义模型
	if os.Getenv("SILICONFLOW_API_KEY") != "" {
		model = piai.Model{
			ID:            os.Getenv("SILICONFLOW_MODEL"),
			API:           piai.APIOpenAICompletions,
			Provider:      piai.ProviderDeepSeek,
			BaseURL:       os.Getenv("SILICONFLOW_BASE_URL"),
			Input:         []piai.Modality{piai.ModalityText},
			ContextWindow: 64000,
			MaxTokens:     4096,
		}
	}

	// 发送请求
	msg, err := piai.CompleteSimple(context.Background(), model, []piai.Message{
		piai.UserMessage{Content: "你好，请用一句话介绍自己"},
	}, piai.SimpleStreamOptions{
		StreamOptions: piai.StreamOptions{
			APIKey: apiKey,
		},
	})
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		return
	}

	// 输出结果
	for _, block := range msg.Content {
		if text, ok := block.(piai.TextContent); ok {
			fmt.Println(text.Text)
		}
	}
}
```

## 运行

```bash
go run main.go
```

## 流式输出示例

```go
package main

import (
	"context"
	"fmt"

	piai "pi-ai-go"
	_ "pi-ai-go/providers"
)

func main() {
	model, _ := piai.GetModel(piai.ProviderOpenAI, "gpt-4o")

	stream, err := piai.StreamSimple(context.Background(), model, []piai.Message{
		piai.UserMessage{Content: "写一首关于春天的诗"},
	}, piai.SimpleStreamOptions{
		StreamOptions: piai.StreamOptions{
			APIKey: "your-api-key",
		},
	})
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		return
	}

	// 处理流式事件
	for event := range stream.Events() {
		switch e := event.(type) {
		case piai.EventTextDelta:
			fmt.Print(e.Delta) // 实时输出
		case piai.EventDone:
			fmt.Printf("\n\n完成！Token 用量: %d\n", e.Message.Usage.TotalTokens)
		case piai.EventError:
			fmt.Printf("\n错误: %v\n", e.Error)
		}
	}
}
```

## 工具调用示例

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"

	piai "pi-ai-go"
	_ "pi-ai-go/providers"
)

func main() {
	model, _ := piai.GetModel(piai.ProviderOpenAI, "gpt-4o")

	// 定义工具
	tools := []piai.Tool{
		{
			Name:        "get_weather",
			Description: "获取指定城市的天气信息",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"city": {
						"type": "string",
						"description": "城市名称"
					}
				},
				"required": ["city"]
			}`),
		},
	}

	// 发送带工具的请求
	ctx := context.Background()
	messages := []piai.Message{
		piai.UserMessage{Content: "北京今天天气怎么样？"},
	}

	stream, _ := piai.Stream(ctx, model, messages, piai.StreamOptions{
		APIKey: "your-api-key",
	})

	for event := range stream.Events() {
		switch e := event.(type) {
		case piai.EventToolCallEnd:
			// 解析工具参数
			var args struct {
				City string `json:"city"`
			}
			json.Unmarshal(e.Arguments, &args)
			fmt.Printf("调用工具: %s, 城市: %s\n", e.ID, args.City)

			// 添加工具结果
			messages = append(messages, piai.ToolResultMessage{
				ToolCallID: e.ID,
				ToolName:   "get_weather",
				Content: []piai.ContentBlock{
					piai.TextContent{Text: "晴天，25°C"},
				},
			})

		case piai.EventDone:
			fmt.Println("完成")
		}
	}
}
```

## 多服务商切换

只需修改模型配置即可切换服务商：

```go
// OpenAI
model, _ := piai.GetModel(piai.ProviderOpenAI, "gpt-4o")

// Anthropic
model, _ := piai.GetModel(piai.ProviderAnthropic, "claude-3-opus")

// Google Gemini
model, _ := piai.GetModel(piai.ProviderGoogle, "gemini-pro")

// 自定义服务商（如 DeepSeek）
model := piai.Model{
	ID:       "deepseek-chat",
	API:      piai.APIOpenAICompletions,
	Provider: piai.ProviderDeepSeek,
	BaseURL:  "https://api.deepseek.com/v1",
}
```

## 常见问题

### Q: 如何查看支持的模型？

```go
providers := piai.GetProviders()
for _, p := range providers {
    models := piai.GetModels(p)
    for _, m := range models {
        fmt.Printf("%s/%s\n", p, m.ID)
    }
}
```

### Q: 如何设置代理？

```bash
# 在 .env 文件中添加
HTTP_PROXY=http://proxy:8080
HTTPS_PROXY=http://proxy:8080
```

### Q: 如何处理超时？

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

msg, err := piai.Complete(ctx, model, messages, piai.StreamOptions{
    APIKey: apiKey,
})
```

### Q: 如何取消请求？

```go
ctx, cancel := context.WithCancel(context.Background())

go func() {
    time.Sleep(5 * time.Second)
    cancel() // 5秒后取消
}()

stream, _ := piai.Stream(ctx, model, messages)
```

## 下一步

- 阅读 [API 参考文档](./API.md) 了解完整 API
- 阅读 [架构设计文档](./DESIGN.md) 了解内部实现
- 查看 `test/` 目录中的示例代码
