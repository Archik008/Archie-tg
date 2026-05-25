package chat

import (
	"github.com/archik008/archie-tg/internal/domain/entity/account"
)

type Chat struct {
	ID       int
	Accounts []account.Account
}

func NewChat(accounts ...account.Account) (Chat, error) {
	if len(accounts) == 0 {
		return Chat{}, ErrChatMembersEmpty
	}
	return Chat{
		Accounts: accounts,
	}, nil
}

func (c Chat) WithID(id int) Chat {
	c.ID = id
	return c
}
