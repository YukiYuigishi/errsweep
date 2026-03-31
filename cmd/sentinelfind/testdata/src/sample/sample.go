package sample

import "errors"

var ErrNotFound = errors.New("not found")

func Find(id int) error {
	if id <= 0 {
		return ErrNotFound
	}
	return nil
}
