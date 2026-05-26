package chat

import (
	"context"

	"github.com/archik008/archie-tg/internal/domain/aggregate/chat"
)

type ChatRepositoryPort interface {
	Create(ctx context.Context, c chat.Chat) error
	Delete(ctx context.Context, c chat.Chat) error
	Get(ctx context.Context, chatId int) (chat.Chat, error)
}
