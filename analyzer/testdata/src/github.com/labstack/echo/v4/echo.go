package echo

import "errors"

var (
	ErrNotFound         = errors.New("echo: not found")
	ErrUnauthorized     = errors.New("echo: unauthorized")
	ErrMethodNotAllowed = errors.New("echo: method not allowed")
)
