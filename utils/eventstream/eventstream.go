// Package eventstream provides an async event stream for streaming LLM responses.
package eventstream

import (
	"context"
	"errors"
	"sync"
)

// Stream is a generic async event stream that yields events of type T
// and resolves to a result of type R when complete.
type Stream[T any, R any] struct {
	ch     chan streamEvent[T]
	done   chan struct{}
	result R
	err    error
	closed bool
	mu     sync.Mutex
}

type streamEvent[T any] struct {
	value T
	err   error
	done  bool
}

// New creates a new Stream.
func New[T any, R any]() *Stream[T, R] {
	return &Stream[T, R]{
		ch:   make(chan streamEvent[T], 64),
		done: make(chan struct{}),
	}
}

// Push sends an event to the stream.
func (s *Stream[T, R]) Push(event T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.ch <- streamEvent[T]{value: event}
}

// End signals successful completion with a result.
func (s *Stream[T, R]) End(result R) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.result = result
	s.ch <- streamEvent[T]{done: true}
	close(s.ch)
	close(s.done)
}

// Error signals an error and terminates the stream.
func (s *Stream[T, R]) Error(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.err = err
	s.ch <- streamEvent[T]{err: err, done: true}
	close(s.ch)
	close(s.done)
}

// Result waits for the stream to complete and returns the final result.
func (s *Stream[T, R]) Result() (R, error) {
	<-s.done
	return s.result, s.err
}

// Events returns a channel that yields stream events.
// The channel is closed when the stream ends.
func (s *Stream[T, R]) Events() <-chan streamEvent[T] {
	return s.ch
}

// ForEach iterates over all events in the stream, calling fn for each one.
// Returns the final result or the first error encountered.
func ForEach[T any, R any](ctx context.Context, stream *Stream[T, R], fn func(T) error) (R, error) {
	var zero R
	for {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case event, ok := <-stream.Events():
			if !ok {
				return stream.Result()
			}
			if event.done {
				if event.err != nil {
					return zero, event.err
				}
				return stream.Result()
			}
			if err := fn(event.value); err != nil {
				return zero, err
			}
		}
	}
}

// Collect drains all events and returns them along with the final result.
func Collect[T any, R any](ctx context.Context, stream *Stream[T, R]) ([]T, R, error) {
	var events []T
	var zero R
	for {
		select {
		case <-ctx.Done():
			return nil, zero, ctx.Err()
		case event, ok := <-stream.Events():
			if !ok {
				result, err := stream.Result()
				return events, result, err
			}
			if event.done {
				if event.err != nil {
					return nil, zero, event.err
				}
				result, err := stream.Result()
				return events, result, err
			}
			events = append(events, event.value)
		}
	}
}

// ErrStreamClosed is returned when trying to push to a closed stream.
var ErrStreamClosed = errors.New("stream closed")
