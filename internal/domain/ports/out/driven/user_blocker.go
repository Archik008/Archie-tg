package driven

import (
	"context"

	"github.com/archik008/archie-tg/internal/application/dto"
)

type UserBlocker interface {
	BlockUser(ctx context.Context, user dto.AccountDTO) error
}
