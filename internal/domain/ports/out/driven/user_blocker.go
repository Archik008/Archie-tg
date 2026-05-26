package driven

import "context"

type UserBlocker interface {
	BlockUser(ctx context.Context, userID int) error
}
