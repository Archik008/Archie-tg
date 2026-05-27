package tgclient

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/archik008/archie-tg/internal/application/dto"
	"github.com/archik008/archie-tg/internal/domain/ports/in"
	"github.com/gotd/td/tg"
)

type messageHandler struct {
	factory     func(api *tg.Client, selfUserID int64) in.UserSpamCheckerPort
	spamChecker in.UserSpamCheckerPort
	api         *atomic.Pointer[tg.Client]
	selfUserID  *atomic.Int64
	log         func(string)
}

func newMessageHandler(
	factory func(api *tg.Client, selfUserID int64) in.UserSpamCheckerPort,
	api *atomic.Pointer[tg.Client],
	selfUserID *atomic.Int64,
	log func(string),
) *messageHandler {
	if log == nil {
		log = func(string) {}
	}
	return &messageHandler{
		factory:    factory,
		api:        api,
		selfUserID: selfUserID,
		log:        log,
	}
}

func (h *messageHandler) checker() in.UserSpamCheckerPort {
	if h.spamChecker != nil {
		return h.spamChecker
	}
	if h.factory == nil || h.api == nil {
		return nil
	}
	api := h.api.Load()
	if api == nil {
		return nil
	}
	h.spamChecker = h.factory(api, h.selfID())
	return h.spamChecker
}

func (h *messageHandler) selfID() int64 {
	if h.selfUserID == nil {
		return 0
	}
	return h.selfUserID.Load()
}

func (h *messageHandler) register(dispatcher tg.UpdateDispatcher) {
	dispatcher.OnNewMessage(h.onNewMessage)
	dispatcher.OnEditMessage(h.onEditMessage)
	dispatcher.OnBotNewBusinessMessage(h.onBotNewBusinessMessage)
}

func (h *messageHandler) onNewMessage(ctx context.Context, entities tg.Entities, update *tg.UpdateNewMessage) error {
	return h.processMessage(ctx, entities, update.Message)
}

func (h *messageHandler) onEditMessage(ctx context.Context, entities tg.Entities, update *tg.UpdateEditMessage) error {
	return h.processMessage(ctx, entities, update.Message)
}

func (h *messageHandler) onBotNewBusinessMessage(ctx context.Context, entities tg.Entities, update *tg.UpdateBotNewBusinessMessage) error {
	return h.processMessage(ctx, entities, update.Message)
}

func (h *messageHandler) processMessage(ctx context.Context, entities tg.Entities, msg tg.MessageClass) error {
	acc, kind, ok := AccountFromMessage(msg, entities, h.selfID())
	if !ok {
		return nil
	}
	return h.dispatch(ctx, acc, kind)
}

func (h *messageHandler) dispatch(ctx context.Context, acc dto.AccountDTO, kind string) error {
	if acc.AccessHash == 0 || acc.Username == "" {
		if resolved, ok := h.resolveUserInfo(ctx, acc.UserID); ok {
			if acc.AccessHash == 0 {
				acc.AccessHash = resolved.AccessHash
			}
			if acc.Username == "" {
				acc.Username = resolved.Username
			}
		}
	}

	h.log(fmt.Sprintf(
		"Сообщение [%s] от user_id=%d username=%s access_hash=%d",
		kind,
		acc.UserID,
		displayUsername(acc.Username),
		acc.AccessHash,
	))

	checker := h.checker()
	if checker == nil {
		h.log("Спам-чекер ещё не инициализирован, сообщение пропущено")
		return nil
	}

	if err := checker.ProcessUser(ctx, acc); err != nil {
		h.log(fmt.Sprintf("Ошибка обработки user_id=%d: %v", acc.UserID, err))
		return err
	}

	h.log(fmt.Sprintf("Обработан user_id=%d", acc.UserID))
	return nil
}

func (h *messageHandler) resolveUserInfo(ctx context.Context, userID int) (*tg.User, bool) {
	if h.api == nil {
		return nil, false
	}
	api := h.api.Load()
	if api == nil {
		return nil, false
	}

	dialogs, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      100,
	})
	if err != nil {
		return nil, false
	}

	var users []tg.UserClass
	switch d := dialogs.(type) {
	case *tg.MessagesDialogs:
		users = d.Users
	case *tg.MessagesDialogsSlice:
		users = d.Users
	default:
		return nil, false
	}

	for _, u := range users {
		user, ok := u.(*tg.User)
		if !ok {
			continue
		}
		if user.ID == int64(userID) {
			return user, true
		}
	}

	return nil, false
}

func displayUsername(username string) string {
	if username == "" {
		return "-"
	}
	return "@" + username
}
