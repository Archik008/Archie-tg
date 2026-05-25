package chat

import "errors"

var (
	ErrChatExists   = errors.New("chat exists in the repo")
	ErrChatNotFound = errors.New("chat not found")
)
