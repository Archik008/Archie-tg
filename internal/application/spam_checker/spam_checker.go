package spamchecker

import (
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

func (i *SpamCheckerService) ProcessUser(a dto.AccountDTO) error {
	if err := i.checkUserInWhiteList(a.UserID); err != nil {
		return err
	}

	if err := i.processNewChat(a); err != nil {
		return err
	}

	if err := i.adapters.userBlocker.BlockUser(a.UserID); err != nil {
		return err
	}

	return nil
}

func (i *SpamCheckerService) checkUserInWhiteList(userId int) error {
	_, err := i.repos.whiteList.Get(userId)
	if err != nil {
		return err
	}
	return nil
}

func (i *SpamCheckerService) processNewChat(a dto.AccountDTO) error {
	getChat, err := i.repos.chatRepo.Get(a.UserID)

	if errors.Is(err, chatPort.ErrChatNotFound) {
		userAccount := account.NewAccount(a.UserID, a.Username)
		userChat, err := chat.NewChat(i.baseAccount, userAccount)
		if err != nil {
			return err
		}
		return i.repos.chatRepo.Create(userChat)
	} else if err != nil {
		return err
	}

	if err := i.adapters.chatDeleter.DeleteChat(getChat.ID); err != nil {
		return err
	}
	if err := i.repos.chatRepo.Delete(getChat); err != nil {
		return err
	}

	return nil
}
