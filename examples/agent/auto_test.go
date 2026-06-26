package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	piai "pi-ai-go"
	"pi-ai-go/agent"
)

// newTestAgent 构造一个最小化的测试 Agent，使用全局 resolveAppConfig。
// 注意：实际运行测试前需要设置 LLM_API_KEY（参见 .env.example）。
func newTestAgent(t *testing.T, systemPrompt string, tools ...agent.AgentTool) *agent.Agent {
	t.Helper()
	cfg := resolveAppConfig(false)
	if cfg.APIKey == "" {
		t.Skip("LLM_API_KEY 未设置，跳过集成测试")
	}
	return agent.New(agent.AgentOptions{
		InitialState: &agent.AgentState{
			Model:        cfg.Model,
			SystemPrompt: systemPrompt,
			Tools:        tools,
			SimpleStreamOptions: piai.SimpleStreamOptions{
				StreamOptions: piai.StreamOptions{APIKey: cfg.APIKey},
			},
		},
	})
}

// runQuery 在 30s 超时下运行一次查询并打印最终 assistant 文本。
func runQuery(t *testing.T, ag *agent.Agent, query string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := ag.Run(ctx, piai.UserMessage{Role: "user", Content: query})
	if err != nil {
		t.Fatalf("运行失败: %v", err)
	}
	fmt.Println("\n最终回复:")
	for _, msg := range result {
		am, ok := msg.(piai.AssistantMessage)
		if !ok {
			continue
		}
		for _, block := range am.Content {
			if text, ok := block.(piai.TextContent); ok {
				fmt.Println(text.Text)
			}
		}
	}
}

// TestCalculatorTool 单元测试：calc 求值正确（不依赖 LLM）。
func TestCalculatorTool(t *testing.T) {
	cases := []struct {
		expr string
		want float64
	}{
		{"1+1", 2},
		{"2+3*4", 14},
		{"(2+3)*4", 20},
		{"2^10", 1024},
		{"sqrt(16)", 4},
		{"sin(0)", 0},
		{"cos(0)", 1},
		{"log(100)", 2},
		{"ln(1)", 0},
		{"abs(-3.5)", 3.5},
		{"pow(2, 8)", 256},
		{"-3+5", 2},
		{"-(2+3)*4", -20},
	}
	for _, c := range cases {
		got, err := evaluateExpression(c.expr)
		if err != nil {
			t.Errorf("evaluateExpression(%q) error: %v", c.expr, err)
			continue
		}
		if !floatNear(got, c.want, 1e-6) {
			t.Errorf("evaluateExpression(%q) = %v, want %v", c.expr, got, c.want)
		}
	}
}

func floatNear(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}

func TestCalculatorIntegration(t *testing.T) {
	ag := newTestAgent(t,
		"你是一个数学计算助手。使用 calculator 工具来计算数学表达式。",
		calculatorTool())
	runQuery(t, ag, "计算 123 + 456 * 789")
}

func TestWeatherIntegration(t *testing.T) {
	ag := newTestAgent(t,
		"你是一个天气查询助手。使用 weather 工具来查询天气信息。",
		weatherTool())
	runQuery(t, ag, "查询北京的天气")
}

func TestDatabaseQueryIntegration(t *testing.T) {
	ag := newTestAgent(t,
		"你是一个数据库查询助手。使用 database_query 工具来查询数据。",
		databaseQueryTool())
	runQuery(t, ag, "查询用户信息")
}

func TestMultipleToolsIntegration(t *testing.T) {
	ag := newTestAgent(t, `你是一个多功能助手。可以使用以下工具：
1. calculator - 数学计算
2. weather - 天气查询
3. database_query - 数据库查询

根据用户问题选择合适的工具。`,
		calculatorTool(), weatherTool(), databaseQueryTool())
	runQuery(t, ag, "帮我计算 2 的 10 次方，然后查询上海的天气")
}
