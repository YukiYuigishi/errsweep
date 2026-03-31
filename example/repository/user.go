package repository

import "errors"

var ErrNotFound = errors.New("user not found")
var ErrDuplicate = errors.New("user already exists")

func FindByID(id int) (string, error) {
	if id <= 0 {
		return "", ErrNotFound
	}
	return "alice", nil
}

func Create(name string) error {
	if name == "alice" {
		return ErrDuplicate
	}
	return nil
}
