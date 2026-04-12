package catalogservice

import (
	"fmt"

	"example.com/myapp/catalogrepo"
)

// ProductService は ProductFinder を DI で受け取るユースケース層。
type ProductService struct {
	finder catalogrepo.ProductFinder
}

func NewProductService(finder catalogrepo.ProductFinder) *ProductService {
	return &ProductService{finder: finder}
}

// ResolveProductFromCatalog は DI された finder 経由で catalogrepo.FindProduct を呼び出す。
// エディタでこの呼び出し位置に hover して sentinel 表示を確認する。
func (s *ProductService) ResolveProductFromCatalog(id string) (string, error) {
	v, err := catalogrepo.FindProduct(s.finder, id)
	if err != nil {
		return "", fmt.Errorf("ResolveProductFromCatalog: %w", err)
	}
	return v, nil
}
