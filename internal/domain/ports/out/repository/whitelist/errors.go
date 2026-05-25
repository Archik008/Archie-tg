package whitelist

import "errors"

var (
	ErrAccountNotFound      = errors.New("account not found in repository")
	ErrAccountAlreadyExists = errors.New("account already exists in repository")
)
