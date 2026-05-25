package driven

type UserBlocker interface {
	BlockUser(userID int) error
}
