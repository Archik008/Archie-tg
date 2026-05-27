package queue

import (
	"context"
	"time"
)

// InMemoryMsgQueue serializes Telegram API calls and enforces a minimum gap
// between them. Many spam-handler goroutines may call Acquire concurrently;
// they wait in FIFO order until the previous caller finishes and calls Release.
//
// Typical usage:
//
//	if err := q.Acquire(ctx); err != nil { return err }
//	defer q.Release()
//	// call Telegram API here
type InMemoryMsgQueue struct {
	tokens      chan struct{}
	minInterval time.Duration
	appCtx      context.Context
}

// NewInMemoryMsgQueue creates a queue. minInterval is the minimum time between
// the end of one API call (Release) and the start of the next (Acquire returns).
// appCtx shuts down all waiters when cancelled.
func NewInMemoryMsgQueue(appCtx context.Context, minInterval time.Duration) *InMemoryMsgQueue {
	q := &InMemoryMsgQueue{
		tokens:      make(chan struct{}, 1),
		minInterval: minInterval,
		appCtx:      appCtx,
	}
	// One token = at most one in-flight Telegram operation.
	q.tokens <- struct{}{}
	return q
}

// Acquire blocks until this worker may call the Telegram API, or until ctx /
// appCtx is cancelled. Later callers wait longer — everyone is processed in order.
func (q *InMemoryMsgQueue) Acquire(ctx context.Context) error {
	if err := q.appCtx.Err(); err != nil {
		return err
	}

	select {
	case <-q.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-q.appCtx.Done():
		return q.appCtx.Err()
	}
}

// Release must be called after the API work finishes (defer is fine).
// The next waiter receives the slot only after minInterval has elapsed.
func (q *InMemoryMsgQueue) Release() {
	if q.minInterval > 0 {
		time.Sleep(q.minInterval)
	}

	select {
	case q.tokens <- struct{}{}:
	default:
		panic("queue: Release called without a matching Acquire")
	}
}
