package whitelist

import (
	"github.com/archik008/archie-tg/internal/domain/entity/account"
)

type UserWhiteListRepositoryPort interface {
	Get(userId int) (account.Account, error)
	Add(a account.Account) error
	Delete(a account.Account) error
}
