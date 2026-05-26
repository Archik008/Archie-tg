package queue

import "errors"

var (
	ErrQueueTimeout = errors.New("queue chan timeout")
)
