package chat

import (
	"sync"

	"github.com/archik008/archie-tg/internal/domain/aggregate/chat"
	repo "github.com/archik008/archie-tg/internal/domain/ports/out/repository/chat"
)

type InMemoryChatRepository struct {
	chats map[int]chat.Chat
	mu    sync.RWMutex
}

func NewInMemoryChatRepository() repo.ChatRepositoryPort {
	return &InMemoryChatRepository{
		chats: make(map[int]chat.Chat),
	}
}

func (i *InMemoryChatRepository) Create(c chat.Chat) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, ok := i.chats[c.ID]; ok {
		return repo.ErrChatExists
	}

	i.chats[c.ID] = c

	return nil
}

func (i *InMemoryChatRepository) Get(chatId int) (chat.Chat, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	getChat, ok := i.chats[chatId]
	if !ok {
		return chat.Chat{}, repo.ErrChatNotFound
	}

	return getChat, nil
}

func (i *InMemoryChatRepository) Delete(c chat.Chat) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, ok := i.chats[c.ID]; !ok {
		return repo.ErrChatNotFound
	}

	delete(i.chats, c.ID)

	return nil
}
