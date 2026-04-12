package catalogrepo

import (
	"errors"
	"fmt"
)

var (
	ErrProductNotFound = errors.New("product not found")
	ErrProductArchived = errors.New("product archived")
)

// ProductFinder は商品検索の実処理を提供するインターフェース。
type ProductFinder interface {
	FindProduct(id string) (string, error)
}

type primaryProductFinder struct{}
type archiveProductFinder struct{}

var _ ProductFinder = primaryProductFinder{}
var _ ProductFinder = archiveProductFinder{}

func (primaryProductFinder) FindProduct(id string) (string, error) {
	if id == "" {
		return "", ErrProductNotFound
	}
	return "primary:" + id, nil
}

func (archiveProductFinder) FindProduct(id string) (string, error) {
	if id == "" {
		return "", ErrProductArchived
	}
	return "archive:" + id, nil
}

func NewProductFinder(kind string) ProductFinder {
	if kind == "primary" {
		return primaryProductFinder{}
	}
	return archiveProductFinder{}
}

// FindProduct は catalogservice から呼ばれる公開関数。
// 内部で interface invoke を行うため、呼び出し側 hover で sentinel を確認するデモになる。
func FindProduct(f ProductFinder, id string) (string, error) {
	v, err := f.FindProduct(id)
	if err != nil {
		return "", fmt.Errorf("FindProduct: %w", err)
	}
	return v, nil
}
