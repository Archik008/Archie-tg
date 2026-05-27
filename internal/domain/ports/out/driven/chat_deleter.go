package driven

import (
	"context"

	"github.com/archik008/archie-tg/internal/application/dto"
)

type ChatDeleter interface {
	DeleteChat(ctx context.Context, user dto.AccountDTO) error
}
