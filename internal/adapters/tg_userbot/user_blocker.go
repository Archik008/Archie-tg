package tguserbot

import (
	"context"

	"github.com/archik008/archie-tg/config"
	"github.com/archik008/archie-tg/internal/application/dto"
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

func (t *TgUserBlocker) BlockUser(ctx context.Context, user dto.AccountDTO) error {
	usrPeer, err := inputPeerUser(user, t.tgCfg)
	if err != nil {
		return err
	}

	if err := t.msgQueue.Acquire(ctx); err != nil {
		return err
	}
	defer t.msgQueue.Release()

	if _, err = t.client.ContactsBlock(ctx, &tg.ContactsBlockRequest{
		ID: usrPeer,
	}); err != nil {
		return err
	}

	return nil
}
