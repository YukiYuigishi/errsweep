package opaque

import "errors"

func NewError() error {
	return errors.New("something went wrong")
}

func AnotherError(msg string) error {
	return errors.New(msg)
}
