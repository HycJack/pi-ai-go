package session

import (
	"sync"
)

type mutationQueue struct {
	mu    sync.Mutex
	ready chan struct{}
}

func newMutationQueue() *mutationQueue {
	q := &mutationQueue{
		ready: make(chan struct{}),
	}
	close(q.ready)
	return q
}

func (q *mutationQueue) enqueue(fn func() error) error {
	q.mu.Lock()
	prevReady := q.ready
	q.ready = make(chan struct{})
	q.mu.Unlock()

	<-prevReady

	err := fn()

	close(q.ready)
	return err
}

type keyedQueue struct {
	mu   sync.Mutex
	queues map[string]*mutationQueue
}

func newKeyedQueue() *keyedQueue {
	return &keyedQueue{
		queues: make(map[string]*mutationQueue),
	}
}

func (k *keyedQueue) enqueue(key string, fn func() error) error {
	k.mu.Lock()
	q, ok := k.queues[key]
	if !ok {
		q = newMutationQueue()
		k.queues[key] = q
	}
	k.mu.Unlock()

	return q.enqueue(fn)
}