package in

import "github.com/archik008/archie-tg/internal/application/dto"

type UserSpamCheckerPort interface {
	ProcessUser(a dto.AccountDTO) error
}

type UserWhiteListCheckerPort interface {
	AddToWhiteList(a dto.AccountDTO) error
}
