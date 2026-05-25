package whitelist

import (
	"github.com/archik008/archie-tg/internal/application/dto"
	"github.com/archik008/archie-tg/internal/domain/entity/account"
)

func mapToAccDTO(accs []account.Account) []dto.AccountDTO {
	accList := make([]dto.AccountDTO, len(accs))
	for i, acc := range accs {
		accList[i] = mapToSingleAccDTO(acc)
	}
	return accList
}

func mapToSingleAccDTO(acc account.Account) dto.AccountDTO {
	return dto.AccountDTO{
		UserID:   acc.UserId,
		Username: acc.Username,
	}
}
