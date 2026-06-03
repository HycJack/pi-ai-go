package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Sentinel error reasons used by classification helpers. They are exposed as
// plain error values so callers can use errors.Is.
var (
	// ErrOverflow is a generic context-overflow marker. Prefer *OverflowError
	// for richer context.
	ErrOverflow = errors.New("context overflow")

	// ErrAborted indicates the caller cancelled the operation.
	ErrAborted = errors.New("operation aborted")

	// ErrAuth indicates an authentication / authorization failure.
	ErrAuth = errors.New("authentication failed")

	// ErrRateLimit indicates the provider rate-limited the request.
	ErrRateLimit = errors.New("rate limited")

	// ErrServer indicates a transient server-side failure.
	ErrServer = errors.New("server error")

	// ErrNetwork indicates a transport / connection failure.
	ErrNetwork = errors.New("network error")
)

// OverflowError is the typed signal raised when a request exceeds the
// provider's context window. Downstream code can detect it via
// errors.As(err, &oe) and inspect the numeric context.
type OverflowError struct {
	// Provider is the provider that returned the overflow.
	Provider KnownProvider
	// Message is the underlying provider error text (may be empty for
	// silent-overflow cases detected from usage).
	Message string
	// ContextWindow is the model's context window (0 if unknown).
	ContextWindow int
	// Usage is the input token count that overflowed (0 if unknown).
	Usage int
}

func (e *OverflowError) Error() string {
	switch {
	case e.Message != "":
		return fmt.Sprintf("%s: context overflow: %s", e.Provider, e.Message)
	case e.ContextWindow > 0 && e.Usage > 0:
		return fmt.Sprintf("%s: context overflow: usage %d > window %d", e.Provider, e.Usage, e.ContextWindow)
	default:
		return fmt.Sprintf("%s: context overflow", e.Provider)
	}
}

func (e *OverflowError) Is(target error) bool { return target == ErrOverflow }

// Unwrap allows errors.Is to recognize *OverflowError.
func (e *OverflowError) Unwrap() error { return ErrOverflow }

// CompactionCancelledError is the typed signal raised when a compaction is
// explicitly aborted. It mirrors oh-my-pi's `CompactionCancelledError` so
// callers can discriminate cancellation from other failures via errors.As
// rather than string matching.
type CompactionCancelledError struct {
	Reason string
}

func (e *CompactionCancelledError) Error() string {
	if e.Reason == "" {
		return "compaction cancelled"
	}
	return "compaction cancelled: " + e.Reason
}

// AbortError signals the consumer cancelled the in-flight operation.
type AbortError struct {
	Cause error
}

func (e *AbortError) Error() string {
	if e.Cause == nil {
		return "operation aborted"
	}
	return "operation aborted: " + e.Cause.Error()
}

func (e *AbortError) Unwrap() error { return e.Cause }
func (e *AbortError) Is(t error) bool {
	return t == ErrAborted || t == context.Canceled
}

// AuthError wraps a 401/403 from the provider.
type AuthError struct {
	Provider KnownProvider
	Cause    error
}

func (e *AuthError) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s: authentication failed", e.Provider)
	}
	return fmt.Sprintf("%s: authentication failed: %v", e.Provider, e.Cause)
}

func (e *AuthError) Unwrap() error { return e.Cause }
func (e *AuthError) Is(t error) bool { return t == ErrAuth }

// RateLimitError wraps a 429 with optional retry-after hint.
type RateLimitError struct {
	Provider   KnownProvider
	RetryAfter time.Duration
	Cause      error
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("%s: rate limited (retry after %s)", e.Provider, e.RetryAfter)
	}
	if e.Cause == nil {
		return fmt.Sprintf("%s: rate limited", e.Provider)
	}
	return fmt.Sprintf("%s: rate limited: %v", e.Provider, e.Cause)
}

func (e *RateLimitError) Unwrap() error { return e.Cause }
func (e *RateLimitError) Is(t error) bool { return t == ErrRateLimit }

// ServerError wraps a 5xx response.
type ServerError struct {
	Provider   KnownProvider
	StatusCode int
	Cause      error
}

func (e *ServerError) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s: server error %d", e.Provider, e.StatusCode)
	}
	return fmt.Sprintf("%s: server error %d: %v", e.Provider, e.StatusCode, e.Cause)
}

func (e *ServerError) Unwrap() error { return e.Cause }
func (e *ServerError) Is(t error) bool { return t == ErrServer }

// NetworkError wraps a transport-level failure (DNS, connect, EOF, etc.).
type NetworkError struct {
	Provider KnownProvider
	Cause    error
}

func (e *NetworkError) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s: network error", e.Provider)
	}
	return fmt.Sprintf("%s: network error: %v", e.Provider, e.Cause)
}

func (e *NetworkError) Unwrap() error { return e.Cause }
func (e *NetworkError) Is(t error) bool { return t == ErrNetwork }

// ClassifyHTTPError maps an HTTP error response to a typed error. The body
// is the error payload returned by the provider. If no classification
// applies, nil is returned and the caller is expected to surface the raw
// error.
func ClassifyHTTPError(provider KnownProvider, status int, body string) error {
	switch {
	case status == http.StatusRequestTimeout, status == http.StatusTooManyRequests:
		return &RateLimitError{Provider: provider, Cause: fmt.Errorf("status %d: %s", status, trimBody(body))}
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return &AuthError{Provider: provider, Cause: fmt.Errorf("status %d: %s", status, trimBody(body))}
	case status >= 500 && status <= 599:
		return &ServerError{Provider: provider, StatusCode: status, Cause: fmt.Errorf("%s", trimBody(body))}
	case status == http.StatusRequestEntityTooLarge:
		return &OverflowError{Provider: provider, Message: trimBody(body)}
	default:
		return nil
	}
}

func trimBody(body string) string {
	body = strings.TrimSpace(body)
	const max = 500
	if len(body) > max {
		return body[:max] + "..."
	}
	return body
}
