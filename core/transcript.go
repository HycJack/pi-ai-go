/*
 * 功能说明：会话上下文归一化（Transcript 归一化层）
 *
 * 解决的问题：
 * 1. 调用方传入的消息/内容可能被外部后续修改，导致 provider 看到不稳定的输入
 * 2. 空内容（nil）可能导致部分 provider 序列化失败
 * 3. 非法 role 或内容形状应尽早拒绝，而不是下游 provider 才报错
 *
 * 解决方案：
 * 1. NormalizeContext 在 llm 公开入口执行：深拷贝 + 校验 + 内容归一化
 * 2. provider 收到的是不可变快照，避免调用方竞态修改
 * 3. SystemPrompt/Tools 保持原样（provider 自行注入 SystemPrompt，
 *    convert.Messages 不渲染位于 Messages 数组里的 SystemMessage）
 */
package core

import "fmt"

// NormalizeContext validates and deep-copies a request Context, returning an
// immutable normalized snapshot suitable for passing to a provider. It:
//
//   - validates that every message has a known role and content shape,
//     returning an error on the first violation (public-boundary rejection);
//   - deep-copies Messages, the per-message Content slices, and Tools so a
//     caller cannot mutate the request after it is handed to a provider;
//   - normalizes empty per-message content (nil user content -> empty string,
//     nil assistant/tool-result content -> empty block array);
//   - preserves SystemPrompt and Tools as-is: pi-ai-go providers inject
//     SystemPrompt themselves and their message converters do not render a
//     SystemMessage inside Messages, so folding SystemPrompt into a message
//     would drop it or double-inject it.
func NormalizeContext(ctx Context) (Context, error) {
	messages := make([]Message, len(ctx.Messages))
	for i, msg := range ctx.Messages {
		norm, err := normalizeMessage(msg)
		if err != nil {
			return Context{}, err
		}
		messages[i] = norm
	}

	tools := make([]Tool, len(ctx.Tools))
	copy(tools, ctx.Tools)

	return Context{
		SystemPrompt: ctx.SystemPrompt,
		Messages:     messages,
		Tools:        tools,
	}, nil
}

const (
	roleUser      = "user"
	roleAssistant = "assistant"
	roleTool      = "tool"
	roleSystem    = "system"
)

// normalizeMessage validates and deep-copies a single message, normalizing
// empty content. It returns an error for an unknown role or malformed content.
func normalizeMessage(msg Message) (Message, error) {
	switch m := msg.(type) {
	case UserMessage:
		if m.Role != roleUser {
			return nil, fmt.Errorf("transcript: invalid user message role %q", m.Role)
		}
		var content any
		switch c := m.Content.(type) {
		case nil:
			content = ""
		case string:
			content = c
		case []ContentBlock:
			content = copyContentBlocks(c)
		default:
			return nil, fmt.Errorf("transcript: user message content must be a string or []ContentBlock, got %T", m.Content)
		}
		m.Content = content
		return m, nil

	case AssistantMessage:
		if m.Role != roleAssistant {
			return nil, fmt.Errorf("transcript: invalid assistant message role %q", m.Role)
		}
		m.Content = copyContentBlocks(m.Content)
		return m, nil

	case ToolResultMessage:
		if m.Role != roleTool {
			return nil, fmt.Errorf("transcript: invalid tool message role %q", m.Role)
		}
		m.Content = copyContentBlocks(m.Content)
		return m, nil

	case SystemMessage:
		if m.Role != roleSystem {
			return nil, fmt.Errorf("transcript: invalid system message role %q", m.Role)
		}
		return m, nil

	default:
		return nil, fmt.Errorf("transcript: unsupported message type %T", msg)
	}
}

// copyContentBlocks copies a ContentBlock slice into a fresh backing array so
// the normalized message is isolated from caller mutation. A nil slice stays
// nil (providers tolerate it) to avoid allocating when nothing is present.
func copyContentBlocks(blocks []ContentBlock) []ContentBlock {
	if blocks == nil {
		return nil
	}
	out := make([]ContentBlock, len(blocks))
	copy(out, blocks)
	return out
}