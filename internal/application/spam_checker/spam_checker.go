package spamchecker

import (
	"context"
	"errors"

	"github.com/archik008/archie-tg/internal/application/dto"
	"github.com/archik008/archie-tg/internal/domain/aggregate/chat"
	"github.com/archik008/archie-tg/internal/domain/entity/account"
	"github.com/archik008/archie-tg/internal/domain/ports/in"
	"github.com/archik008/archie-tg/internal/domain/ports/out/driven"
	chatPort "github.com/archik008/archie-tg/internal/domain/ports/out/repository/chat"
	"github.com/archik008/archie-tg/internal/domain/ports/out/repository/whitelist"
)

type SpamCheckerService struct {
	repos struct {
		chatRepo  chatPort.ChatRepositoryPort
		whiteList whitelist.UserWhiteListRepositoryPort
	}
	adapters struct {
		chatDeleter driven.ChatDeleter
		userBlocker driven.UserBlocker
	}
	baseAccount account.Account
}

func NewSpamCheckerService(
	chatRepo chatPort.ChatRepositoryPort,
	whiteList whitelist.UserWhiteListRepositoryPort,
	chatDeleter driven.ChatDeleter,
	userBlocker driven.UserBlocker,
	baseAccount account.Account,
) in.UserSpamCheckerPort {
	return &SpamCheckerService{
		repos: struct {
			chatRepo  chatPort.ChatRepositoryPort
			whiteList whitelist.UserWhiteListRepositoryPort
		}{
			chatRepo:  chatRepo,
			whiteList: whiteList,
		},
		adapters: struct {
			chatDeleter driven.ChatDeleter
			userBlocker driven.UserBlocker
		}{
			chatDeleter: chatDeleter,
			userBlocker: userBlocker,
		},
		baseAccount: baseAccount,
	}
}

func (i *SpamCheckerService) ProcessUser(ctx context.Context, a dto.AccountDTO) error {
	if a.UserID == i.baseAccount.UserId {
		return nil
	}

	inWhiteList, err := i.checkUserInWhiteList(ctx, a.UserID)
	if err != nil {
		return err
	}
	if inWhiteList {
		return nil
	}

	if err := i.processNewChat(ctx, a); err != nil {
		return err
	}

	if err := i.adapters.userBlocker.BlockUser(ctx, a.UserID); err != nil {
		return err
	}

	return nil
}

func (i *SpamCheckerService) checkUserInWhiteList(ctx context.Context, userId int) (bool, error) {
	_, err := i.repos.whiteList.Get(ctx, userId)
	if errors.Is(err, whitelist.ErrAccountNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (i *SpamCheckerService) processNewChat(ctx context.Context, a dto.AccountDTO) error {
	getChat, err := i.repos.chatRepo.Get(ctx, a.UserID)

	if errors.Is(err, chatPort.ErrChatNotFound) {
		userAccount := account.NewAccount(a.UserID, a.Username)
		userChat, err := chat.NewChat(i.baseAccount, userAccount)
		if err != nil {
			return err
		}
		userChat = userChat.WithID(userAccount.UserId)
		return i.repos.chatRepo.Create(ctx, userChat)
	} else if err != nil {
		return err
	}

	if err := i.adapters.chatDeleter.DeleteChat(ctx, getChat.ID); err != nil {
		return err
	}
	if err := i.repos.chatRepo.Delete(ctx, getChat); err != nil {
		return err
	}

	return nil
}
