package gateway

import (
	"context"
	"errors"

	"example.com/cleanarch/domain/payment"
)

// PayPalGateway は PayPal を使った PaymentGateway の実装。
// StripeGateway とは異なる sentinel を返すため、
// errsweep の複数 concrete 解決（breakdown）の検証に使用する。
type PayPalGateway struct {
	clientID string
}

// ErrPayPalAccountSuspended は PayPal アカウント停止エラー。
// このパッケージ固有の sentinel。
var ErrPayPalAccountSuspended = errors.New("paypal account suspended")

// NewPayPalGateway はコンストラクタ。
func NewPayPalGateway(clientID string) *PayPalGateway {
	return &PayPalGateway{clientID: clientID}
}

// Charge は PayPal で決済を実行する。
//
// errsweep 検出:
//   - payment.ErrInvalidCard       : カード情報不正（直接 return）
//   - payment.ErrGatewayTimeout    : タイムアウト（直接 return）
//   - ErrPayPalAccountSuspended    : アカウント停止（直接 return）
//   - payment.InsufficientFundsError: 残高不足（カスタム値型 return）
func (g *PayPalGateway) Charge(ctx context.Context, amount int, card payment.Card) (payment.ChargeResult, error) {
	if card.Number == "" {
		return payment.ChargeResult{}, payment.ErrInvalidCard
	}
	if ctx.Err() != nil {
		return payment.ChargeResult{}, payment.ErrGatewayTimeout
	}
	if g.clientID == "" {
		return payment.ChargeResult{}, ErrPayPalAccountSuspended
	}
	// 値型カスタムエラーの例（ポインタレシーバでない）
	if amount > 500000 {
		return payment.ChargeResult{}, payment.InsufficientFundsError{
			Required:  amount,
			Available: 500000,
		}
	}
	return payment.ChargeResult{
		ChargeID: "pp_" + card.Number[:4],
		Status:   "completed",
	}, nil
}

// Refund は PayPal で返金を実行する。
//
// errsweep 検出:
//   - payment.ErrRefundFailed    : 返金失敗（直接 return）
//   - ErrPayPalAccountSuspended  : アカウント停止（直接 return）
func (g *PayPalGateway) Refund(ctx context.Context, chargeID string) (payment.RefundResult, error) {
	if chargeID == "" {
		return payment.RefundResult{}, payment.ErrRefundFailed
	}
	if g.clientID == "" {
		return payment.RefundResult{}, ErrPayPalAccountSuspended
	}
	return payment.RefundResult{
		RefundID: "pp_re_" + chargeID,
		Status:   "completed",
	}, nil
}
