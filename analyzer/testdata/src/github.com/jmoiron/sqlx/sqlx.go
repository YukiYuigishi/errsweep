package sqlx

import "errors"

var ErrNotFound = errors.New("sqlx: not found")

func Get(_ any, _ string, _ ...any) error {
	return ErrNotFound
}
