package queue

import (
	"context"
	"time"
)

type InMemoryMsgQueue struct {
	queueChan chan struct{}
	timeout   time.Duration
	appCtx    context.Context
}

func NewInMemoryMsgQueue(appCtx context.Context, timeout time.Duration) *InMemoryMsgQueue {
	return &InMemoryMsgQueue{
		queueChan: make(chan struct{}, 1),
		timeout:   timeout,
		appCtx:    appCtx,
	}
}

func (i *InMemoryMsgQueue) ListenQueue() {
	go func() {
		ticker := time.NewTicker(i.timeout)
		for {
			select {
			case <-ticker.C:
			case <-i.appCtx.Done():
				return
			}

			select {
			case <-i.queueChan:
			case <-i.appCtx.Done():
				return
			default:
			}
		}
	}()
}

func (i *InMemoryMsgQueue) Process() error {
	doneCh := make(chan struct{}, 1)
	go func() {
		i.queueChan <- struct{}{}
		doneCh <- struct{}{}
	}()

	select {
	case <-doneCh:
		return nil
	case <-time.After(i.timeout + (time.Millisecond * 500)):
		return ErrQueueTimeout
	}
}

// func (i *InMemoryMsgQueue) ProcessQueue() error {

// }
