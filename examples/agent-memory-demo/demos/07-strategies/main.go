// Demo 07: 压缩策略对比 (Compaction Strategies)
//
// 演示场景:
//   1. 对比 sliding_window 和 summarize 两种策略
//   2. sliding_window: 丢弃旧消息，保留尾部
//   3. summarize: LLM 生成摘要，保留关键信息
//   4. 验证两种策略压缩后 Agent 的记忆保持
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
	common.PrintHeader("Demo 07: 压缩策略对比")

	apiKey := common.GetAPIKey("deepseek")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: API key not configured")
		os.Exit(1)
	}
	model := common.ResolveModel("deepseek", "deepseek-chat")

	// ── 测试 sliding_window 策略 ──
	common.PrintStep("策略 A: sliding_window (滑动窗口)")
	testStrategy(model, apiKey, "sliding_window", agent.CompactionStrategySlidingWindow, nil)

	// ── 测试 summarize 策略 ──
	common.PrintStep("策略 B: summarize (LLM 摘要)")
	testStrategy(model, apiKey, "summarize", agent.CompactionStrategySummarize, &agent.SummarizeModel{
		Model: model,
		Stream: nil, // 使用默认
	})
}

func testStrategy(model core.Model, apiKey, name string, strategy agent.CompactionStrategy, sm *agent.SummarizeModel) {
	storage := session.NewMemoryStorage()
	sess, _ := common.NewSession(storage, name)
	defer sess.Close()

	config := common.BuildAgentConfig(model, apiKey,
		&agent.ContextPolicy{
			SoftLimit:       0.30,
			HardLimit:       0.50,
			ReservedOutput:  512,
			MinTailMessages: 2,
			Strategy:        strategy,
		},
		func(e agent.CompactionEvent) {
			common.PrintResult("  [%s] 压缩: 策略=%s 触发=%s 丢弃=%d token %d→%d",
				name, e.Strategy, e.TriggeredBy, e.Dropped, e.TokensBefore, e.TokensAfter)
		},
	)

	// 构造关键信息
	keyInfo := []string{
		"我的名字是张三。",
		"我在北京工作。",
		"我的职业是软件工程师。",
		"我擅长 Go 和 Python。",
	}

	ctx := context.Background()
	common.PrintResult("[%s] 写入关键信息...", name)
	for _, info := range keyInfo {
		common.AppendMessage(sess, core.UserMessage{
			Role: "user", Content: info, Timestamp: time.Now(),
		})

		ctx2 := sess.BuildContext()
		messages := append(ctx2.Messages, core.UserMessage{
			Role: "user", Content: info, Timestamp: time.Now(),
		})

		runCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
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
	}

	// 触发压缩
	common.PrintResult("[%s] 写入更多消息触发压缩...", name)
	for i := 0; i < 3; i++ {
		q := fmt.Sprintf("补充信息 %d: 我喜欢 %s。", i+1, []string{"阅读", "编程", "音乐"}[i])
		common.AppendMessage(sess, core.UserMessage{
			Role: "user", Content: q, Timestamp: time.Now(),
		})

		ctx2 := sess.BuildContext()
		messages := append(ctx2.Messages, core.UserMessage{
			Role: "user", Content: q, Timestamp: time.Now(),
		})

		runCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
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
	}

	// 验证记忆保持
	common.PrintResult("[%s] 验证记忆: 问 '我是谁？'", name)
	ctx3 := sess.BuildContext()
	messages := append(ctx3.Messages, core.UserMessage{
		Role: "user", Content: "请根据我们之前的对话，总结一下我告诉你的关于我的信息。", Timestamp: time.Now(),
	})

	stream, _ := agent.AgentLoopDetailed(ctx, messages, config)
	_, _ = stream.ForEach(ctx, func(evt agent.AgentEvent) error {
		if e, ok := evt.(agent.EventMessageUpdate); ok {
			if ae, ok := e.AssistantEvent.(core.EventTextDelta); ok {
				fmt.Print(ae.Delta)
			}
		}
		return nil
	})
	fmt.Println()
	fmt.Fprintln(os.Stderr)

	common.PrintSessionStats(sess)
}
