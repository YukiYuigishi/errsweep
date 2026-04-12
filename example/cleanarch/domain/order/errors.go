package order

import (
	"errors"
	"fmt"
)

// --- エクスポートされた Sentinel 変数 (errsweep 検出対象) ---

var (
	// ErrOrderNotFound は注文が存在しない場合のエラー。
	ErrOrderNotFound = errors.New("order not found")

	// ErrInsufficientStock は在庫不足エラー。
	ErrInsufficientStock = errors.New("insufficient stock")

	// ErrOrderAlreadyPaid は二重決済防止エラー。
	ErrOrderAlreadyPaid = errors.New("order already paid")

	// ErrOrderCancelled はキャンセル済み注文への操作エラー。
	ErrOrderCancelled = errors.New("order cancelled")
)

// --- 非エクスポート Sentinel 変数 (errsweep 検出対象外: "Err" プレフィックスなし) ---
// errsweep は var 名が "Err" で始まるもののみ Sentinel とみなす。
// 以下は内部利用のエラーだが、呼び出し元に伝播しても検出されない。

var (
	errValidation = errors.New("validation failed")
	errInternal   = errors.New("internal order error")
)

// --- カスタムエラー型 (errsweep 検出対象: エクスポートされた named type + error 実装) ---

// OrderValidationError はバリデーション失敗の詳細を持つカスタムエラー型。
// errsweep は MakeInterface 命令から型を検出し、KindType として報告する。
type OrderValidationError struct {
	Field   string
	Message string
}

func (e *OrderValidationError) Error() string {
	return fmt.Sprintf("order validation: %s: %s", e.Field, e.Message)
}

// --- 動的エラー生成 (errsweep 検出対象外) ---
// errors.New を関数内で直接呼ぶパターンは、パッケージレベル変数でないため
// Sentinel として認識されない。

// ValidateOrder は注文の妥当性を検証する。
//
// errsweep 検出:
//   - *OrderValidationError : カスタム型（検出可能）
//
// errsweep 非検出:
//   - errValidation         : 非エクスポート変数（検出不可能）
//   - errors.New(...)       : 動的生成（検出不可能）
func ValidateOrder(o Order) error {
	if o.UserID == "" {
		return &OrderValidationError{Field: "user_id", Message: "required"}
	}
	if len(o.Items) == 0 {
		// 非エクスポート Sentinel → 検出されない
		return errValidation
	}
	for _, item := range o.Items {
		if item.Quantity <= 0 {
			// 動的 errors.New → 検出されない
			return errors.New("quantity must be positive")
		}
	}
	return nil
}
