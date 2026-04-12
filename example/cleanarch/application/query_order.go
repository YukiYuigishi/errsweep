package application

import (
	"context"
	"fmt"

	"example.com/cleanarch/domain/order"
	"example.com/cleanarch/infra/persistence"
)

// ======================================================================
// パターン C: CQRS Query 側 — 複数 concrete の compile-time assertion
// ======================================================================

// compile-time assertion: OrderQueryService の 2 つの実装。
// SQL 版とキャッシュ版で異なる sentinel を返す。
var _ order.OrderQueryService = (*persistence.SQLOrderQueryService)(nil)
var _ order.OrderQueryService = (*persistence.CachedOrderQueryService)(nil)

// QueryOrderUseCase は注文照会ユースケース。
type QueryOrderUseCase struct {
	query order.OrderQueryService
}

func NewQueryOrderUseCase(query order.OrderQueryService) *QueryOrderUseCase {
	return &QueryOrderUseCase{query: query}
}

// ListUserOrders はユーザーの注文一覧を返す。
//
// ■ errsweep 検出（2 concrete の union + breakdown）:
//   - order.ErrOrderNotFound        : SQL 版（直接 return）
//   - sql.ErrNoRows                 : SQL 版（known map 経由）
//   - persistence.ErrCacheExpired   : キャッシュ版（直接 return）
//
// errsweep は concrete ごとの内訳も報告する:
//   via *persistence.SQLOrderQueryService:    order.ErrOrderNotFound, sql.ErrNoRows
//   via *persistence.CachedOrderQueryService: persistence.ErrCacheExpired, order.ErrOrderNotFound, sql.ErrNoRows
func (uc *QueryOrderUseCase) ListUserOrders(ctx context.Context, userID string) ([]order.Order, error) {
	orders, err := uc.query.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("QueryOrderUseCase.ListUserOrders: %w", err)
	}
	return orders, nil
}

// CountPendingOrders は保留中の注文数を返す。
//
// ■ errsweep 検出:
//   - persistence.ErrQueryTimeout : SQL 版 CountByStatus 経由
func (uc *QueryOrderUseCase) CountPendingOrders(ctx context.Context) (int, error) {
	count, err := uc.query.CountByStatus(ctx, order.StatusPending)
	if err != nil {
		return 0, fmt.Errorf("QueryOrderUseCase.CountPendingOrders: %w", err)
	}
	return count, nil
}
