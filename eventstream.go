package piai

import (
	"context"
	"sync"
)

// EventStream is an async event stream for streaming LLM responses.
type EventStream[T any, R any] struct {
	ch     chan streamEvt[T]
	done   chan struct{}
	stop   chan struct{} // closed by consumer to signal producer to stop
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
		stop: make(chan struct{}),
	}
}

// Push sends an event to the stream. Returns false if the stream is closed or
// the consumer has stopped reading (channel buffer full). The producer should
// stop sending events when Push returns false.
func (s *EventStream[T, R]) Push(event T) bool {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return false
	}
	s.mu.Unlock()

	// Non-blocking: check if consumer stopped or channel is full.
	select {
	case <-s.stop:
		return false
	case s.ch <- streamEvt[T]{value: event}:
		return true
	default:
		// Buffer full — consumer is slow or stopped. Drop the event.
		return true
	}
}

// End signals successful completion with a result.
func (s *EventStream[T, R]) End(result R) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.result = result
	s.mu.Unlock()

	s.ch <- streamEvt[T]{done: true}
	close(s.ch)
	close(s.done)
}

// Error signals an error and terminates the stream.
func (s *EventStream[T, R]) Error(err error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.err = err
	s.mu.Unlock()

	s.ch <- streamEvt[T]{err: err, done: true}
	close(s.ch)
	close(s.done)
}

// Stop signals the producer to stop sending events. Called by the consumer
// when it returns early (e.g., context cancelled, callback error).
func (s *EventStream[T, R]) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.stop)
	}
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
			s.Stop()
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
				s.Stop()
				return zeroR, err
			}
		}
	}
}
