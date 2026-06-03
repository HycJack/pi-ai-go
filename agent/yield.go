package agent

import (
	"context"
	"sync/atomic"
	"time"
)

// YieldConfig controls the cooperative scheduling checkpoint. The agent
// loop calls yieldIfDue between heavy operations (between LLM calls,
// between tool calls, between tool result ingestion) so that:
//   - context cancellation can be observed without waiting for an LLM
//     stream to drain, and
//   - background telemetry / metrics work gets a chance to flush.
//
// Mirrors oh-my-pi's `utils/yield.ts` policy.
type YieldConfig struct {
	// Interval is the minimum wall-clock gap between yields. Yields are
	// throttled so a tight loop is not slowed down by a yield every
	// iteration. Defaults to 50ms.
	Interval time.Duration
	// OnYield, if set, runs on each yield. Use it to flush telemetry, or
	// to do any other background bookkeeping.
	OnYield func()
	// now is overridable for testing; nil means time.Now.
	now func() time.Time
}

func (c *YieldConfig) interval() time.Duration {
	if c.Interval <= 0 {
		return 50 * time.Millisecond
	}
	return c.Interval
}

func (c *YieldConfig) clock() func() time.Time {
	if c.now != nil {
		return c.now
	}
	return time.Now
}

// Yielder is the runtime side of YieldConfig. It tracks the last yield
// time atomically so it is safe to call Yield from any goroutine that
// owns the agent loop.
type Yielder struct {
	cfg     YieldConfig
	lastNs  atomic.Int64
}

// NewYielder creates a Yielder. The first call to Yield always yields
// (so the loop starts in a "ready to cancel" state).
func NewYielder(cfg YieldConfig) *Yielder {
	return &Yielder{cfg: cfg}
}

// YieldIfDue checks the elapsed time since the last yield. If it is
// greater than the configured interval, it blocks on ctx.Done() (which
// returns when the run is cancelled) and then resets the timer.
//
// Returns ctx.Err() if the context was cancelled, otherwise nil.
func (y *Yielder) YieldIfDue(ctx context.Context) error {
	if y == nil {
		return ctx.Err()
	}
	now := y.cfg.clock()()
	last := time.Unix(0, y.lastNs.Load())
	if !last.IsZero() && now.Sub(last) < y.cfg.interval() {
		return nil
	}
	return y.yield(ctx)
}

func (y *Yielder) yield(ctx context.Context) error {
	if y.cfg.OnYield != nil {
		y.cfg.OnYield()
	}
	// Record the yield *before* waiting so a second caller racing
	// against the first sees the update.
	y.lastNs.Store(y.cfg.clock()().UnixNano())
	// Cooperative wait on ctx. We do not sleep — we just observe the
	// cancel state and reset the timer so the next call can yield again.
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// Reset clears the last-yield timestamp. Call it at the start of a new
// run so the first yieldIfDue fires.
func (y *Yielder) Reset() {
	if y == nil {
		return
	}
	y.lastNs.Store(0)
}
