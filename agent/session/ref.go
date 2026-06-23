package session

import (
	"strings"
	"unicode"
)

func validateRef(name RefName) error {
	if name == "" || name == "@" || name == "HEAD" {
		return &SessionError{Code: ErrInvalidRef, Message: "invalid ref name: empty, '@', or 'HEAD'"}
	}

	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return &SessionError{Code: ErrInvalidRef, Message: "ref name cannot start or end with '.'"}
	}

	if strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".lock") {
		return &SessionError{Code: ErrInvalidRef, Message: "ref name cannot end with '/' or '.lock'"}
	}

	if strings.Contains(name, "..") || strings.Contains(name, "//") ||
		strings.Contains(name, "/.") || strings.Contains(name, "@{") {
		return &SessionError{Code: ErrInvalidRef, Message: "ref name cannot contain '..', '//', '/.', or '@{'"}
	}

	for _, ch := range name {
		if unicode.IsControl(ch) {
			return &SessionError{Code: ErrInvalidRef, Message: "ref name cannot contain control characters"}
		}
		if strings.ContainsRune(" ~^:?*[]\\", ch) {
			return &SessionError{Code: ErrInvalidRef, Message: "ref name cannot contain ' ', '~', '^', ':', '?', '*', '[', ']', or '\\'"}
		}
	}

	return nil
}