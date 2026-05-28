// Package jsonparse provides JSON parsing with repair capabilities.
package jsonparse

import (
	"encoding/json"
	"strings"
	"unicode"
)

// Parse attempts to parse JSON, falling back to repair if standard parsing fails.
func Parse[T any](data string) (T, error) {
	var zero T
	var result T
	if err := json.Unmarshal([]byte(data), &result); err == nil {
		return result, nil
	}
	repaired := Repair(data)
	if err := json.Unmarshal([]byte(repaired), &result); err != nil {
		return zero, err
	}
	return result, nil
}

// Repair fixes common JSON malformation issues.
func Repair(s string) string {
	// Fix raw control characters in strings
	s = fixControlChars(s)
	// Fix bad escape sequences
	s = fixBadEscapes(s)
	return s
}

// fixControlChars replaces unescaped control characters within JSON strings.
func fixControlChars(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))
	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if escaped {
			buf.WriteByte(ch)
			escaped = false
			continue
		}

		if ch == '\\' && inString {
			buf.WriteByte(ch)
			escaped = true
			continue
		}

		if ch == '"' {
			inString = !inString
			buf.WriteByte(ch)
			continue
		}

		if inString && ch < 0x20 {
			// Escape control characters
			switch ch {
			case '\n':
				buf.WriteString("\\n")
			case '\r':
				buf.WriteString("\\r")
			case '\t':
				buf.WriteString("\\t")
			case '\b':
				buf.WriteString("\\b")
			case '\f':
				buf.WriteString("\\f")
			default:
				buf.WriteString("\\u00")
				buf.WriteByte("0123456789abcdef"[ch>>4])
				buf.WriteByte("0123456789abcdef"[ch&0x0f])
			}
			continue
		}

		buf.WriteByte(ch)
	}
	return buf.String()
}

// fixBadEscapes fixes invalid escape sequences in JSON strings.
func fixBadEscapes(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))
	inString := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if ch == '\\' && inString {
			if i+1 < len(s) {
				next := s[i+1]
				switch next {
				case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
					buf.WriteByte(ch)
					buf.WriteByte(next)
					i++
					continue
				case 'u':
					// Validate \uXXXX
					if i+5 < len(s) {
						valid := true
						for j := i + 2; j < i+6; j++ {
							if !isHex(s[j]) {
								valid = false
								break
							}
						}
						if valid {
							buf.WriteString(s[i : i+6])
							i += 5
							continue
						}
					}
					// Invalid \u escape, skip the backslash
					buf.WriteByte(next)
					i++
					continue
				default:
					// Unknown escape, remove the backslash
					buf.WriteByte(next)
					i++
					continue
				}
			}
		}

		if ch == '"' {
			inString = !inString
		}

		buf.WriteByte(ch)
	}
	return buf.String()
}

func isHex(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

// Streaming parses a potentially incomplete JSON string (e.g., during streaming).
// It tries multiple strategies: complete JSON, partial JSON repair, and relaxed parsing.
func Streaming[T any](partial string) (T, bool) {
	var zero T

	// Try complete parse first
	var result T
	if err := json.Unmarshal([]byte(partial), &result); err == nil {
		return result, true
	}

	// Try to complete the JSON by closing open structures
	completed := completeJSON(partial)
	if err := json.Unmarshal([]byte(completed), &result); err == nil {
		return result, true
	}

	// Try with repair
	repaired := Repair(partial)
	completed = completeJSON(repaired)
	if err := json.Unmarshal([]byte(completed), &result); err == nil {
		return result, true
	}

	return zero, false
}

// completeJSON attempts to close open JSON structures.
func completeJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "{}"
	}

	// Track open brackets/braces
	var stack []byte
	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' && inString {
			escaped = true
			continue
		}

		if ch == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		switch ch {
		case '{', '[':
			stack = append(stack, ch)
		case '}':
			if len(stack) > 0 && stack[len(stack)-1] == '{' {
				stack = stack[:len(stack)-1]
			}
		case ']':
			if len(stack) > 0 && stack[len(stack)-1] == '[' {
				stack = stack[:len(stack)-1]
			}
		}
	}

	// Build result, trimming trailing commas and incomplete values
	var buf strings.Builder
	buf.WriteString(s)

	// Trim trailing comma/whitespace before closing
	result := buf.String()
	result = strings.TrimRight(result, " \t\n\r")

	// Handle trailing comma
	if len(result) > 0 && result[len(result)-1] == ',' {
		result = result[:len(result)-1]
	}

	// Handle trailing colon (incomplete value)
	if len(result) > 0 && result[len(result)-1] == ':' {
		result += "null"
	}

	// If we're in a string, close it
	if inString {
		result += "\""
	}

	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i] {
		case '{':
			result += "}"
		case '[':
			result += "]"
		}
	}

	return result
}

// TrimTrailingComma removes trailing commas before closing brackets/braces.
func TrimTrailingComma(s string) string {
	s = strings.TrimRightFunc(s, unicode.IsSpace)
	if len(s) > 0 && s[len(s)-1] == ',' {
		s = s[:len(s)-1]
	}
	return s
}
