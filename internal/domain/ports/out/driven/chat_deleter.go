package driven

import "context"

type ChatDeleter interface {
	DeleteChat(ctx context.Context, chatId int) error
}
