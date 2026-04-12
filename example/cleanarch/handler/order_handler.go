package handler

import (
	"context"
	"errors"
	"fmt"

	"example.com/cleanarch/application"
	"example.com/cleanarch/di"
	"example.com/cleanarch/domain/order"
	"example.com/cleanarch/domain/payment"
)

// ======================================================================
// パターン G: Handler 層 — 静的 DI コンテナ経由（検出可能）
// ======================================================================

// ErrBadRequest はリクエスト不正エラー。
var ErrBadRequest = errors.New("bad request")

// OrderHandler は HTTP ハンドラ。
// DI コンテナから具象ユースケースを受け取る。
type OrderHandler struct {
	container *di.Container
}

func NewOrderHandler(c *di.Container) *OrderHandler {
	return &OrderHandler{container: c}
}

// HandlePlaceOrder は注文確定リクエストを処理する。
//
// ■ errsweep 検出:
//   container.PlaceOrder は *application.PlaceOrderUseCase（具象型）なので、
//   Execute の呼び出しは静的に解決され、PlaceOrderUseCase.Execute の
//   sentinel が伝播する。
//
//   - ErrBadRequest                  : バリデーション（直接 return）
//   - order.ErrOrderNotFound         : PlaceOrder.Execute 経由
//   - order.ErrOrderAlreadyPaid      : PlaceOrder.Execute 経由
//   - sql.ErrNoRows                  : PlaceOrder.Execute 経由
//   - payment.ErrPaymentDeclined     : PlaceOrder.Execute 経由（Stripe）
//   - payment.ErrInvalidCard         : PlaceOrder.Execute 経由（PayPal）
//   - payment.ErrGatewayTimeout      : PlaceOrder.Execute 経由
//   - *payment.PaymentError          : PlaceOrder.Execute 経由
//   - ... 他、PlaceOrderUseCase が伝播する全 sentinel
func (h *OrderHandler) HandlePlaceOrder(ctx context.Context, orderID string, card payment.Card) error {
	if orderID == "" {
		return ErrBadRequest
	}
	if err := h.container.PlaceOrder.Execute(ctx, orderID, card); err != nil {
		return fmt.Errorf("HandlePlaceOrder: %w", err)
	}
	return nil
}

// HandleCancelOrder は注文キャンセルリクエストを処理する。
//
// ■ errsweep 検出:
//   - ErrBadRequest           : バリデーション（直接 return）
//   - order.ErrOrderNotFound  : CancelOrder.Execute 経由
//   - order.ErrOrderCancelled : CancelOrder.Execute 経由
//   - payment.ErrRefundFailed : CancelOrder.Execute 経由
//   - ... 他、CancelOrderUseCase が伝播する全 sentinel
func (h *OrderHandler) HandleCancelOrder(ctx context.Context, orderID string) error {
	if orderID == "" {
		return ErrBadRequest
	}
	if err := h.container.CancelOrder.Execute(ctx, orderID); err != nil {
		return fmt.Errorf("HandleCancelOrder: %w", err)
	}
	return nil
}

// ======================================================================
// パターン H: Registry 経由の動的解決（検出不可能）
// ======================================================================

// DynamicHandler はレジストリ経由でユースケースを取得するハンドラ。
// 型アサーションで具象を取り出すが、errsweep は追跡できない。
type DynamicHandler struct {
	registry *di.Registry
}

func NewDynamicHandler(r *di.Registry) *DynamicHandler {
	return &DynamicHandler{registry: r}
}

// HandlePlaceOrderDynamic はレジストリ経由で PlaceOrderUseCase を取得して実行する。
//
// ■ errsweep 検出:
//   - di.ErrServiceNotFound : Registry.Resolve 経由（Fact 伝播）
//   - di.ErrServiceTypeMismatch : 型不一致（直接 return）
//   - ErrBadRequest         : バリデーション（直接 return）
//   - PlaceOrderUseCase.Execute の全 sentinel（下記参照）
//
// ■ 重要な知見:
//   SSA の TypeAssert で具象型（*PlaceOrderUseCase）にアサーションした後の
//   メソッド呼び出しは **静的呼び出し** になるため、errsweep は追跡できる。
//   つまりレジストリ + 具象型アサーションの組み合わせは検出可能。
//   検出不可能になるのは interface 型へのアサーション（後述の HandleViaInterface）。
func (h *DynamicHandler) HandlePlaceOrderDynamic(ctx context.Context, orderID string, card payment.Card) error {
	if orderID == "" {
		return ErrBadRequest
	}
	svc, err := h.registry.Resolve("placeOrder")
	if err != nil {
		return fmt.Errorf("HandlePlaceOrderDynamic: %w", err)
	}
	uc, ok := svc.(*application.PlaceOrderUseCase)
	if !ok {
		return fmt.Errorf("HandlePlaceOrderDynamic: %w", di.ErrServiceTypeMismatch)
	}
	if err := uc.Execute(ctx, orderID, card); err != nil {
		return fmt.Errorf("HandlePlaceOrderDynamic: %w", err)
	}
	return nil
}

// ======================================================================
// パターン I: interface フィールド DI（assertion 有無で結果が変わる）
// ======================================================================

// GenericHandler は interface 経由でユースケースを呼び出すハンドラ。
// compile-time assertion がなければ errsweep は具象を解決できない。
type GenericHandler struct {
	repo order.OrderRepository // interface フィールド
}

func NewGenericHandler(repo order.OrderRepository) *GenericHandler {
	return &GenericHandler{repo: repo}
}

// HandleGetOrder は interface フィールド経由で注文を取得する。
//
// ■ errsweep 検出（handler パッケージが application をインポートしているため、
//   application/place_order.go の compile-time assertion が有効）:
//   - order.ErrOrderNotFound    : SQLOrderRepository.FindByID 経由
//   - sql.ErrNoRows             : SQLOrderRepository.FindByID 経由
//   - persistence.ErrCacheCorrupted : CachedOrderRepository.FindByID 経由
//
// ■ 条件:
//   handler パッケージが application をインポートしていなければ assertion が見えず、
//   auto-discovery + RTA のみに依存する。具象型が直接インポートのスコープに
//   なければ検出されない可能性がある。
func (h *GenericHandler) HandleGetOrder(ctx context.Context, id string) (order.Order, error) {
	o, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return order.Order{}, fmt.Errorf("HandleGetOrder: %w", err)
	}
	return o, nil
}

// ======================================================================
// パターン K: interface 型へのアサーション（検出不可能）
// ======================================================================

// HandleViaInterface はレジストリから取り出した値を interface にアサーションする。
// concrete 型ではなく interface 型にアサーションするため、
// SSA 上では TypeAssert → Invoke となり、errsweep は具象を解決できない。
//
// HandlePlaceOrderDynamic（concrete 型アサーション）との対比:
//   - concrete 型アサーション: TypeAssert → Static Call → 検出可能
//   - interface 型アサーション: TypeAssert → Invoke → 検出不可能（assertion がこのパッケージにない限り）
//
// ■ errsweep 検出:
//   - di.ErrServiceNotFound : Registry.Resolve 経由
//   - ErrBadRequest         : バリデーション（直接 return）
//
// ■ errsweep 非検出:
//   repo.FindByID は interface invoke となり、handler パッケージ単体では
//   OrderRepository の具象が不明。ただし application パッケージの assertion が
//   インポート経由で見えるため、実際には解決される可能性がある。
//   これは「解析対象パッケージがどのパッケージをインポートしているか」に依存する。
func (h *DynamicHandler) HandleViaInterface(ctx context.Context, id string) (order.Order, error) {
	if id == "" {
		return order.Order{}, ErrBadRequest
	}
	svc, err := h.registry.Resolve("orderRepo")
	if err != nil {
		return order.Order{}, fmt.Errorf("HandleViaInterface: %w", err)
	}
	repo, ok := svc.(order.OrderRepository)
	if !ok {
		return order.Order{}, fmt.Errorf("HandleViaInterface: %w", di.ErrServiceTypeMismatch)
	}
	o, err := repo.FindByID(ctx, id)
	if err != nil {
		return order.Order{}, fmt.Errorf("HandleViaInterface: %w", err)
	}
	return o, nil
}
