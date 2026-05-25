package whitelist

import (
	"sync"

	"github.com/archik008/archie-tg/internal/domain/entity/account"
	"github.com/archik008/archie-tg/internal/domain/ports/out/repository/whitelist"
)

type InMemoryUserWhiteListRepository struct {
	users map[int]account.Account
	mu    sync.RWMutex
}

func NewInMemoryUserWhiteListRepository() whitelist.UserWhiteListRepositoryPort {
	return &InMemoryUserWhiteListRepository{
		users: make(map[int]account.Account),
	}
}

func (i *InMemoryUserWhiteListRepository) Add(a account.Account) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, ok := i.users[a.UserId]; ok {
		return whitelist.ErrAccountAlreadyExists
	}

	i.users[a.UserId] = a

	return nil
}

func (i *InMemoryUserWhiteListRepository) Get(userId int) (account.Account, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	acc, ok := i.users[userId]
	if !ok {
		return account.Account{}, whitelist.ErrAccountNotFound
	}

	return acc, nil
}

func (i *InMemoryUserWhiteListRepository) Delete(a account.Account) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, ok := i.users[a.UserId]; !ok {
		return whitelist.ErrAccountNotFound
	}

	delete(i.users, a.UserId)

	return nil
}
