package piai

import (
	"context"
	"sync"
)

// EventStream is an async event stream for streaming LLM responses.
type EventStream[T any, R any] struct {
	ch     chan streamEvt[T]
	done   chan struct{}
	result R
	err    error
	closed bool
	mu     sync.Mutex
}

type streamEvt[T any] struct {
	value T
	err   error
	done  bool
}

// NewEventStream creates a new EventStream.
func NewEventStream[T any, R any]() *EventStream[T, R] {
	return &EventStream[T, R]{
		ch:   make(chan streamEvt[T], 64),
		done: make(chan struct{}),
	}
}

// Push sends an event to the stream.
func (s *EventStream[T, R]) Push(event T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.ch <- streamEvt[T]{value: event}
}

// End signals successful completion with a result.
func (s *EventStream[T, R]) End(result R) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.result = result
	s.ch <- streamEvt[T]{done: true}
	close(s.ch)
	close(s.done)
}

// Error signals an error and terminates the stream.
func (s *EventStream[T, R]) Error(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.err = err
	s.ch <- streamEvt[T]{err: err, done: true}
	close(s.ch)
	close(s.done)
}

// Result waits for the stream to complete and returns the final result.
func (s *EventStream[T, R]) Result() (R, error) {
	<-s.done
	return s.result, s.err
}

// Events returns a channel that yields stream events.
func (s *EventStream[T, R]) Events() <-chan streamEvt[T] {
	return s.ch
}

// ForEach iterates over all events in the stream, calling fn for each one.
func (s *EventStream[T, R]) ForEach(ctx context.Context, fn func(T) error) (R, error) {
	var zeroR R
	for {
		select {
		case <-ctx.Done():
			return zeroR, ctx.Err()
		case evt, ok := <-s.Events():
			if !ok {
				return s.Result()
			}
			if evt.done {
				if evt.err != nil {
					return zeroR, evt.err
				}
				return s.Result()
			}
			if err := fn(evt.value); err != nil {
				return zeroR, err
			}
		}
	}
}
