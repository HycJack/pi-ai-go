// Demo 01: 上下文压缩 (Compaction)
//
// 演示场景:
//   1. 模拟长对话，触发自动压缩
//   2. 手动触发压缩
//   3. 验证压缩后 Agent 仍能记住关键信息
//   4. 对比压缩前后的 token 使用
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"examples/agent-memory-demo/demos/internal/common"

	"pi-ai-go/agent"
	"pi-ai-go/agent/session"
	"pi-ai-go/core"
)

func main() {
	common.LoadEnv()
	common.PrintHeader("Demo 01: 上下文压缩 (Compaction)")

	apiKey := common.GetAPIKey("deepseek")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: API key not configured")
		os.Exit(1)
	}
	model := common.ResolveModel("deepseek", "deepseek-chat")

	storage := session.NewMemoryStorage()
	sess, _ := common.NewSession(storage, "压缩演示")
	defer sess.Close()
	common.PrintResult("会话 ID: %s", sess.ID())

	compactionCount := 0
	config := common.BuildAgentConfig(model, apiKey,
		&agent.ContextPolicy{
			SoftLimit:       0.50,
			HardLimit:       0.80,
			ReservedOutput:  1024,
			MinTailMessages: 4,
			Strategy:        agent.CompactionStrategySlidingWindow,
		},
		func(e agent.CompactionEvent) {
			compactionCount++
			common.PrintResult("压缩 #%d: 策略=%s 触发=%s 丢弃=%d token %d→%d",
				compactionCount, e.Strategy, e.TriggeredBy, e.Dropped, e.TokensBefore, e.TokensAfter)
		},
	)

	ctx := context.Background()
	questions := []string{
		"我正在开发一个电商网站，请记住这个项目背景。",
		"项目使用 Go 语言和 PostgreSQL 数据库。",
		"前端使用 React + TypeScript。",
		"后端采用微服务架构，包含用户服务、订单服务、支付服务。",
		"请基于以上信息，推荐一个项目目录结构。",
		"总结一下我刚才告诉你的项目信息。",
	}

	common.PrintStep("开始多轮对话 (激进压缩策略: soft=50%%, hard=80%%)")
	for i, q := range questions {
		fmt.Fprintf(os.Stderr, "\n🧑 Q%d: %s\n", i+1, q)

		common.AppendMessage(sess, core.UserMessage{
			Role: "user", Content: q, Timestamp: time.Now(),
		})

		ctx2 := sess.BuildContext()
		messages := ctx2.Messages
		messages = append(messages, core.UserMessage{
			Role: "user", Content: q, Timestamp: time.Now(),
		})

		runCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		stream, _ := agent.AgentLoopDetailed(runCtx, messages, config)

		var lastAssistant core.AssistantMessage
		_, _ = stream.ForEach(runCtx, func(evt agent.AgentEvent) error {
			if e, ok := evt.(agent.EventMessageEnd); ok {
				lastAssistant = e.Message
			}
			return nil
		})
		cancel()

		if lastAssistant.Role == "assistant" {
			common.AppendMessage(sess, lastAssistant)
		}

		common.PrintSessionStats(sess)
	}

	common.PrintStep("最终上下文状态")
	ctx3 := sess.BuildContext()
	common.PrintResult("消息数: %d", len(ctx3.Messages))
	common.PrintResult("压缩次数: %d", compactionCount)
}
