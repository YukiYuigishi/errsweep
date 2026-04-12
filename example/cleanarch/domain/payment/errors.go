package payment

import (
	"errors"
	"fmt"
)

// --- エクスポートされた Sentinel 変数 ---

var (
	// ErrPaymentDeclined は決済が拒否された場合のエラー。
	ErrPaymentDeclined = errors.New("payment declined")

	// ErrInvalidCard はカード情報が不正な場合のエラー。
	ErrInvalidCard = errors.New("invalid card")

	// ErrRefundFailed は返金処理が失敗した場合のエラー。
	ErrRefundFailed = errors.New("refund failed")

	// ErrGatewayTimeout は決済ゲートウェイがタイムアウトした場合のエラー。
	ErrGatewayTimeout = errors.New("gateway timeout")
)

// --- カスタムエラー型 ---

// PaymentError は決済プロバイダ固有のエラー詳細を保持するカスタムエラー型。
// errsweep は *PaymentError を KindType として検出する。
//
// 使用例:
//
//	var pe *payment.PaymentError
//	if errors.As(err, &pe) {
//	    log.Printf("provider=%s code=%s", pe.Provider, pe.Code)
//	}
type PaymentError struct {
	Provider string // "stripe", "paypal" など
	Code     string // プロバイダ固有のエラーコード
	Message  string
}

func (e *PaymentError) Error() string {
	return fmt.Sprintf("payment error [%s/%s]: %s", e.Provider, e.Code, e.Message)
}

// InsufficientFundsError は残高不足エラー。値レシーバで error を実装する。
// errsweep は値型（非ポインタ）のカスタムエラーも検出する。
type InsufficientFundsError struct {
	Required  int
	Available int
}

func (e InsufficientFundsError) Error() string {
	return fmt.Sprintf("insufficient funds: required=%d available=%d", e.Required, e.Available)
}
