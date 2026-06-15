// Demo 03: 多会话并行管理 (Multi-Session)
//
// 演示场景:
//   1. 同时创建多个独立的会话
//   2. 每个会话维护独立的上下文
//   3. 在会话间切换，验证隔离性
//   4. 跨会话统计
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
	common.PrintHeader("Demo 03: 多会话并行管理")

	apiKey := common.GetAPIKey("deepseek")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: API key not configured")
		os.Exit(1)
	}
	model := common.ResolveModel("deepseek", "deepseek-chat")

	storage := session.NewMemoryStorage()

	// 创建 3 个独立会话
	sessions := make([]*session.Session, 3)
	topics := []string{
		"Go 编程",
		"Python 编程",
		"Rust 编程",
	}
	for i := range sessions {
		sess, _ := common.NewSession(storage, topics[i])
		sessions[i] = sess
		common.PrintResult("会话 %d [%s]: %s", i+1, topics[i], sess.ID())
	}

	config := common.BuildAgentConfig(model, apiKey, nil, nil)

	// ── 阶段 1: 向每个会话写入主题相关的问题 ──
	common.PrintStep("阶段 1: 向每个会话写入不同主题")
	questions := []string{
		"这门语言的主要特点是什么？",
		"它的并发模型是怎样的？",
		"它的内存管理机制是什么？",
	}
	for i, sess := range sessions {
		common.AppendMessage(sess, core.UserMessage{
			Role: "user", Content: questions[i], Timestamp: time.Now(),
		})

		ctx2 := sess.BuildContext()
		messages := append(ctx2.Messages, core.UserMessage{
			Role: "user", Content: questions[i], Timestamp: time.Now(),
		})

		runCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
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
		common.PrintResult("会话 %d: 已添加 %d 条消息", i+1, len(sess.Entries()))
	}

	// ── 阶段 2: 跨会话统计 ──
	common.PrintStep("阶段 2: 跨会话统计")
	for _, sess := range sessions {
		common.PrintSessionStats(sess)
	}

	// ── 阶段 3: 验证隔离性 ──
	common.PrintStep("阶段 3: 验证会话隔离性")
	common.PrintResult("Go 会话消息数: %d", len(sessions[0].Entries()))
	common.PrintResult("Python 会话消息数: %d", len(sessions[1].Entries()))
	common.PrintResult("Rust 会话消息数: %d", len(sessions[2].Entries()))

	// 验证每个会话的上下文独立
	for idx, sess := range sessions {
		ctx2 := sess.BuildContext()
		hasOtherTopic := false
		for _, m := range ctx2.Messages {
			if um, ok := m.(core.UserMessage); ok {
				if s, ok := um.Content.(string); ok {
					// 检查是否包含其他主题
					for j, t := range topics {
						if j != idx && len(s) > 0 && containsAny(s, []string{t}) {
							hasOtherTopic = true
						}
					}
				}
			}
		}
		if hasOtherTopic {
			common.PrintResult("❌ 会话 %d 包含其他主题内容！", idx+1)
		} else {
			common.PrintResult("✓ 会话 %d 隔离正常", idx+1)
		}
	}
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if len(sub) > 0 && contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
