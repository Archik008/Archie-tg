package tgclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/archik008/archie-tg/internal/domain/ports/in"
	setupclient "github.com/archik008/archie-tg/internal/setup/tg_client"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

// Runtime runs Telegram update loop and forwards user messages to spam checker.
type Runtime struct {
	client        *setupclient.Client
	buildChecker  func(api *tg.Client, selfUserID int64) in.UserSpamCheckerPort

	mu        sync.Mutex
	runCancel context.CancelFunc
}

func NewRuntime(
	client *setupclient.Client,
	buildChecker func(api *tg.Client, selfUserID int64) in.UserSpamCheckerPort,
) *Runtime {
	return &Runtime{
		client:       client,
		buildChecker: buildChecker,
	}
}

// Start launches update handling in background and writes human-readable logs to logs channel.
func (r *Runtime) Start(ctx context.Context, logs chan<- string) error {
	r.mu.Lock()
	if r.runCancel != nil {
		r.mu.Unlock()
		return errors.New("bot is already running")
	}

	runCtx, cancel := context.WithCancel(ctx)
	r.runCancel = cancel
	r.mu.Unlock()

	go func() {
		defer func() {
			r.mu.Lock()
			r.runCancel = nil
			r.mu.Unlock()
			close(logs)
		}()

		logf := func(format string, args ...any) {
			select {
			case <-runCtx.Done():
			case logs <- fmt.Sprintf(format, args...):
			}
		}

		logf("Запуск антиспам бота...")

		dispatcher := tg.NewUpdateDispatcher()
		var selfUserID atomic.Int64
		var apiPtr atomic.Pointer[tg.Client]

		handler := newMessageHandler(r.buildChecker, &apiPtr, &selfUserID, func(line string) {
			logf("%s", line)
		})
		handler.register(dispatcher)

		err := r.client.RunSession(runCtx, dispatcher, func(ctx context.Context, api *tg.Client) error {
			apiPtr.Store(api)

			id, err := r.client.EnsureSelfUserID(ctx, api)
			if err != nil {
				return err
			}
			selfUserID.Store(id)
			logf("Авторизован. self_user_id=%d", id)
			logf("Слушаю входящие сообщения (любой тип)...")

			<-ctx.Done()
			return ctx.Err()
		})

		if err != nil && !errors.Is(err, context.Canceled) {
			if tgErr, ok := tgerr.As(err); ok {
				logf("Ошибка Telegram: %s", tgErr.Error())
			} else {
				logf("Ошибка: %v", err)
			}
		}
		logf("Бот остановлен")
	}()

	return nil
}

// Stop cancels active bot session.
func (r *Runtime) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.runCancel != nil {
		r.runCancel()
	}
}

// IsRunning reports whether bot loop is active.
func (r *Runtime) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runCancel != nil
}
