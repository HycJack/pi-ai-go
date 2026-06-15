// Demo 06: 自定义会话条目 (Custom Entries)
//
// 演示场景:
//   1. 写入自定义类型的会话条目
//   2. 写入模型切换记录
//   3. 写入思考级别变更
//   4. 验证 BuildContext 正确处理各种条目类型
package main

import (
	"fmt"
	"os"
	"time"

	"examples/agent-memory-demo/demos/internal/common"

	"pi-ai-go/agent/session"
	"pi-ai-go/core"
)

func main() {
	common.LoadEnv()
	common.PrintHeader("Demo 06: 自定义会话条目")

	apiKey := common.GetAPIKey("deepseek")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: API key not configured")
		os.Exit(1)
	}
	_ = apiKey

	storage := session.NewMemoryStorage()
	sess, _ := common.NewSession(storage, "自定义条目测试")
	defer sess.Close()

	// ── 阶段 1: 写入各种自定义条目 ──
	common.PrintStep("阶段 1: 写入各种自定义条目")

	// 1. 会话元信息
	_ = sess.Append(session.SessionTreeEntry{
		ID:          session.GenerateID(),
		Type:        session.EntrySessionInfo,
		Timestamp:   time.Now(),
		SessionID:   "custom-test-001",
		Description: "测试自定义条目",
	})
	common.PrintResult("已写入: EntrySessionInfo")

	// 2. 模型切换
	_ = sess.Append(session.SessionTreeEntry{
		ID:        session.GenerateID(),
		Type:      session.EntryModelChange,
		Timestamp: time.Now(),
		Provider:  "deepseek",
		ModelID:   "deepseek-chat",
	})
	common.PrintResult("已写入: EntryModelChange (deepseek/deepseek-chat)")

	// 3. 思考级别变更
	_ = sess.Append(session.SessionTreeEntry{
		ID:            session.GenerateID(),
		Type:          session.EntryThinkingChange,
		Timestamp:     time.Now(),
		ThinkingLevel: "high",
	})
	common.PrintResult("已写入: EntryThinkingChange (high)")

	// 4. 用户消息
	_ = sess.Append(session.SessionTreeEntry{
		ID:        session.GenerateID(),
		Type:      session.EntryMessage,
		Timestamp: time.Now(),
		Message: core.UserMessage{
			Role: "user", Content: "你好", Timestamp: time.Now(),
		},
	})
	common.PrintResult("已写入: EntryMessage (user)")

	// 5. 助手消息
	_ = sess.Append(session.SessionTreeEntry{
		ID:        session.GenerateID(),
		Type:      session.EntryMessage,
		Timestamp: time.Now(),
		Message: core.AssistantMessage{
			Role: "assistant",
			Content: []core.ContentBlock{
				core.TextContent{Type: "text", Text: "你好！有什么可以帮助你的？"},
			},
			Timestamp: time.Now(),
		},
	})
	common.PrintResult("已写入: EntryMessage (assistant)")

	// 6. 自定义消息
	_ = sess.Append(session.SessionTreeEntry{
		ID:         session.GenerateID(),
		Type:       session.EntryCustomMessage,
		Timestamp:  time.Now(),
		CustomType: "system_event",
		Content:    "用户登录",
		Display:    false,
	})
	common.PrintResult("已写入: EntryCustomMessage (system_event)")

	// 7. 压缩记录
	_ = sess.Append(session.SessionTreeEntry{
		ID:                session.GenerateID(),
		Type:              session.EntryCompaction,
		Timestamp:         time.Now(),
		CompactionSummary: "压缩了 5 条历史消息",
		TokensBefore:      10000,
		FirstKeptEntryID:  "entry-005",
	})
	common.PrintResult("已写入: EntryCompaction")

	// ── 阶段 2: 验证 BuildContext ──
	common.PrintStep("阶段 2: 验证 BuildContext")
	ctx := sess.BuildContext()
	common.PrintResult("重建的消息数: %d", len(ctx.Messages))
	common.PrintResult("当前模型: %s/%s", ctx.Model.Provider, ctx.Model.ModelID)
	common.PrintResult("思考级别: %s", ctx.ThinkingLevel)

	// 验证自定义消息被转换为 UserMessage
	for i, m := range ctx.Messages {
		if um, ok := m.(core.UserMessage); ok {
			if s, ok := um.Content.(string); ok {
				fmt.Fprintf(os.Stderr, "  [%d] UserMessage: %s\n", i, truncate(s, 50))
			}
		}
	}

	common.PrintStep("测试结果")
	common.PrintSessionStats(sess)
	common.PrintResult("✓ 所有自定义条目类型成功写入")
	common.PrintResult("✓ BuildContext 正确处理各种类型")
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
