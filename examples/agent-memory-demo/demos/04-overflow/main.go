// Demo 04: 上下文窗口溢出 (Overflow)
//
// 演示场景:
//   1. 构造超出 context window 的大消息
//   2. 验证 OverflowSignal 检测
//   3. 演示 OnOverflow 钩子
//   4. 验证溢出后自动压缩恢复
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"examples/agent-memory-demo/demos/internal/common"

	"pi-ai-go/agent"
	"pi-ai-go/agent/session"
	"pi-ai-go/core"
)

func main() {
	common.LoadEnv()
	common.PrintHeader("Demo 04: 上下文窗口溢出 (Overflow)")

	apiKey := common.GetAPIKey("deepseek")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: API key not configured")
		os.Exit(1)
	}
	model := common.ResolveModel("deepseek", "deepseek-chat")

	storage := session.NewMemoryStorage()
	sess, _ := common.NewSession(storage, "溢出测试")
	defer sess.Close()

	// 配置紧凑的压缩策略
	overflowCount := 0
	compactionCount := 0
	config := common.BuildAgentConfig(model, apiKey,
		&agent.ContextPolicy{
			SoftLimit:       0.30,
			HardLimit:       0.60,
			ReservedOutput:  512,
			MinTailMessages: 2,
			Strategy:        agent.CompactionStrategySlidingWindow,
		},
		func(e agent.CompactionEvent) {
			compactionCount++
			common.PrintResult("压缩 #%d: %s 丢弃=%d token %d→%d",
				compactionCount, e.TriggeredBy, e.Dropped, e.TokensBefore, e.TokensAfter)
		},
	)
	config.OnOverflow = func(sig *agent.OverflowSignal) error {
		overflowCount++
		common.PrintResult("⚠️  溢出检测 #%d: provider=%s usage=%d window=%d source=%s",
			overflowCount, sig.Provider, sig.Usage, sig.ContextWindow, sig.Source)
		return nil
	}

	ctx := context.Background()

	// ── 阶段 1: 发送大量消息触发溢出 ──
	common.PrintStep("阶段 1: 发送大量消息")

	// 构造一些较大的消息
	largeContent := strings.Repeat("这是一段用于填充上下文的测试内容。", 50)
	questions := []string{
		"请记住这段内容: " + largeContent,
		"继续添加: " + largeContent,
		"再加一段: " + largeContent,
		"再加: " + largeContent,
		"再加: " + largeContent,
		"现在请总结一下我们讨论了什么。",
	}

	for i, q := range questions {
		fmt.Fprintf(os.Stderr, "\n🧑 Q%d: %s...\n", i+1, truncate(q, 50))

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

	common.PrintStep("测试结果")
	common.PrintResult("压缩次数: %d", compactionCount)
	common.PrintResult("溢出检测次数: %d", overflowCount)
	common.PrintResult("最终消息数: %d", len(sess.Entries()))
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
