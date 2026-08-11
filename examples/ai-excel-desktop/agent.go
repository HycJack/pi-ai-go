package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"pi-ai-go/agent"
	"pi-ai-go/core"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================================
// Agent 流程：AI 对话使用 pi-ai-go 的 agent loop
// ============================================================================

// AgentMessage 处理一个 agent 模式的流式对话请求。
// 它构建消息历史、运行 agent loop、并把事件流发射到前端。
func (a *App) AgentMessage(jsonStr string) error {
	var req AgentRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		LogError("[agent] parse error: %v", err)
		runtime.EventsEmit(a.ctx, "agent-error", fmt.Sprintf("parse error: %v", err))
		return err
	}

	LogInfo("[agent] request: provider=%s model=%s msgLen=%d history=%d",
		req.Provider, req.Model, len(req.Message), len(req.Messages))

	model, _, opts := a.providerFromRequest(req.Provider, req.APIKey, req.BaseURL, req.Model)
	if req.MaxTokens > 0 {
		t := req.MaxTokens
		opts.MaxTokens = &t
	}
	if req.Temperature > 0 {
		t := req.Temperature
		opts.Temperature = &t
	}

	systemPrompt := a.buildSystemPrompt()
	tools := a.toolsForDocType()

	config := agent.AgentLoopConfig{
		Model:              model,
		SystemPrompt:       systemPrompt,
		Tools:              tools,
		SimpleStreamOptions: opts,
	}

	messages := a.buildAgentMessages(req)

	streamCtx, cancelFn := context.WithCancel(a.ctx)
	a.cancelFn = cancelFn

	eventStream, detailed := agent.AgentLoopDetailed(streamCtx, messages, config)

	LogInfo("[agent] loop started, messages=%d tools=%d", len(messages), len(config.Tools))

	go func() {
		defer func() {
			a.cancelFn = nil
		}()
		textLen := 0
		_, forEachErr := eventStream.ForEach(streamCtx, func(evt agent.AgentEvent) error {
			switch e := evt.(type) {
			case agent.EventMessageUpdate:
				if e.AssistantEvent != nil {
					switch ae := e.AssistantEvent.(type) {
					case core.EventTextDelta:
						textLen += len(ae.Delta)
						runtime.EventsEmit(a.ctx, "agent-text-delta", ae.Delta)
					case core.EventThinkingDelta:
						runtime.EventsEmit(a.ctx, "agent-thinking-delta", ae.Delta)
					case core.EventToolCallStart:
						data, _ := json.Marshal(map[string]interface{}{"id": ae.ID, "name": ae.Name, "arguments": ""})
						runtime.EventsEmit(a.ctx, "agent-tool-call-start", string(data))
					case core.EventToolCallDelta:
						runtime.EventsEmit(a.ctx, "agent-tool-call-delta", ae.ArgumentsDelta)
					case core.EventToolCallEnd:
						argsStr := string(ae.Arguments)
						safeArgs, _ := json.Marshal(argsStr)
						runtime.EventsEmit(a.ctx, "agent-tool-call-end", string(safeArgs))
					}
				}
			case agent.EventTurnEnd:
				LogInfo("[agent] turn end, text length so far=%d", textLen)
			case agent.EventToolExecStart:
				data, _ := json.Marshal(map[string]interface{}{
					"toolCallID": e.ToolCallID,
					"toolName":   e.ToolName,
				})
				runtime.EventsEmit(a.ctx, "agent-tool-exec-start", string(data))
			case agent.EventToolExecEnd:
				result, _ := json.Marshal(map[string]interface{}{"success": !e.IsError})
				if !e.IsError && len(e.Result) > 0 {
					var toolResult core.AgentToolResult
					if err := json.Unmarshal(e.Result, &toolResult); err == nil {
						var textParts []string
						for _, block := range toolResult.Content {
							if tc, ok := block.(core.TextContent); ok {
								textParts = append(textParts, tc.Text)
							}
						}
						if len(textParts) > 0 {
							runtime.EventsEmit(a.ctx, "agent-tool-result", strings.Join(textParts, "\n"))
						} else {
							runtime.EventsEmit(a.ctx, "agent-tool-result", string(e.Result))
						}
					} else {
						runtime.EventsEmit(a.ctx, "agent-tool-result", string(e.Result))
					}
				}
				runtime.EventsEmit(a.ctx, "agent-tool-exec-end", string(result))
			case agent.EventAgentEnd:
				// agent 完成
			}
			return nil
		})
		if forEachErr != nil {
			LogError("[agent] ForEach error: %v (textLen=%d)", forEachErr, textLen)
			runtime.EventsEmit(a.ctx, "agent-error", fmt.Sprintf("agent error: %v", forEachErr))
			runtime.EventsEmit(a.ctx, "agent-done", "")
			return
		}

		result, err := detailed()
		if err != nil {
			LogError("[agent] detailed() error: %v (textLen=%d)", err, textLen)
			runtime.EventsEmit(a.ctx, "agent-error", fmt.Sprintf("agent error: %v", err))
			runtime.EventsEmit(a.ctx, "agent-done", "")
			return
		}

		LogInfo("[agent] done, total text length=%d, messages=%d", textLen, len(result.Messages))
		if textLen == 0 {
			LogWarn("[agent] done but no text was emitted — agent may have only used tools or returned empty content")
		}

		runtime.EventsEmit(a.ctx, "agent-done", "")
	}()

	return nil
}

// buildSystemPrompt 根据 当前文档类型构建 agent 的系统提示词。
func (a *App) buildSystemPrompt() string {
	docType := a.GetDocType()
	var sb strings.Builder
	sb.WriteString("你是 AI 文档工作台的智能助手。\n")
	sb.WriteString("你可以访问用户已加载的文档，并通过工具调用完成分析、编辑与图表生成。\n\n")

	switch docType {
	case "excel", "csv":
		sb.WriteString("## 当前文档类型：表格数据 (" + docType + ")\n")
		sb.WriteString("你可以使用以下工具：\n")
		sb.WriteString("- `get_data_summary` 查看数据集概览（列名、类型、行数）\n")
		sb.WriteString("- `get_column_stats` 获取指定列的统计信息（最小值/最大值/均值/中位数/标准差/唯一值数）\n")
		sb.WriteString("- `get_sample_rows` 预览前 N 行数据\n")
		sb.WriteString("- `generate_chart` 生成图表（会自动渲染到画布）\n\n")
		sb.WriteString("## 工作流程\n")
		sb.WriteString("1. 先用 `get_data_summary` 了解数据结构\n")
		sb.WriteString("2. 根据用户需求，必要时用 `get_column_stats` 确认列的类型与分布\n")
		sb.WriteString("3. 用 `generate_chart` 生成图表\n")
		sb.WriteString("4. 用自然语言向用户解释你的分析结果与图表配置\n\n")
		sb.WriteString("## generate_chart 参数说明\n")
		sb.WriteString("- type: bar(柱状图) | line(折线图) | pie(饼图) | scatter(散点图) | histogram(直方图) | area(面积图) | scatter3d(三维散点图)\n")
		sb.WriteString("- xAxis: X 轴列名\n")
		sb.WriteString("- yAxes: Y 轴列名数组（scatter3d: 第1个Y轴, 第2个Z轴）\n")
		sb.WriteString("- title: 图表标题\n")
		sb.WriteString("- bin: 直方图分箱数（默认 20，仅 histogram 有效）\n")
		sb.WriteString("- sample: 采样行数（数据量大时使用，0 表示不采样）\n\n")
	case "word":
		sb.WriteString("## 当前文档类型：Word 文档\n")
		sb.WriteString("用户已加载 Word 文档，你可以在右侧编辑器中查看与编辑。\n")
		sb.WriteString("你可以帮助用户：撰写、润色、翻译、总结文档内容，回答关于文档的问题。\n")
		sb.WriteString("如需生成新的 Word 内容，直接以 markdown 输出，用户可复制使用。\n\n")
	case "ppt":
		sb.WriteString("## 当前文档类型：PowerPoint 演示文稿\n")
		sb.WriteString("用户已加载 PPT 文件。你可以帮助用户：设计幻灯片结构、撰写演讲稿、总结要点、优化排版建议。\n\n")
	case "pdf":
		sb.WriteString("## 当前文档类型：PDF 文档\n")
		sb.WriteString("用户已加载 PDF 文件。你可以帮助用户：总结内容、提取关键信息、翻译、回答关于文档的问题。\n\n")
	case "text":
		sb.WriteString("## 当前文档类型：文本文件\n")
		sb.WriteString("用户已加载文本/Markdown 文件。你可以帮助用户：编辑、润色、格式化、翻译、总结。\n\n")
	default:
		sb.WriteString("## 当前未加载文档\n")
		sb.WriteString("请引导用户点击\"打开文件\"加载文档。支持 Excel/CSV/Word/PPT/PDF/文本。\n\n")
	}

	sb.WriteString("## 输出规范\n")
	sb.WriteString("- 使用简洁、专业的中文回复\n")
	sb.WriteString("- 代码与配置使用 markdown 格式\n")
	sb.WriteString("- 不要在回复中使用 emoji\n")
	return sb.String()
}

// toolsForDocType 根据当前文档类型返回相应的工具集。
func (a *App) toolsForDocType() []agent.AgentTool {
	docType := a.GetDocType()
	switch docType {
	case "excel", "csv":
		return a.excelTools()
	default:
		// Word/PPT/PDF/Text 等文档类型暂无自定义工具，纯对话模式
		return []agent.AgentTool{}
	}
}

// buildAgentMessages 从 AgentRequest 构建消息历史。
func (a *App) buildAgentMessages(req AgentRequest) []core.Message {
	var messages []core.Message
	for _, m := range req.Messages {
		role, _ := m["role"].(string)
		if role == "" {
			continue
		}
		content, _ := m["content"].(string)
		if role == "user" {
			messages = append(messages, core.UserMessage{Content: content})
		} else if role == "assistant" {
			messages = append(messages, core.AssistantMessage{
				Content: []core.ContentBlock{core.TextContent{Text: content}},
			})
		}
	}
	messages = append(messages, core.UserMessage{Content: req.Message})
	return messages
}

// ============================================================================
// 自定义 Excel 工具：让 agent 能查询数据、生成图表
// ============================================================================

const schemaGetDataSummary = `{
	"type": "object",
	"properties": {},
	"required": []
}`

const schemaGetColumnStats = `{
	"type": "object",
	"properties": {
		"columns": {
			"type": "array",
			"items": { "type": "string" },
			"description": "要统计的列名列表。为空则返回所有列。"
		}
	},
	"required": []
}`

const schemaGetSampleRows = `{
	"type": "object",
	"properties": {
		"count": {
			"type": "integer",
			"description": "返回的行数，默认 10，最大 50。"
		}
	},
	"required": []
}`

const schemaGenerateChart = `{
	"type": "object",
	"properties": {
		"type":   { "type": "string", "enum": ["bar","line","pie","scatter","histogram","area","scatter3d"], "description": "图表类型（scatter3d 为三维散点图）" },
		"xAxis":  { "type": "string", "description": "X 轴列名" },
		"yAxes":  { "type": "array", "items": { "type": "string" }, "description": "Y 轴列名数组（scatter3d 时：第1个元素是Y轴，第2个元素是Z轴）" },
		"zAxis":  { "type": "string", "description": "Z 轴列名（仅 scatter3d 三维散点图使用）" },
		"title":  { "type": "string", "description": "图表标题" },
		"bin":    { "type": "integer", "description": "直方图分箱数，默认 20" },
		"sample": { "type": "integer", "description": "采样行数（数据量大时使用，0 表示不采样）" }
	},
	"required": ["type", "xAxis", "yAxes"]
}`

// excelTools 返回 Excel 数据相关的自定义工具集。
func (a *App) excelTools() []agent.AgentTool {
	return []agent.AgentTool{
		{
			Name:        "get_data_summary",
			Label:       "数据概览",
			Description: "获取当前已加载数据集的概览：文件名、工作表、总行数、总列数、每列的名称与类型。",
			Parameters:  json.RawMessage(schemaGetDataSummary),
			Execute:     a.executeGetDataSummary,
		},
		{
			Name:        "get_column_stats",
			Label:       "列统计",
			Description: "获取指定列的统计信息：类型、计数、空值数、最小值、最大值、均值、中位数、标准差、唯一值数。",
			Parameters:  json.RawMessage(schemaGetColumnStats),
			Execute:     a.executeGetColumnStats,
		},
		{
			Name:        "get_sample_rows",
			Label:       "预览数据",
			Description: "预览前 N 行数据，用于了解数据内容。默认 10 行，最大 50 行。",
			Parameters:  json.RawMessage(schemaGetSampleRows),
			Execute:     a.executeGetSampleRows,
		},
		{
			Name:        "generate_chart",
			Label:       "生成图表",
			Description: "根据指定配置生成图表并渲染到画布。type 为图表类型，xAxis/yAxes 为列名。",
			Parameters:  json.RawMessage(schemaGenerateChart),
			Execute:     a.executeGenerateChart,
		},
	}
}

// --- 工具执行函数 ---

func (a *App) executeGetDataSummary(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	summary, err := a.GetDataSummary()
	if err != nil {
		return errResult("get_data_summary: " + err.Error()), nil
	}
	return okResult(summary), nil
}

type columnStatsArgs struct {
	Columns []string `json:"columns"`
}

func (a *App) executeGetColumnStats(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	a.dataMu.RLock()
	data := a.sheetData
	a.dataMu.RUnlock()
	if data == nil {
		return errResult("未加载数据"), nil
	}

	var args columnStatsArgs
	_ = json.Unmarshal(params, &args)

	allStats, err := a.AnalyzeColumns()
	if err != nil {
		return errResult("get_column_stats: " + err.Error()), nil
	}

	if len(args.Columns) == 0 {
		j, _ := json.MarshalIndent(allStats, "", "  ")
		return okResult(string(j)), nil
	}

	want := make(map[string]bool, len(args.Columns))
	for _, c := range args.Columns {
		want[c] = true
	}
	var filtered []*ColumnStats
	for _, s := range allStats {
		if want[s.Name] {
			filtered = append(filtered, s)
		}
	}
	if len(filtered) == 0 {
		return errResult("未找到指定的列：" + strings.Join(args.Columns, ", ")), nil
	}
	j, _ := json.MarshalIndent(filtered, "", "  ")
	return okResult(string(j)), nil
}

type sampleRowsArgs struct {
	Count int `json:"count"`
}

func (a *App) executeGetSampleRows(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	a.dataMu.RLock()
	data := a.sheetData
	a.dataMu.RUnlock()
	if data == nil {
		return errResult("未加载数据"), nil
	}

	var args sampleRowsArgs
	_ = json.Unmarshal(params, &args)
	count := args.Count
	if count <= 0 {
		count = 10
	}
	if count > 50 {
		count = 50
	}
	if count > len(data.Rows) {
		count = len(data.Rows)
	}

	// 构造带表头的对象数组，便于 LLM 理解
	objRows := make([]map[string]interface{}, 0, count)
	for i := 0; i < count; i++ {
		o := make(map[string]interface{}, len(data.Headers))
		for j, h := range data.Headers {
			if j < len(data.Rows[i]) {
				o[h] = data.Rows[i][j]
			}
		}
		objRows = append(objRows, o)
	}
	j, _ := json.MarshalIndent(objRows, "", "  ")
	return okResult(fmt.Sprintf("前 %d 行数据：\n%s", count, string(j))), nil
}

type generateChartArgs struct {
	Type   string   `json:"type"`
	XAxis  string   `json:"xAxis"`
	YAxes  []string `json:"yAxes"`
	ZAxis  string   `json:"zAxis"`
	Title  string   `json:"title"`
	Bin    int      `json:"bin"`
	Sample int      `json:"sample"`
}

func (a *App) executeGenerateChart(ctx context.Context, toolCallID string, params json.RawMessage, onUpdate func(json.RawMessage)) (core.AgentToolResult, error) {
	a.dataMu.RLock()
	data := a.sheetData
	a.dataMu.RUnlock()
	if data == nil {
		return errResult("未加载数据"), nil
	}

	var args generateChartArgs
	if err := json.Unmarshal(params, &args); err != nil {
		return errResult("invalid arguments: " + err.Error()), nil
	}
	if args.Type == "" || args.XAxis == "" || len(args.YAxes) == 0 {
		return errResult("type, xAxis, yAxes 为必填参数"), nil
	}
	if args.Bin <= 0 {
		args.Bin = 20
	}

	// 校验列名是否存在
	headerSet := make(map[string]bool, len(data.Headers))
	for _, h := range data.Headers {
		headerSet[h] = true
	}
	if !headerSet[args.XAxis] {
		return errResult(fmt.Sprintf("X 轴列名不存在: %s", args.XAxis)), nil
	}
	for _, y := range args.YAxes {
		if !headerSet[y] {
			return errResult(fmt.Sprintf("Y 轴列名不存在: %s", y)), nil
		}
	}

	cfg := &ChartConfig{
		Type:   args.Type,
		XAxis:  args.XAxis,
		YAxes:  args.YAxes,
		Title:  args.Title,
		Bin:    args.Bin,
		Sample: args.Sample,
	}

	// 通过事件把图表配置发射到前端，前端负责渲染
	cfgJSON, _ := json.Marshal(cfg)
	runtime.EventsEmit(a.ctx, "chart-config", string(cfgJSON))

	summary := fmt.Sprintf("已生成 %s 图表：X轴=%s，Y轴=%s", args.Type, args.XAxis, strings.Join(args.YAxes, ", "))
	return okResult(summary), nil
}

// --- 工具结果辅助函数 ---

func okResult(text string) core.AgentToolResult {
	return core.AgentToolResult{
		Content: []core.ContentBlock{core.TextContent{Type: "text", Text: text}},
	}
}

func errResult(text string) core.AgentToolResult {
	return core.AgentToolResult{
		Content: []core.ContentBlock{core.TextContent{Type: "text", Text: text}},
		IsError: true,
	}
}

