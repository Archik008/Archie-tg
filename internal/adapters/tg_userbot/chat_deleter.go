package tguserbot

import (
	"context"

	"github.com/archik008/archie-tg/config"
	"github.com/archik008/archie-tg/internal/application/dto"
	"github.com/archik008/archie-tg/internal/queue"
	"github.com/gotd/td/tg"
)

type TgUserBotСhatDeleter struct {
	client   *tg.Client
	tgCfg    config.TelegramUserCfg
	msgQueue *queue.InMemoryMsgQueue
}

func NewTgUserBotChatDeleter(client *tg.Client, tgCfg config.TelegramUserCfg,
	msgQueue *queue.InMemoryMsgQueue) *TgUserBotСhatDeleter {
	return &TgUserBotСhatDeleter{
		client:   client,
		tgCfg:    tgCfg,
		msgQueue: msgQueue,
	}
}

func (t *TgUserBotСhatDeleter) DeleteChat(ctx context.Context, user dto.AccountDTO) error {
	usrPeer, err := inputPeerUser(user, t.tgCfg)
	if err != nil {
		return err
	}

	if err := t.msgQueue.Acquire(ctx); err != nil {
		return err
	}
	defer t.msgQueue.Release()

	_, err = t.client.MessagesDeleteHistory(ctx, &tg.MessagesDeleteHistoryRequest{
		Revoke: true,
		Peer:   usrPeer,
	})

	return err
}
