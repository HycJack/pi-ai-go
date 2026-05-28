// Package sanitize provides Unicode sanitization utilities.
package sanitize

import "unicode/utf8"

// Surrogates removes unpaired Unicode surrogate characters from text.
// This prevents JSON serialization errors caused by invalid UTF-8 sequences.
func Surrogates(text string) string {
	// Fast path: if the string is valid UTF-8, return as-is.
	if utf8.ValidString(text) {
		return text
	}

	buf := make([]byte, 0, len(text))
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError && size == 1 {
			// Skip invalid byte
			i++
			continue
		}
		buf = utf8.AppendRune(buf, r)
		i += size
	}
	return string(buf)
}
