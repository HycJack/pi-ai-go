// 可收起/展开的 AI 侧边栏（接入 pi-ai-go agent 事件流 + 内联图表）
import {
  Sparkles,
  PanelRightClose,
  Copy,
  RefreshCw,
  Trash2,
  Square,
  Wrench,
  ChevronRight,
  Brain,
} from "lucide-react";
import {
  Conversation,
  Message,
  MessageContent,
  MessageActions,
  MessageAction,
  ConversationEmptyState,
  PromptInput,
  ChartBlock,
} from "./Chat";
import { ToolCallStep, ToolResultStep, ThinkingBlock } from "./AgentSteps";

export function AISidebar({
  open,
  onToggle,
  messages,
  onSend,
  onClear,
  onCopyLast,
  onRegenerate,
  onCancel,
  isLoading,
  dataLoaded,
}) {
  return (
    <>
      {!open && (
        <button
          onClick={onToggle}
          className="pulse-glow fixed right-4 top-1/2 z-40 flex h-12 w-12 -translate-y-1/2 items-center justify-center rounded-full bg-brand-gradient shadow-glow-lg transition-transform hover:scale-105"
          title="打开 AI 助手"
        >
          <Sparkles className="h-5 w-5 text-white" />
        </button>
      )}

      <aside
        className={`sidebar-transition relative flex h-full flex-col border-l border-border/60 bg-background-elevated/60 backdrop-blur-xl ${
          open ? "w-[400px]" : "w-0"
        } overflow-hidden`}
      >
        {/* 头部 */}
        <div className="relative flex items-center justify-between border-b border-border/60 px-4 py-3">
          <div className="flex items-center gap-2.5 overflow-hidden">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-brand-gradient shadow-glow">
              <Sparkles className="h-4 w-4 text-white" />
            </div>
            <div className="overflow-hidden">
              <h2 className="truncate text-[13px] font-semibold tracking-tight">AI 数据助手</h2>
              <p className="truncate text-[11px] text-foreground-muted">pi-ai-go Agent · 工具调用</p>
            </div>
          </div>
          <button
            onClick={onToggle}
            className="inline-flex h-8 w-8 items-center justify-center rounded-md text-foreground-muted transition-colors hover:bg-secondary hover:text-foreground"
            title="收起侧边栏"
          >
            <PanelRightClose className="h-4 w-4" />
          </button>
        </div>

        {/* 对话区 */}
        <Conversation className="min-h-0">
          {messages.length === 0 ? (
            <ConversationEmptyState
              title="AI 数据助手"
              description={
                dataLoaded
                  ? "描述你想做的分析，AI 会调用工具查询数据并生成图表"
                  : "请先加载数据文件"
              }
            />
          ) : (
            messages.map((m) => (
              <Message key={m.id} from={m.role}>
                {/* 思考块 */}
                {m.thinking && <ThinkingBlock content={m.thinking} />}
                {/* 工具调用步骤 */}
                {m.steps && m.steps.length > 0 && (
                  <div className="mb-1.5 flex flex-col gap-1">
                    {m.steps.map((step, i) => {
                      if (step.type === "tool_call") {
                        return <ToolCallStep key={i} step={step} />;
                      }
                      if (step.type === "tool_result") {
                        return <ToolResultStep key={i} result={step.content} />;
                      }
                      return null;
                    })}
                  </div>
                )}
                {/* 内联图表 — 支持一条消息中生成多张图 */}
                {m.chartBlocks && m.chartBlocks.length > 0 && m.chartBlocks.map((block, i) => (
                  <ChartBlock key={i} chartConfig={block.config} chartData={block.data} />
                ))}
                {/* 文本内容 */}
                {m.content && (
                  <MessageContent from={m.role}>{m.content}</MessageContent>
                )}
                {/* 流式占位：没有文本、没有工具调用、没有思考、没有图表时显示 */}
                {m.streaming && !m.content && (!m.steps || m.steps.length === 0) && !m.thinking && (!m.chartBlocks || m.chartBlocks.length === 0) && (
                  <div className="flex items-center gap-2 text-sm text-foreground-muted">
                    <div className="h-2 w-2 animate-pulse rounded-full bg-primary" />
                    <span className="typing-indicator">AI 正在思考</span>
                  </div>
                )}
                {m.role === "assistant" && m.content && !m.streaming && (
                  <MessageActions className="mt-1">
                    <MessageAction icon={Copy} label="复制" onClick={() => onCopyLast?.()} />
                    <MessageAction icon={RefreshCw} label="重新生成" onClick={() => onRegenerate?.()} />
                  </MessageActions>
                )}
              </Message>
            ))
          )}
          {isLoading && messages.length > 0 && messages[messages.length - 1]?.role === "user" && (
            <Message from="assistant">
              <div className="flex items-center gap-2 text-sm text-foreground-muted">
                <div className="h-2 w-2 animate-pulse rounded-full bg-primary" />
                <span className="typing-indicator">AI 正在分析数据</span>
              </div>
            </Message>
          )}
        </Conversation>

        {/* 输入区 */}
        <PromptInput
          value={undefined}
          onChange={undefined}
          onSubmit={onSend}
          isLoading={isLoading}
          placeholder={dataLoaded ? "描述你想做的分析..." : "请先加载数据"}
          suggestions={
            dataLoaded
              ? ["分析这份数据的概况", "绘制销售额的折线图", "显示数值列的分布直方图", "对比各分类的柱状图"]
              : []
          }
        />

        {/* 底部工具栏 */}
        <div className="flex items-center justify-between border-t border-border/60 px-3 py-1.5 text-[11px] text-foreground-muted">
          <span>{messages.length} 条消息</span>
          <div className="flex items-center gap-2">
            {isLoading && (
              <button
                onClick={onCancel}
                className="inline-flex items-center gap-1 transition-colors hover:text-danger"
                title="停止生成"
              >
                <Square className="h-3 w-3" />
                停止
              </button>
            )}
            {messages.length > 0 && (
              <button
                onClick={onClear}
                className="inline-flex items-center gap-1 transition-colors hover:text-foreground"
              >
                <Trash2 className="h-3 w-3" />
                清空
              </button>
            )}
          </div>
        </div>
      </aside>
    </>
  );
}
