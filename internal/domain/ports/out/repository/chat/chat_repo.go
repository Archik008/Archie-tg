package chat

import "github.com/archik008/archie-tg/internal/domain/aggregate/chat"

type ChatRepositoryPort interface {
	Create(c chat.Chat) error
	Delete(c chat.Chat) error
	Get(chatId int) (chat.Chat, error)
}
