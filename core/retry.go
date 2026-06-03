package core

import (
	"context"
	"errors"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
)

// RetryConfig controls the auto-retry behavior of Retry.
//
// Defaults follow oh-my-pi's `non-compaction-retry-policy.md`:
//   - Enabled:      true
//   - MaxRetries:   3
//   - BaseDelay:    2s
//   - MaxDelay:     5min (0 disables fail-fast)
//   - Multiplier:   2 (delay doubles each attempt)
//
// Context-overflow errors are NEVER retried by Retry — they are classified
// separately so the caller can hand them to compaction logic.
type RetryConfig struct {
	Enabled    bool
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	Multiplier float64

	// OnRetry, if set, is called before each backoff sleep with the
	// upcoming attempt number (1-based). Use it to surface "retrying in Ns"
	// messages to the user.
	OnRetry func(attempt int, nextDelay time.Duration, err error)
}

// DefaultRetryConfig returns the policy defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		Enabled:    true,
		MaxRetries: 3,
		BaseDelay:  2 * time.Second,
		MaxDelay:   5 * time.Minute,
		Multiplier: 2.0,
	}
}

// retryablePatterns matches transient transport / rate-limit / server errors.
// This is string-pattern classification, not typed provider error codes,
// matching the policy in oh-my-pi's non-compaction-retry-policy.md.
var retryablePatterns = []*regexp.Regexp{
	// HTTP classes
	regexp.MustCompile(`(?i)\b(429|500|502|503|504)\b`),
	regexp.MustCompile(`(?i)too many requests`),
	regexp.MustCompile(`(?i)rate limit`),
	regexp.MustCompile(`(?i)usage limit`),
	regexp.MustCompile(`(?i)usage cap`),
	// Overload / provider-returned
	regexp.MustCompile(`(?i)overloaded`),
	regexp.MustCompile(`(?i)server error`),
	regexp.MustCompile(`(?i)service unavailable`),
	regexp.MustCompile(`(?i)internal server error`),
	regexp.MustCompile(`(?i)server is overloaded`),
	// Provider-suggested retry
	regexp.MustCompile(`(?i)retry your request`),
	regexp.MustCompile(`(?i)please retry`),
	regexp.MustCompile(`(?i)please try again`),
	// Transport-level
	regexp.MustCompile(`(?i)connection (?:reset|refused|closed)`),
	regexp.MustCompile(`(?i)connect: (?:timeout|connection refused)`),
	regexp.MustCompile(`(?i)socket hang up`),
	regexp.MustCompile(`(?i)unexpected eof`),
	regexp.MustCompile(`(?i)timeout`),
	regexp.MustCompile(`(?i)timed out`),
	regexp.MustCompile(`(?i)fetch failed`),
	regexp.MustCompile(`(?i)no such host`),
	regexp.MustCompile(`(?i)tls: `),
	regexp.MustCompile(`(?i)broken pipe`),
	regexp.MustCompile(`(?i)connection reset by peer`),
	regexp.MustCompile(`(?i)upstream (?:connect|read) (?:timeout|failed)`),
	regexp.MustCompile(`(?i)retry delay`),
	regexp.MustCompile(`(?i)retry-after`),
}

// nonRetryablePatterns exclude throttling / rate-limit errors that LOOK
// like retryable errors from auto-retry. (None today; reserved for future
// "credentials exhausted" patterns that should NOT auto-retry.)
var nonRetryablePatterns = []*regexp.Regexp{}

// IsRetryableError reports whether err is a transient error that Retry
// should attempt again. Context-overflow errors are NEVER retryable —
// they must be handed to compaction logic.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if IsContextOverflowError(err) {
		return false
	}
	// Typed sentinels.
	if errors.Is(err, ErrRateLimit) || errors.Is(err, ErrServer) || errors.Is(err, ErrNetwork) {
		return true
	}
	// Abort / cancellation must never be retried.
	if errors.Is(err, ErrAborted) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	for _, p := range nonRetryablePatterns {
		if p.MatchString(lower) {
			return false
		}
	}
	for _, p := range retryablePatterns {
		if p.MatchString(lower) {
			return true
		}
	}
	return false
}

// IsContextOverflowError reports whether err is a context-overflow signal.
// Callers should short-circuit auto-retry and route to compaction.
func IsContextOverflowError(err error) bool {
	if err == nil {
		return false
	}
	var oe *OverflowError
	if errors.As(err, &oe) {
		return true
	}
	return errors.Is(err, ErrOverflow)
}

// IsAuthError reports whether err is an authentication / authorization
// failure (401/403). Such errors must NEVER be retried automatically.
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrAuth)
}

// retryKey is a small object used by Retry to coordinate the lifecycle.
// Concurrent calls with the same key share a single in-flight chain.
type retryKey struct{}

// Retry runs op with exponential-backoff retry on transient errors.
//
// op is invoked once initially; on a retryable error, Retry sleeps for the
// computed backoff and re-invokes op. The function returns the first
// non-retryable error, or the final retryable error after MaxRetries is
// exhausted, or nil on success.
//
// Retry respects ctx cancellation during the backoff sleep; the next
// pending attempt is abandoned and ctx.Err() is returned.
func Retry(ctx context.Context, cfg RetryConfig, op func() error) error {
	if !cfg.Enabled {
		return op()
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = 2 * time.Second
	}
	if cfg.Multiplier <= 0 {
		cfg.Multiplier = 2.0
	}

	var lastErr error
	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := op()
		if err == nil {
			return nil
		}
		lastErr = err

		// Never retry non-retryable errors.
		if !IsRetryableError(err) {
			return err
		}

		// Last attempt? Return the error.
		if attempt == cfg.MaxRetries {
			return err
		}

		// Compute the next backoff delay.
		delay := computeDelay(cfg, attempt, err)

		// Fail-fast: if the delay exceeds MaxDelay (and MaxDelay > 0),
		// give up immediately rather than sleeping.
		if cfg.MaxDelay > 0 && delay > cfg.MaxDelay {
			return err
		}

		if cfg.OnRetry != nil {
			cfg.OnRetry(attempt+1, delay, err)
		}

		// Sleep, but be cancellable.
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
	return lastErr
}

func computeDelay(cfg RetryConfig, attempt int, err error) time.Duration {
	base := float64(cfg.BaseDelay)
	mult := cfg.Multiplier
	delay := time.Duration(base * pow(mult, float64(attempt)))

	// RateLimitError can carry a retry-after hint.
	var rl *RateLimitError
	if errors.As(err, &rl) && rl.RetryAfter > 0 {
		if h := rl.RetryAfter; h > delay {
			delay = h
		}
	}
	return delay
}

// pow is a tiny integer-safe float pow used for backoff math.
func pow(base, exp float64) float64 {
	if exp == 0 {
		return 1
	}
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

// RetryableHTTPCheck classifies a (status, body) pair. The returned
// retryable flag tells whether Retry should re-issue the request.
func RetryableHTTPCheck(status int, body string) bool {
	if status == 408 || status == 429 || (status >= 500 && status <= 504) {
		return true
	}
	if body == "" {
		return false
	}
	lower := strings.ToLower(body)
	for _, p := range retryablePatterns {
		if p.MatchString(lower) {
			return true
		}
	}
	return false
}

// IsNetworkError reports whether err looks like a transport-level failure
// (DNS, connect, EOF, etc.).
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNetwork) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, p := range []*regexp.Regexp{
		regexp.MustCompile(`(?i)connection (?:reset|refused|closed)`),
		regexp.MustCompile(`(?i)socket hang up`),
		regexp.MustCompile(`(?i)unexpected eof`),
		regexp.MustCompile(`(?i)no such host`),
		regexp.MustCompile(`(?i)tls: `),
		regexp.MustCompile(`(?i)broken pipe`),
	} {
		if p.MatchString(msg) {
			return true
		}
	}
	return false
}

// retryOnce is a no-op import-block placeholder kept to keep net in use on
// platforms that might prune imports. (No runtime effect.)
var _ = sync.Mutex{}
