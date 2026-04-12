package gateway

import (
	"context"
	"fmt"

	"example.com/cleanarch/domain/payment"
)

// StripeGateway は Stripe を使った PaymentGateway の実装。
type StripeGateway struct {
	apiKey string
}

// NewStripeGateway はコンストラクタ。
func NewStripeGateway(apiKey string) *StripeGateway {
	return &StripeGateway{apiKey: apiKey}
}

// Charge は Stripe で決済を実行する。
//
// errsweep 検出:
//   - payment.ErrPaymentDeclined : カード情報不正（直接 return）
//   - payment.ErrGatewayTimeout  : タイムアウト（直接 return）
//   - *payment.PaymentError      : Stripe 固有エラー（カスタム型 return）
func (g *StripeGateway) Charge(ctx context.Context, amount int, card payment.Card) (payment.ChargeResult, error) {
	if card.Number == "" {
		return payment.ChargeResult{}, payment.ErrPaymentDeclined
	}

	if ctx.Err() != nil {
		return payment.ChargeResult{}, payment.ErrGatewayTimeout
	}

	// Stripe API 固有のエラーをカスタム型で返すパターン
	if amount > 1000000 {
		return payment.ChargeResult{}, &payment.PaymentError{
			Provider: "stripe",
			Code:     "amount_too_large",
			Message:  fmt.Sprintf("amount %d exceeds limit", amount),
		}
	}

	return payment.ChargeResult{
		ChargeID: "ch_stripe_" + card.Number[:4],
		Status:   "succeeded",
	}, nil
}

// Refund は Stripe で返金を実行する。
//
// errsweep 検出:
//   - payment.ErrRefundFailed : 返金失敗（直接 return）
//   - *payment.PaymentError   : Stripe 固有エラー（カスタム型 return）
func (g *StripeGateway) Refund(ctx context.Context, chargeID string) (payment.RefundResult, error) {
	if chargeID == "" {
		return payment.RefundResult{}, payment.ErrRefundFailed
	}

	if ctx.Err() != nil {
		return payment.RefundResult{}, &payment.PaymentError{
			Provider: "stripe",
			Code:     "timeout",
			Message:  "refund timed out",
		}
	}

	return payment.RefundResult{
		RefundID: "re_stripe_" + chargeID,
		Status:   "succeeded",
	}, nil
}
