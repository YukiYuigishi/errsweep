package gin

import "errors"

var (
	ErrUnauthorized = errors.New("gin: unauthorized")
	ErrBadRequest   = errors.New("gin: bad request")
)
