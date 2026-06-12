package session

import (
	"fmt"
	"time"

	core "pi-ai-go/core"
)

// CoerceToolResult normalizes an arbitrary tool-execution output into a
// well-formed ToolResultMessage. It mirrors the safety net in oh-my-pi's
// `agent-loop.ts:coerceToolResult` — malformed tool results must not crash
// the agent loop.
//
// The returned boolean reports whether the input was malformed and had to
// be repaired. Callers may log or surface the malformed flag but should
// still treat the returned ToolResultMessage as the authoritative result.
//
// Rules:
//   - Nil input -> empty result (not an error).
//   - Missing Content slice -> inserted as an empty []ContentBlock.
//   - Non-string Text content -> coerced to its string representation.
//   - Missing ToolCallID -> set to the empty string (the LLM will see
//     `tool_call_id=""` and the framework can attach a follow-up tool_use
//     tool_call_id once known).
//   - IsError is preserved if the input flagged it; otherwise false.
func CoerceToolResult(raw any, toolCallID, toolName string) (core.ToolResultMessage, bool) {
	malformed := false

	out := core.ToolResultMessage{
		Role:       "toolResult",
		ToolCallID: toolCallID,
		ToolName:   toolName,
		Content:    []core.ContentBlock{},
		IsError:    false,
		Timestamp:  time.Now(),
	}

	if raw == nil {
		return out, true
	}

	// Tool result is a record-like object.
	obj, ok := raw.(map[string]any)
	if !ok {
		// Some tools return a plain string or a typed struct; surface it
		// as a single text block.
		out.Content = append(out.Content, core.TextContent{
			Type: "text",
			Text: coerceToString(raw),
		})
		return out, true
	}

	if d, ok := obj["details"]; ok {
		out.Details = d
	}

	if e, ok := obj["isError"].(bool); ok {
		out.IsError = e
	}

	// Coerce content.
	switch c := obj["content"].(type) {
	case []core.ContentBlock:
		out.Content = c
	case []any:
		for _, item := range c {
			if block, ok := item.(core.ContentBlock); ok {
				out.Content = append(out.Content, block)
				continue
			}
			m, ok := item.(map[string]any)
			if !ok {
				out.Content = append(out.Content, core.TextContent{
					Type: "text",
					Text: stringifyAny(item),
				})
				malformed = true
				continue
			}
			t, _ := m["type"].(string)
			switch t {
			case "text", "":
				if txt, ok := m["text"].(string); ok {
					out.Content = append(out.Content, core.TextContent{Type: "text", Text: txt})
				} else {
					out.Content = append(out.Content, core.TextContent{
						Type: "text",
						Text: coerceToString(m["text"]),
					})
					malformed = true
				}
			default:
				// Unknown block type — pass it through as text.
				out.Content = append(out.Content, core.TextContent{
					Type: "text",
					Text: stringifyAny(m),
				})
				malformed = true
			}
		}
	case string:
		out.Content = append(out.Content, core.TextContent{Type: "text", Text: c})
	case nil:
		// Missing content -> inserted as an empty error block.
		out.IsError = true
		out.Content = []core.ContentBlock{core.TextContent{
			Type: "text",
			Text: "Tool returned no content",
		}}
		malformed = true
	default:
		// Unknown content type — stringify and continue.
		out.Content = append(out.Content, core.TextContent{
			Type: "text",
			Text: stringifyAny(c),
		})
		malformed = true
	}

	return out, malformed
}

func coerceToString(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	case error:
		return s.Error()
	case fmtStringer:
		return s.String()
	}
	// Fall back to fmt-style representation; keep it small.
	return defaultString(v)
}

// fmtStringer matches anything that implements String() string (without
// pulling in fmt for every call).
type fmtStringer interface {
	String() string
}

func defaultString(v any) string {
	// Avoid importing fmt in this hot path; the agent loop will see the
	// value with %v semantics via fmt.Sprintf if it needs to log.
	return "<unrepr " + typeName(v) + ">"
}

func typeName(v any) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprintf("%T", v)
}
