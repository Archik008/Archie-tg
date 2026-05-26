package whitelist

import (
	"context"

	"github.com/archik008/archie-tg/internal/domain/entity/account"
)

type UserWhiteListRepositoryPort interface {
	Add(ctx context.Context, a account.Account) error
	Delete(ctx context.Context, a account.Account) error
	Get(ctx context.Context, userId int) (account.Account, error)
	GetAll(ctx context.Context) ([]account.Account, error)
}
