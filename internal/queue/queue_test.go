package queue

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquire_Release_allowsSingleHolder(t *testing.T) {
	t.Parallel()

	q := NewInMemoryMsgQueue(context.Background(), 0)

	if err := q.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	acquired := make(chan struct{})
	go func() {
		_ = q.Acquire(context.Background())
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatal("second Acquire should block until Release")
	case <-time.After(20 * time.Millisecond):
	}

	q.Release()

	select {
	case <-acquired:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("second Acquire should succeed after Release")
	}
	q.Release()
}

func TestAcquire_AllWaitersEventuallyRunOneAtATime(t *testing.T) {
	const workers = 4
	q := NewInMemoryMsgQueue(context.Background(), 0)

	if err := q.Acquire(context.Background()); err != nil {
		t.Fatalf("hold slot: %v", err)
	}

	start := make(chan struct{})
	acquiredBy := make([]int, 0, workers)
	var orderMu sync.Mutex
	var wg sync.WaitGroup

	for id := range workers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			<-start
			if err := q.Acquire(context.Background()); err != nil {
				t.Errorf("worker %d Acquire() error = %v", workerID, err)
				return
			}
			orderMu.Lock()
			acquiredBy = append(acquiredBy, workerID)
			orderMu.Unlock()
			q.Release()
		}(id)
	}

	close(start)
	q.Release()
	wg.Wait()

	orderMu.Lock()
	got := append([]int(nil), acquiredBy...)
	orderMu.Unlock()

	if len(got) != workers {
		t.Fatalf("acquired count = %d, want %d", len(got), workers)
	}
	seen := make(map[int]bool, len(got))
	for _, id := range got {
		if seen[id] {
			t.Fatalf("worker %d acquired slot twice", id)
		}
		seen[id] = true
	}
}

func TestAcquire_MultipleSpammersAllEventuallyProceed(t *testing.T) {
	const spammers = 6
	minInterval := 10 * time.Millisecond
	apiWork := 5 * time.Millisecond

	q := NewInMemoryMsgQueue(context.Background(), minInterval)

	start := make(chan struct{})
	var processed atomic.Int32
	errs := make([]error, spammers)
	var wg sync.WaitGroup

	for i := range spammers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			if err := q.Acquire(context.Background()); err != nil {
				errs[idx] = err
				return
			}
			time.Sleep(apiWork)
			processed.Add(1)
			q.Release()
		}(i)
	}

	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("spammer %d error = %v", i, err)
		}
	}
	if got := processed.Load(); got != spammers {
		t.Fatalf("processed = %d, want %d (every spammer must be handled)", got, spammers)
	}
}

func TestAcquire_respectsContextCancelWhileWaiting(t *testing.T) {
	q := NewInMemoryMsgQueue(context.Background(), 0)

	if err := q.Acquire(context.Background()); err != nil {
		t.Fatalf("hold slot: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	err := q.Acquire(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Acquire() error = %v, want context.DeadlineExceeded", err)
	}

	q.Release()
}

func TestAcquire_respectsAppShutdown(t *testing.T) {
	appCtx, cancel := context.WithCancel(context.Background())
	q := NewInMemoryMsgQueue(appCtx, 0)

	if err := q.Acquire(context.Background()); err != nil {
		t.Fatalf("hold slot: %v", err)
	}

	waiting := make(chan error, 1)
	go func() {
		waiting <- q.Acquire(context.Background())
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case err := <-waiting:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Acquire() error = %v, want context.Canceled", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("waiter should unblock on app shutdown")
	}

	q.Release()
}

func TestRelease_enforcesMinIntervalBetweenSlots(t *testing.T) {
	minInterval := 40 * time.Millisecond
	q := NewInMemoryMsgQueue(context.Background(), minInterval)

	if err := q.Acquire(context.Background()); err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}

	waiting := make(chan struct{})
	go func() {
		if err := q.Acquire(context.Background()); err != nil {
			t.Errorf("second Acquire() error = %v", err)
			return
		}
		close(waiting)
		q.Release()
	}()

	time.Sleep(5 * time.Millisecond)
	start := time.Now()
	q.Release()

	select {
	case <-waiting:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("second worker should acquire after first Release")
	}

	if elapsed := time.Since(start); elapsed < minInterval-5*time.Millisecond {
		t.Fatalf("waiter unblocked after %v, want at least ~%v", elapsed, minInterval)
	}
}
