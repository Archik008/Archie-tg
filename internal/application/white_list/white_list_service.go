package whitelist

import (
	"context"

	"github.com/archik008/archie-tg/internal/application/dto"
	"github.com/archik008/archie-tg/internal/domain/entity/account"
	"github.com/archik008/archie-tg/internal/domain/ports/out/repository/whitelist"
)

type WhiteListService struct {
	whiteListRepo whitelist.UserWhiteListRepositoryPort
}

func (w *WhiteListService) AddToWhiteList(ctx context.Context, a dto.AccountDTO) error {
	newAccount := account.NewAccount(a.UserID, a.Username)
	return w.whiteListRepo.Add(ctx, newAccount)
}

func (w *WhiteListService) GetAll(ctx context.Context) ([]dto.AccountDTO, error) {
	accounts, err := w.whiteListRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	dtoAccs := mapToAccDTO(accounts)
	return dtoAccs, nil
}

func (w *WhiteListService) Get(ctx context.Context, userId int) (dto.AccountDTO, error) {
	acc, err := w.whiteListRepo.Get(ctx, userId)
	if err != nil {
		return dto.AccountDTO{}, err
	}

	return mapToSingleAccDTO(acc), nil
}

func (w *WhiteListService) Delete(ctx context.Context, a dto.AccountDTO) error {
	newAccount := account.NewAccount(a.UserID, a.Username)
	return w.whiteListRepo.Delete(ctx, newAccount)
}
