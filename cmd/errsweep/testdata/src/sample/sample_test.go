package sample

import "errors"

var ErrTestOnly = errors.New("test only")

func helperFunc() error {
	return ErrTestOnly
}
