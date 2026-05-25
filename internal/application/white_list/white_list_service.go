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

func (w *WhiteListService) GetAll() ([]dto.AccountDTO, error) {
	accounts, err := w.whiteListRepo.GetAll()
	if err != nil {
		return nil, err
	}
	dtoAccs := mapToAccDTO(accounts)
	return dtoAccs, nil
}

func (w *WhiteListService) Get(userId int) (dto.AccountDTO, error) {
	acc, err := w.whiteListRepo.Get(userId)
	if err != nil {
		return dto.AccountDTO{}, err
	}

	return mapToSingleAccDTO(acc), nil
}

func (w *WhiteListService) Delete(a dto.AccountDTO) error {
	newAccount := account.NewAccount(a.UserID, a.Username)
	return w.whiteListRepo.Delete(newAccount)
}
