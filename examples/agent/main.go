// Package main 演示如何基于 pi-ai-go/agent 构造带工具调用的交互式 Agent。
//
// 功能：
//  1. 加载 .env 与统一 LLM env（LLM_API_KEY / LLM_PROVIDER / LLM_MODEL / LLM_BASE_URL）
//  2. 启动带 4 个内置工具（calculator/weather/database_query/search）的 Agent
//  3. 支持 -reasoning 思维链级别（默认 medium）
//  4. 交互式 REPL：输入问题、打印事件流
//
// 运行：go run .
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	piai "pi-ai-go"
	"pi-ai-go/agent"
	"pi-ai-go/agent/session"
	_ "pi-ai-go/providers"
)

func main() {
	// 加载 .env（按顺序查找，第一个存在即生效）
	loadEnv(
		".env",
		"../.env",
		"../../.env",
		`C:\Users\huangyicao\Downloads\hyperframes-test\pi-ai-go\.env`,
	)

	// CLI flags
	verbose := flag.Bool("v", false, "Verbose mode")
	reasoningFlag := flag.String("reasoning", envFirst("LLM_REASONING", "REASONING"),
		"Reasoning level: off|minimal|low|medium|high|xhigh (default medium)")
	timeoutFlag := flag.Int("timeout", 60, "Per-turn timeout in seconds")
	builtinFlag := flag.Bool("builtin", envBool("AGENT_BUILTIN_TOOLS", false),
		"Enable pi-ai-go/agent/tools built-in toolset (read/write/edit/bash/glob/grep)")
	skillsDir := flag.String("skills", envFirst("AGENT_SKILLS_DIR", "skills"),
		"Directory containing SKILL.md files (set empty to disable)")
	flag.Parse()

	cfg := resolveAppConfig(*verbose)
	if *reasoningFlag != "" {
		cfg.Reasoning = resolveReasoning(*reasoningFlag)
	}
	cfg.print()

	if cfg.APIKey == "" {
		fmt.Fprintln(os.Stderr, "错误: 请设置 LLM_API_KEY（或 XIAOMI_API_KEY / SILICONFLOW_API_KEY）")
		os.Exit(1)
	}

	// 工具列表：自定义工具 +（可选）内置工具
	tools := defaultTools()
	if *builtinFlag {
		tools = append(tools, builtinTools()...)
	}

	// Skills 加载
	skills, skillDiags := loadSkillsOrEmpty(*skillsDir)
	for _, d := range skillDiags {
		fmt.Fprintf(os.Stderr, "[skill] %s: %s\n", d.Path, d.Message)
	}
	fmt.Printf("🔧 工具: %d 个 | 🧠 Skills: %d 个\n", len(tools), len(skills))

	// 构建 Agent
	aiAgent := agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model:        cfg.Model,
			SystemPrompt: buildSystemPrompt(skills, *builtinFlag),
			Tools:        tools,
			Skills:       skills,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{
					APIKey:          cfg.APIKey,
					MaxRetries:      3,
					MaxRetryDelayMs: 30000,
				},
				Reasoning: cfg.Reasoning,
			},
		},
	})
	aiAgent.Subscribe(makePrinter(*verbose))

	// REPL
	runREPL(aiAgent, *timeoutFlag)
}

// loadSkillsOrEmpty 从指定目录加载 SKILL.md，目录为空或不存在则返回空列表。
// 同时按相对路径尝试常见位置（./skills、../skills、../../skills）以方便从子目录运行。
func loadSkillsOrEmpty(dir string) ([]session.Skill, []session.SkillDiagnostic) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil
	}
	// 收集所有候选路径
	candidates := []string{dir}
	if abs, err := filepath.Abs(dir); err == nil {
		candidates = append(candidates, abs)
	}
	// 同时兼容 ../skills、../../skills
	if !filepath.IsAbs(dir) {
		candidates = append(candidates,
			filepath.Join("..", dir),
			filepath.Join("..", "..", dir),
		)
	}

	var allSkills []session.Skill
	var allDiags []session.SkillDiagnostic
	seen := make(map[string]bool)
	for _, d := range candidates {
		abs, _ := filepath.Abs(d)
		if seen[abs] {
			continue
		}
		seen[abs] = true
		skills, diags := session.LoadSkills(d)
		allSkills = append(allSkills, skills...)
		allDiags = append(allDiags, diags...)
	}
	return allSkills, allDiags
}

// buildSystemPrompt 拼接系统提示，附加已加载 skills 摘要。
func buildSystemPrompt(skills []session.Skill, builtinEnabled bool) string {
	var sb strings.Builder
	sb.WriteString(defaultSystemPrompt)
	if builtinEnabled {
		sb.WriteString("\n\n已启用内置文件/命令工具：read_file / write_file / edit_file / bash / glob / grep。\n")
	}
	if len(skills) > 0 {
		sb.WriteString("\n## 已加载 Skills\n")
		for _, s := range skills {
			if s.Name != "" {
				fmt.Fprintf(&sb, "- %s", s.Name)
				if s.Description != "" {
					fmt.Fprintf(&sb, ": %s", oneLine(s.Description))
				}
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

// runREPL 简单的 read-eval-print loop。
func runREPL(aiAgent *agent.Agent, timeoutSec int) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("🤖 Agent 工具调用测试 Demo")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("💬 开始对话（输入 'quit' / 'exit' 退出，'tools' 列出所有工具）")
	fmt.Println("📝 示例问题:")
	fmt.Println("   - 计算 (123 + 456) * 789 / 10")
	fmt.Println("   - 用 calculator 求 sqrt(2) + log(100)")
	fmt.Println("   - 查询北京的天气")
	fmt.Println("   - 查询用户信息")
	fmt.Println("   - 搜索人工智能的最新进展")
	fmt.Println()

	for {
		fmt.Print("\n👤 你: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch strings.ToLower(input) {
		case "quit", "exit", "q":
			fmt.Println("\n👋 再见！")
			return
		case "tools":
			fmt.Println("🔧 可用工具:")
			for _, t := range aiAgent.State().Tools {
				fmt.Printf("   - %s: %s\n", t.Name, oneLine(t.Description))
			}
			continue
		case "skills":
			fmt.Printf("🧠 Skills: %d 个\n", len(aiAgent.State().Skills))
			for _, s := range aiAgent.State().Skills {
				fmt.Printf("   - %s\n", s.Name)
			}
			continue
		case "history":
			fmt.Printf("📝 历史: %d 条\n", len(aiAgent.Messages()))
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
		_, err := aiAgent.Run(ctx, piai.UserMessage{
			Role:    "user",
			Content: input,
		})
		cancel()
		if err != nil {
			fmt.Printf("\n❌ 错误: %v\n", err)
		}
		fmt.Printf("\n📊 [消息历史: %d 条]\n", len(aiAgent.Messages()))
	}
}

// makePrinter 构造事件订阅器。
func makePrinter(verbose bool) func(agent.AgentEvent) {
	return func(evt agent.AgentEvent) {
		switch e := evt.(type) {
		case agent.EventAgentStart:
			fmt.Println("\n🤖 [Agent 开始运行]")
		case agent.EventAgentEnd:
			fmt.Printf("\n✅ [Agent 运行结束，共 %d 条消息]\n", len(e.Messages))
		case agent.EventTurnStart:
			fmt.Println("\n🔄 [新一轮对话开始]")
		case agent.EventTurnEnd:
			fmt.Printf("🔄 [对话轮次结束，工具调用数: %d]\n", len(e.ToolResults))
		case agent.EventMessageStart:
			fmt.Print("💬 [助手回复]: ")
		case agent.EventMessageUpdate:
			switch ae := e.AssistantEvent.(type) {
			case piai.EventTextDelta:
				fmt.Print(ae.Delta)
			case piai.EventThinkingDelta:
				if verbose {
					fmt.Fprintf(os.Stderr, "\n💭 [思考] %s", ae.Delta)
				}
			}
		case agent.EventMessageEnd:
			fmt.Println()
		case agent.EventToolExecStart:
			fmt.Printf("\n🔧 [开始执行工具] %s (id=%s)\n", e.ToolName, e.ToolCallID)
			if verbose {
				fmt.Printf("   参数: %s\n", string(e.Args))
			}
		case agent.EventToolExecUpdate:
			if verbose {
				fmt.Printf("   ⏳ [工具执行中] %s\n", string(e.PartialResult))
			}
		case agent.EventToolExecEnd:
			if e.IsError {
				fmt.Printf("   ❌ [工具执行失败] %s\n", string(e.Result))
			} else {
				fmt.Printf("   ✅ [工具执行完成]\n")
			}
		}
	}
}

// oneLine 折叠多行描述为单行。
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.Join(strings.Fields(s), " ")
}
