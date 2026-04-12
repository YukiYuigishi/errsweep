package application

import (
	"context"
	"fmt"

	"example.com/cleanarch/domain/order"
	"example.com/cleanarch/domain/payment"
)

// ======================================================================
// パターン B: compile-time assertion ありの複数 interface 連携
// ======================================================================

// CancelOrderUseCase は注文キャンセル＋返金ユースケース。
// 2 つの interface (OrderRepository + PaymentGateway) を連携して使う。
type CancelOrderUseCase struct {
	repo    order.OrderRepository
	gateway payment.PaymentGateway
}

func NewCancelOrderUseCase(repo order.OrderRepository, gw payment.PaymentGateway) *CancelOrderUseCase {
	return &CancelOrderUseCase{repo: repo, gateway: gw}
}

// Execute は注文をキャンセルし、決済済みなら返金する。
//
// ■ errsweep 検出:
//   - order.ErrOrderNotFound  : repo.FindByID 経由
//   - order.ErrOrderCancelled : 二重キャンセル防止（直接 return）
//   - sql.ErrNoRows           : repo.FindByID 経由
//   - payment.ErrRefundFailed : gateway.Refund 経由
//   - *payment.PaymentError   : gateway.Refund 経由（Stripe タイムアウト）
//   - gateway.ErrPayPalAccountSuspended : gateway.Refund 経由（PayPal）
//   - persistence.ErrCacheCorrupted     : repo 経由（CachedOrderRepository）
//   - *order.OrderValidationError       : repo.Save → ValidateOrder 経由
func (uc *CancelOrderUseCase) Execute(ctx context.Context, orderID string) error {
	o, err := uc.repo.FindByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("CancelOrderUseCase.Execute: find: %w", err)
	}

	if o.Status == order.StatusCancelled {
		return order.ErrOrderCancelled
	}

	// 決済済みなら返金
	if o.Status == order.StatusPaid && o.ChargeID != "" {
		if _, err := uc.gateway.Refund(ctx, o.ChargeID); err != nil {
			return fmt.Errorf("CancelOrderUseCase.Execute: refund: %w", err)
		}
	}

	o.Status = order.StatusCancelled
	if err := uc.repo.Save(ctx, o); err != nil {
		return fmt.Errorf("CancelOrderUseCase.Execute: save: %w", err)
	}
	return nil
}
