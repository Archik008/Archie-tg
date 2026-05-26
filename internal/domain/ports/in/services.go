package in

import (
	"context"

	"github.com/archik008/archie-tg/internal/application/dto"
)

type UserSpamCheckerPort interface {
	ProcessUser(ctx context.Context, a dto.AccountDTO) error
}

type UserWhiteListCheckerPort interface {
	AddToWhiteList(ctx context.Context, a dto.AccountDTO) error
	Get(ctx context.Context, userId int) (dto.AccountDTO, error)
	GetAll(ctx context.Context) ([]dto.AccountDTO, error)
	Delete(ctx context.Context, a dto.AccountDTO) error
}
