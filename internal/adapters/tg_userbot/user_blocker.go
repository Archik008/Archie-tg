package tguserbot

import (
	"context"

	"github.com/archik008/archie-tg/config"
	"github.com/archik008/archie-tg/internal/queue"
	"github.com/gotd/td/tg"
)

type TgUserBlocker struct {
	client   *tg.Client
	tgCfg    config.TelegramUserCfg
	msgQueue *queue.InMemoryMsgQueue
}

func NewTgUserBlocker(client *tg.Client, tgCfg config.TelegramUserCfg,
	msgQueue *queue.InMemoryMsgQueue) *TgUserBlocker {
	return &TgUserBlocker{
		client:   client,
		tgCfg:    tgCfg,
		msgQueue: msgQueue,
	}
}

func (t *TgUserBlocker) BlockUser(ctx context.Context, userID int) error {
	usrPeer := &tg.InputPeerUser{
		UserID:     int64(userID),
		AccessHash: t.tgCfg.USER_ACCESS_HASH,
	}

	if err := t.msgQueue.Acquire(ctx); err != nil {
		return err
	}
	defer t.msgQueue.Release()

	if _, err := t.client.ContactsBlock(ctx, &tg.ContactsBlockRequest{
		ID:            usrPeer,
		MyStoriesFrom: true,
	}); err != nil {
		return err
	}

	return nil
}
