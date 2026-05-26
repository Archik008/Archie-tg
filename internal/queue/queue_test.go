package queue

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestProcess_EnqueuesWhenQueueHasCapacity(t *testing.T) {
	t.Parallel()

	q := NewInMemoryMsgQueue(context.Background(), 10*time.Millisecond)

	if err := q.Process(); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if got := len(q.queueChan); got != 1 {
		t.Fatalf("queue size = %d, want 1", got)
	}
}

func TestProcess_ReturnsTimeoutWhenQueueIsFull(t *testing.T) {
	q := NewInMemoryMsgQueue(context.Background(), time.Millisecond)

	if err := q.Process(); err != nil {
		t.Fatalf("Process() first call error = %v", err)
	}

	err := q.Process()
	if !errors.Is(err, ErrQueueTimeout) {
		t.Fatalf("Process() second call error = %v, want %v", err, ErrQueueTimeout)
	}

	// Unblock the sender goroutine started by the timed-out Process call.
	<-q.queueChan
	time.Sleep(10 * time.Millisecond)
	select {
	case <-q.queueChan:
	default:
	}
}

func TestListenQueue_DrainsQueueAndAllowsNextProcess(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	q := NewInMemoryMsgQueue(ctx, 10*time.Millisecond)
	q.ListenQueue()

	if err := q.Process(); err != nil {
		t.Fatalf("Process() first call error = %v", err)
	}

	start := time.Now()
	if err := q.Process(); err != nil {
		t.Fatalf("Process() second call error = %v", err)
	}

	if elapsed := time.Since(start); elapsed >= 500*time.Millisecond {
		t.Fatalf("Process() waited too long after listener started: %v", elapsed)
	}
}
