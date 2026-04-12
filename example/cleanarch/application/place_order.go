// Package application はユースケース（アプリケーションサービス）を提供する。
//
// Clean Architecture ではユースケース層が domain 層のインターフェースを
// コンストラクタ DI で受け取り、ビジネスロジックをオーケストレーションする。
//
// errsweep 観点:
//   このパッケージでは、検出可能なパターンと検出不可能なパターンを
//   明示的に対比する。各関数のコメントに検出結果の期待値を記載。
package application

import (
	"context"
	"fmt"

	"example.com/cleanarch/domain/order"
	"example.com/cleanarch/domain/payment"
	"example.com/cleanarch/infra/gateway"
	"example.com/cleanarch/infra/persistence"
)

// ======================================================================
// パターン A: compile-time assertion による interface 解決（検出可能）
// ======================================================================

// compile-time assertion: errsweep はこの宣言から
// OrderRepository → *SQLOrderRepository, *CachedOrderRepository の対応を抽出する。
var _ order.OrderRepository = (*persistence.SQLOrderRepository)(nil)
var _ order.OrderRepository = (*persistence.CachedOrderRepository)(nil)

// compile-time assertion: PaymentGateway → Stripe, PayPal の両方を宣言。
// errsweep は全具象の sentinel を union で報告し、concrete ごとの内訳も表示する。
var _ payment.PaymentGateway = (*gateway.StripeGateway)(nil)
var _ payment.PaymentGateway = (*gateway.PayPalGateway)(nil)

// PlaceOrderUseCase は注文確定ユースケース。
// コンストラクタ DI でインターフェースを受け取る標準的な Clean Architecture パターン。
type PlaceOrderUseCase struct {
	repo    order.OrderRepository
	gateway payment.PaymentGateway
}

// NewPlaceOrderUseCase はコンストラクタ。
func NewPlaceOrderUseCase(repo order.OrderRepository, gw payment.PaymentGateway) *PlaceOrderUseCase {
	return &PlaceOrderUseCase{repo: repo, gateway: gw}
}

// Execute は注文を確定する。
//
// ■ errsweep 検出（compile-time assertion 経由で解決）:
//   - order.ErrOrderNotFound        : repo.FindByID 経由
//   - order.ErrOrderAlreadyPaid     : ステータスチェック（直接 return）
//   - sql.ErrNoRows                 : repo.FindByID 経由
//   - payment.ErrPaymentDeclined    : gateway.Charge 経由（Stripe）
//   - payment.ErrInvalidCard        : gateway.Charge 経由（PayPal）
//   - payment.ErrGatewayTimeout     : gateway.Charge 経由（両方）
//   - *payment.PaymentError         : gateway.Charge 経由（Stripe）
//   - payment.InsufficientFundsError: gateway.Charge 経由（PayPal）
//   - gateway.ErrPayPalAccountSuspended : gateway.Charge 経由（PayPal）
//   - persistence.ErrCacheCorrupted : repo.FindByID 経由（CachedOrderRepository）
//   - persistence.ErrConnectionLost : repo.Save 経由（SQLOrderRepository.checkHealth は到達しないが Save 経由）
//   - *order.OrderValidationError   : repo.Save 経由（SQLOrderRepository.Save → ValidateOrder）
//
// ■ errsweep 非検出（compile-time assertion があっても原理的に追跡できないもの）:
//   - errValidation (非エクスポート変数)
//   - errors.New(...) (動的生成)
func (uc *PlaceOrderUseCase) Execute(ctx context.Context, orderID string, card payment.Card) error {
	o, err := uc.repo.FindByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("PlaceOrderUseCase.Execute: find: %w", err)
	}

	if o.Status == order.StatusPaid {
		return order.ErrOrderAlreadyPaid
	}

	result, err := uc.gateway.Charge(ctx, o.TotalAmount(), card)
	if err != nil {
		return fmt.Errorf("PlaceOrderUseCase.Execute: charge: %w", err)
	}

	o.Status = order.StatusPaid
	o.ChargeID = result.ChargeID
	if err := uc.repo.Save(ctx, o); err != nil {
		return fmt.Errorf("PlaceOrderUseCase.Execute: save: %w", err)
	}
	return nil
}
