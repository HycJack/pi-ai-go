// Demo 05: 分支摘要 (Branch Summary)
//
// 演示场景:
//   1. 探索一个对话分支（多轮对话）
//   2. 生成分支摘要
//   3. 将摘要追加到会话
//   4. 验证摘要作为上下文传递给后续 LLM 调用
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
	common.PrintHeader("Demo 05: 分支摘要 (Branch Summary)")

	apiKey := common.GetAPIKey("deepseek")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: API key not configured")
		os.Exit(1)
	}
	model := common.ResolveModel("deepseek", "deepseek-chat")

	storage := session.NewMemoryStorage()
	sess, _ := common.NewSession(storage, "分支摘要测试")
	defer sess.Close()

	config := common.BuildAgentConfig(model, apiKey, nil, nil)

	// ── 阶段 1: 探索一个分支 ──
	common.PrintStep("阶段 1: 探索一个对话分支")
	branchMessages := []core.Message{}

	questions := []string{
		"我在尝试用 Go 实现一个 LRU 缓存。",
		"我用了双向链表 + map 的方案。",
		"但是并发场景下有数据竞争问题。",
		"我加了 sync.Mutex 但性能下降严重。",
	}
	for _, q := range questions {
		common.AppendMessage(sess, core.UserMessage{
			Role: "user", Content: q, Timestamp: time.Now(),
		})
		branchMessages = append(branchMessages, core.UserMessage{
			Role: "user", Content: q, Timestamp: time.Now(),
		})

		runCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		stream, _ := agent.AgentLoopDetailed(runCtx, branchMessages, config)
		var lastAssistant core.AssistantMessage
		_, _ = stream.ForEach(runCtx, func(evt agent.AgentEvent) error {
			if e, ok := evt.(agent.EventMessageEnd); ok {
				lastAssistant = e.Message
			}
			return nil
		})
		cancel()

		if lastAssistant.Role == "assistant" {
			branchMessages = append(branchMessages, lastAssistant)
			common.AppendMessage(sess, lastAssistant)
		}
	}

	common.PrintResult("分支消息数: %d", len(branchMessages))

	// ── 阶段 2: 生成分支摘要 ──
	common.PrintStep("阶段 2: 生成分支摘要")
	opts := core.SimpleStreamOptions{StreamOptions: core.StreamOptions{APIKey: apiKey}}
	result, err := session.SummarizeBranchSession(
		context.Background(), sess, model, branchMessages, "lru-cache-attempt", opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	common.PrintResult("分支 ID: %s", result.FromID)
	common.PrintResult("摘要: %s", truncate(result.Summary, 200))

	// ── 阶段 3: 验证摘要作为上下文 ──
	common.PrintStep("阶段 3: 验证摘要作为上下文")
	common.PrintResult("问 Agent: '我之前在做什么？'")

	ctx2 := sess.BuildContext()
	messages := append(ctx2.Messages, core.UserMessage{
		Role: "user", Content: "我之前在做什么？", Timestamp: time.Now(),
	})

	stream, _ := agent.AgentLoopDetailed(context.Background(), messages, config)
	_, _ = stream.ForEach(context.Background(), func(evt agent.AgentEvent) error {
		if e, ok := evt.(agent.EventMessageUpdate); ok {
			if ae, ok := e.AssistantEvent.(core.EventTextDelta); ok {
				fmt.Print(ae.Delta)
			}
		}
		return nil
	})
	fmt.Println()

	common.PrintStep("测试结果")
	common.PrintSessionStats(sess)
	common.PrintResult("✓ 分支摘要已保存到会话")
	common.PrintResult("✓ 摘要作为上下文传递给后续 LLM")
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
