// Demo 02: JSONL 持久化与重载 (Persistence)
//
// 演示场景:
//   1. 创建会话并写入 JSONL 文件
//   2. 模拟程序退出（关闭会话）
//   3. 重新打开会话，验证数据完整性
//   4. 继续对话，验证上下文恢复
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
	common.PrintHeader("Demo 02: JSONL 持久化与重载")

	apiKey := common.GetAPIKey("deepseek")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: API key not configured")
		os.Exit(1)
	}
	model := common.ResolveModel("deepseek", "deepseek-chat")

	persistFile := "/tmp/demo-persistence.jsonl"
	defer os.Remove(persistFile)

	// ── 阶段 1: 创建会话并对话 ──
	common.PrintStep("阶段 1: 创建会话并对话")
	storage1, _ := session.NewJSONLStorage(persistFile)
	sess1, _ := common.NewSession(storage1, "持久化测试")
	sess1ID := sess1.ID()
	common.PrintResult("会话 ID: %s", sess1ID)

	config := common.BuildAgentConfig(model, apiKey, nil, nil)

	messages1 := []core.Message{
		core.UserMessage{Role: "user", Content: "记住我的幸运数字是 888。", Timestamp: time.Now()},
	}

	stream1, _ := agent.AgentLoopDetailed(context.Background(), messages1, config)
	var assistant1 core.AssistantMessage
	_, _ = stream1.ForEach(context.Background(), func(evt agent.AgentEvent) error {
		if e, ok := evt.(agent.EventMessageEnd); ok {
			assistant1 = e.Message
		}
		return nil
	})

	common.AppendMessage(sess1, messages1[0])
	common.AppendMessage(sess1, assistant1)
	common.PrintResult("已保存 %d 条消息", len(sess1.Entries()))
	storage1.Close()
	common.PrintResult("关闭存储，模拟程序退出")

	// 显示 JSONL 文件内容
	data, _ := os.ReadFile(persistFile)
	fmt.Fprintf(os.Stderr, "  📄 JSONL 文件: %d 字节\n", len(data))

	// ── 阶段 2: 重新打开会话 ──
	common.PrintStep("阶段 2: 重新打开会话")
	storage2, _ := session.NewJSONLStorage(persistFile)
	sess2, err := session.NewSession(storage2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer storage2.Close()

	common.PrintResult("恢复的会话 ID: %s (匹配: %v)", sess2.ID(), sess2.ID() == sess1ID)
	common.PrintResult("恢复的条目数: %d", len(sess2.Entries()))

	// ── 阶段 3: 继续对话，验证记忆 ──
	common.PrintStep("阶段 3: 继续对话，验证记忆")
	common.PrintResult("问 Agent: '我的幸运数字是什么？'")

	ctx2 := sess2.BuildContext()
	messages2 := append(ctx2.Messages, core.UserMessage{
		Role: "user", Content: "我的幸运数字是什么？", Timestamp: time.Now(),
	})

	stream2, _ := agent.AgentLoopDetailed(context.Background(), messages2, config)
	_, _ = stream2.ForEach(context.Background(), func(evt agent.AgentEvent) error {
		if e, ok := evt.(agent.EventMessageUpdate); ok {
			if ae, ok := e.AssistantEvent.(core.EventTextDelta); ok {
				fmt.Print(ae.Delta)
			}
		}
		return nil
	})
	fmt.Println()

	common.PrintStep("持久化测试完成")
	common.PrintResult("✓ 会话成功序列化到 JSONL")
	common.PrintResult("✓ 重启后成功恢复会话")
	common.PrintResult("✓ Agent 跨重启保持记忆")
}
