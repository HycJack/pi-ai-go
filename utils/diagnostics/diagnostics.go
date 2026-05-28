// Package diagnostics provides diagnostic utilities for tracking message processing.
package diagnostics

import (
	"fmt"
	"time"
)

// Diagnostic represents a diagnostic event during message processing.
type Diagnostic struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
	Details   any       `json:"details,omitempty"`
}

// New creates a new diagnostic with the given type and optional details.
func New(diagType string, details ...any) Diagnostic {
	d := Diagnostic{
		Type:      diagType,
		Timestamp: time.Now(),
	}
	if len(details) > 0 {
		d.Details = details[0]
	}
	return d
}

// NewWithError creates a new diagnostic with an error.
func NewWithError(diagType string, err error) Diagnostic {
	return Diagnostic{
		Type:      diagType,
		Timestamp: time.Now(),
		Error:     err.Error(),
	}
}

// ExtractError returns the error string from a diagnostic, or empty string.
func ExtractError(d Diagnostic) string {
	return d.Error
}

// FormatThrownValue safely formats any value as a string for error reporting.
func FormatThrownValue(v any) string {
	if v == nil {
		return "<nil>"
	}
	switch val := v.(type) {
	case error:
		return val.Error()
	case string:
		return val
	case fmt.Stringer:
		return val.String()
	default:
		return fmt.Sprintf("%v", val)
	}
}
