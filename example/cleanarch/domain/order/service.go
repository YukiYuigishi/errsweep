package order

// PricingStrategy は価格計算戦略のインターフェース。
// Strategy パターンにより、割引ロジックを差し替え可能にする。
//
// errsweep 観点:
//   このインターフェースは runtime に実装が決定されるパターンの例。
//   Factory 関数経由で返される場合、compile-time assertion がなければ
//   具象が解決できず sentinel 検出が不可能になりうる。
type PricingStrategy interface {
	// Calculate は注文に対する最終価格を返す。
	// エラーは価格計算が不可能な場合（割引条件不整合など）に返される。
	Calculate(order Order) (int, error)
}

// StockChecker は在庫確認サービスのインターフェース。
// 外部マイクロサービスへの問い合わせを想定。
type StockChecker interface {
	// CheckAvailability は商品の在庫を確認する。
	// 在庫不足の場合は ErrInsufficientStock を返す。
	CheckAvailability(productID string, quantity int) error
}
