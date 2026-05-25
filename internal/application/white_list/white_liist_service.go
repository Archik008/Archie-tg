package whitelist

import (
	"github.com/archik008/archie-tg/internal/application/dto"
	"github.com/archik008/archie-tg/internal/domain/entity/account"
	"github.com/archik008/archie-tg/internal/domain/ports/out/repository/whitelist"
)

type WhiteListService struct {
	whiteListRepo whitelist.UserWhiteListRepositoryPort
}

func (w *WhiteListService) AddToWhiteList(a dto.AccountDTO) error {
	newAccount := account.NewAccount(a.UserID, a.Username)
	return w.whiteListRepo.Add(newAccount)
}
