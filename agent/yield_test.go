package agent

import (
	"context"
	"testing"
	"time"
)

func TestYielderFirstCallYields(t *testing.T) {
	y := NewYielder(YieldConfig{Interval: time.Millisecond})
	// The first call should return ctx.Err() if ctx is cancelled,
	// but never block.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := y.YieldIfDue(ctx); err == nil {
		t.Error("expected cancellation error")
	}
}

func TestYielderThrottles(t *testing.T) {
	calls := 0
	y := NewYielder(YieldConfig{
		Interval: 100 * time.Millisecond,
		OnYield:  func() { calls++ },
	})
	ctx := context.Background()
	// First call: yields.
	if err := y.YieldIfDue(ctx); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("expected 1 yield call, got %d", calls)
	}
	// Immediate second call: throttled.
	if err := y.YieldIfDue(ctx); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("expected 1 yield call (throttled), got %d", calls)
	}
}

func TestYielderReset(t *testing.T) {
	calls := 0
	y := NewYielder(YieldConfig{
		Interval: time.Hour,
		OnYield:  func() { calls++ },
	})
	ctx := context.Background()
	_ = y.YieldIfDue(ctx)
	if calls != 1 {
		t.Fatalf("expected 1 yield, got %d", calls)
	}
	_ = y.YieldIfDue(ctx)
	if calls != 1 {
		t.Fatalf("expected 1 yield (throttled), got %d", calls)
	}
	y.Reset()
	_ = y.YieldIfDue(ctx)
	if calls != 2 {
		t.Fatalf("expected 2 yields after reset, got %d", calls)
	}
}

func TestYielderNilSafe(t *testing.T) {
	var y *Yielder
	if err := y.YieldIfDue(context.Background()); err != nil {
		t.Errorf("nil yielder should be safe, got %v", err)
	}
	y.Reset()
}
