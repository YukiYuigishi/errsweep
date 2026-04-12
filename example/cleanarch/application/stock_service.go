package application

import (
	"errors"
	"fmt"

	"example.com/cleanarch/domain/order"
)

// ======================================================================
// パターン E: 関数変数 DI（パッケージレベル var = Concrete）
// ======================================================================
// errsweep は init 内の Store 命令を解析して、
// パッケージレベルの関数変数から具象関数を解決する。

// StockCheckFunc は在庫確認関数の型。
type StockCheckFunc func(productID string, quantity int) error

// パッケージレベルの関数変数。errsweep はこの初期化を追跡する。
var checkStock StockCheckFunc = defaultStockCheck

// ErrStockUnavailable は在庫切れエラー。
var ErrStockUnavailable = errors.New("stock unavailable")

// defaultStockCheck はデフォルトの在庫確認実装。
func defaultStockCheck(productID string, quantity int) error {
	if productID == "" {
		return order.ErrInsufficientStock
	}
	if quantity > 100 {
		return ErrStockUnavailable
	}
	return nil
}

// CheckOrderStock は関数変数経由で在庫を確認する。
//
// ■ errsweep 検出（関数変数 DI: var checkStock = defaultStockCheck）:
//   - order.ErrInsufficientStock : defaultStockCheck 経由
//   - ErrStockUnavailable        : defaultStockCheck 経由
func CheckOrderStock(items []order.OrderItem) error {
	for _, item := range items {
		if err := checkStock(item.ProductID, item.Quantity); err != nil {
			return fmt.Errorf("CheckOrderStock: %w", err)
		}
	}
	return nil
}
