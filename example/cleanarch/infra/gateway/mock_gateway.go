package gateway

import (
	"context"
	"fmt"
	"sync/atomic"

	"example.com/cleanarch/domain/payment"
)

// MockGateway はインメモリの PaymentGateway 実装。
// DB や外部 API なしに動作検証するために使う。
type MockGateway struct {
	seq atomic.Int64
}

func NewMockGateway() *MockGateway {
	return &MockGateway{}
}

// Charge はモック決済を実行する。
//
// errsweep 検出:
//   - payment.ErrPaymentDeclined : カード番号が空
//   - payment.ErrInvalidCard     : CVC が空
//   - *payment.PaymentError      : 高額決済
func (g *MockGateway) Charge(_ context.Context, amount int, card payment.Card) (payment.ChargeResult, error) {
	if card.Number == "" {
		return payment.ChargeResult{}, payment.ErrPaymentDeclined
	}
	if card.CVC == "" {
		return payment.ChargeResult{}, payment.ErrInvalidCard
	}
	if amount > 1000000 {
		return payment.ChargeResult{}, &payment.PaymentError{
			Provider: "mock",
			Code:     "amount_too_large",
			Message:  fmt.Sprintf("amount %d exceeds limit", amount),
		}
	}
	id := g.seq.Add(1)
	return payment.ChargeResult{
		ChargeID: fmt.Sprintf("mock_ch_%d", id),
		Status:   "succeeded",
	}, nil
}

// Refund はモック返金を実行する。
//
// errsweep 検出:
//   - payment.ErrRefundFailed : chargeID が空
func (g *MockGateway) Refund(_ context.Context, chargeID string) (payment.RefundResult, error) {
	if chargeID == "" {
		return payment.RefundResult{}, payment.ErrRefundFailed
	}
	id := g.seq.Add(1)
	return payment.RefundResult{
		RefundID: fmt.Sprintf("mock_re_%d", id),
		Status:   "succeeded",
	}, nil
}
