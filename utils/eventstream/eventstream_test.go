package eventstream

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStreamPushAndEnd(t *testing.T) {
	s := New[string, int]()

	s.Push("a")
	s.Push("b")
	s.Push("c")
	s.End(42)

	result, err := s.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Errorf("expected result 42, got %d", result)
	}

	// Drain events
	events := <-s.Events()
	if events.value != "a" {
		t.Errorf("expected 'a', got %s", events.value)
	}
	events = <-s.Events()
	if events.value != "b" {
		t.Errorf("expected 'b', got %s", events.value)
	}
	events = <-s.Events()
	if events.value != "c" {
		t.Errorf("expected 'c', got %s", events.value)
	}
}

func TestStreamError(t *testing.T) {
	s := New[string, int]()

	expectedErr := errors.New("test error")
	s.Error(expectedErr)

	_, err := s.Result()
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestStreamForEach(t *testing.T) {
	s := New[string, int]()

	go func() {
		s.Push("a")
		s.Push("b")
		s.End(42)
	}()

	var collected []string
	result, err := ForEach(context.Background(), s, func(v string) error {
		collected = append(collected, v)
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Errorf("expected result 42, got %d", result)
	}
	if len(collected) != 2 {
		t.Errorf("expected 2 events, got %d", len(collected))
	}
}

func TestStreamForEachError(t *testing.T) {
	s := New[string, int]()

	go func() {
		s.Push("a")
	}()

	expectedErr := errors.New("callback error")
	_, err := ForEach(context.Background(), s, func(v string) error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestStreamContextCancel(t *testing.T) {
	s := New[string, int]()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := ForEach(ctx, s, func(v string) error {
		return nil
	})

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestStreamCollect(t *testing.T) {
	s := New[string, int]()

	go func() {
		s.Push("x")
		s.Push("y")
		s.End(99)
	}()

	events, result, err := Collect(context.Background(), s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 99 {
		t.Errorf("expected result 99, got %d", result)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestStreamDoubleEnd(t *testing.T) {
	s := New[string, int]()

	s.End(1)
	s.End(2) // Should be no-op

	result, _ := s.Result()
	if result != 1 {
		t.Errorf("expected result 1, got %d", result)
	}
}

func TestStreamPushAfterEnd(t *testing.T) {
	s := New[string, int]()

	s.End(1)
	s.Push("after") // Should be no-op

	result, _ := s.Result()
	if result != 1 {
		t.Errorf("expected result 1, got %d", result)
	}
}

func TestStreamTimeout(t *testing.T) {
	s := New[string, int]()

	// Never close the stream
	done := make(chan struct{})
	go func() {
		_, _ = s.Result()
		close(done)
	}()

	select {
	case <-done:
		t.Error("Result should block on unclosed stream")
	case <-time.After(50 * time.Millisecond):
		// Expected: blocks
	}
}
