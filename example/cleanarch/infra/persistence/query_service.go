package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"example.com/cleanarch/domain/order"
)

// --- CQRS の Query 側実装 ---
// OrderQueryService の 2 つの実装で、
// errsweep の複数 concrete breakdown を検証する。

var (
	// ErrQueryTimeout はクエリタイムアウト。
	ErrQueryTimeout = errors.New("query timeout")
)

// SQLOrderQueryService は SQL ベースのクエリサービス。
type SQLOrderQueryService struct {
	db *sql.DB
}

func NewSQLOrderQueryService(db *sql.DB) *SQLOrderQueryService {
	return &SQLOrderQueryService{db: db}
}

// ListByUser はユーザーの注文一覧を返す。
//
// errsweep 検出:
//   - order.ErrOrderNotFound : ユーザーに注文がない（直接 return）
//   - sql.ErrNoRows          : DB に該当行なし（known map 経由）
func (s *SQLOrderQueryService) ListByUser(ctx context.Context, userID string) ([]order.Order, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, user_id, status FROM orders WHERE user_id = ?", userID)
	if err != nil {
		return nil, fmt.Errorf("SQLOrderQueryService.ListByUser: %w", err)
	}
	defer rows.Close()

	var orders []order.Order
	for rows.Next() {
		var o order.Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Status); err != nil {
			return nil, fmt.Errorf("SQLOrderQueryService.ListByUser: scan: %w", err)
		}
		orders = append(orders, o)
	}
	if len(orders) == 0 {
		return nil, order.ErrOrderNotFound
	}
	return orders, nil
}

// CountByStatus はステータスごとの注文数を返す。
//
// errsweep 検出:
//   - ErrQueryTimeout : コンテキスト期限切れ（直接 return）
func (s *SQLOrderQueryService) CountByStatus(ctx context.Context, status order.Status) (int, error) {
	if ctx.Err() != nil {
		return 0, ErrQueryTimeout
	}
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM orders WHERE status = ?", status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("SQLOrderQueryService.CountByStatus: %w", err)
	}
	return count, nil
}

// CachedOrderQueryService はキャッシュ付きクエリサービス。
// SQL 版と異なる sentinel を返すことで breakdown 検証に使う。
type CachedOrderQueryService struct {
	inner *SQLOrderQueryService
	cache map[string][]order.Order
}

// ErrCacheExpired はキャッシュ有効期限切れ。
var ErrCacheExpired = errors.New("cache expired")

func NewCachedOrderQueryService(inner *SQLOrderQueryService) *CachedOrderQueryService {
	return &CachedOrderQueryService{inner: inner, cache: make(map[string][]order.Order)}
}

// ListByUser はキャッシュ → SQL の順で取得する。
//
// errsweep 検出:
//   - ErrCacheExpired        : キャッシュ有効期限切れ（直接 return）
//   - order.ErrOrderNotFound : inner.ListByUser 経由（静的呼び出し）
//   - sql.ErrNoRows          : inner.ListByUser 経由
func (s *CachedOrderQueryService) ListByUser(ctx context.Context, userID string) ([]order.Order, error) {
	if cached, ok := s.cache[userID]; ok {
		if len(cached) == 0 {
			return nil, ErrCacheExpired
		}
		return cached, nil
	}
	orders, err := s.inner.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("CachedOrderQueryService.ListByUser: %w", err)
	}
	s.cache[userID] = orders
	return orders, nil
}

// CountByStatus は SQL 版に委譲する。
//
// errsweep 検出:
//   - ErrQueryTimeout : inner.CountByStatus 経由（静的呼び出し）
func (s *CachedOrderQueryService) CountByStatus(ctx context.Context, status order.Status) (int, error) {
	return s.inner.CountByStatus(ctx, status)
}
