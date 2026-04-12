// Package persistence は domain 層のリポジトリインターフェースの具象実装を提供する。
//
// errsweep 検出パターン:
//   - 直接 sentinel return（ErrOrderNotFound）
//   - sql.ErrNoRows（known map 経由）
//   - fmt.Errorf %w 経由の伝播
package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"example.com/cleanarch/domain/order"
)

// SQLOrderRepository は SQL データベースによる OrderRepository の実装。
type SQLOrderRepository struct {
	db *sql.DB
}

// NewSQLOrderRepository はコンストラクタ。
func NewSQLOrderRepository(db *sql.DB) *SQLOrderRepository {
	return &SQLOrderRepository{db: db}
}

// FindByID は注文を ID で取得する。
//
// errsweep 検出:
//   - order.ErrOrderNotFound : id が空（直接 return）
//   - sql.ErrNoRows          : DB に該当行なし（known map 経由）
func (r *SQLOrderRepository) FindByID(ctx context.Context, id string) (order.Order, error) {
	if id == "" {
		return order.Order{}, order.ErrOrderNotFound
	}
	var o order.Order
	err := r.db.QueryRowContext(ctx,
		"SELECT id, user_id, status, charge_id, created_at FROM orders WHERE id = ?", id,
	).Scan(&o.ID, &o.UserID, &o.Status, &o.ChargeID, &o.CreatedAt)
	if err != nil {
		return order.Order{}, fmt.Errorf("SQLOrderRepository.FindByID: %w", err)
	}
	return o, nil
}

// Save は注文を保存する。
//
// errsweep 検出:
//   - order.ErrOrderNotFound     : 更新対象が存在しない（直接 return）
//   - *order.OrderValidationError: バリデーション失敗（ValidateOrder 経由）
func (r *SQLOrderRepository) Save(ctx context.Context, o order.Order) error {
	if err := order.ValidateOrder(o); err != nil {
		return fmt.Errorf("SQLOrderRepository.Save: %w", err)
	}
	result, err := r.db.ExecContext(ctx,
		"UPDATE orders SET status = ?, charge_id = ? WHERE id = ?",
		o.Status, o.ChargeID, o.ID,
	)
	if err != nil {
		return fmt.Errorf("SQLOrderRepository.Save: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return order.ErrOrderNotFound
	}
	return nil
}

// Delete は注文を削除する。
//
// errsweep 検出:
//   - order.ErrOrderNotFound : 削除対象が存在しない（FindByID 経由）
//   - sql.ErrNoRows          : FindByID 経由
func (r *SQLOrderRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.FindByID(ctx, id); err != nil {
		return fmt.Errorf("SQLOrderRepository.Delete: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, "DELETE FROM orders WHERE id = ?", id); err != nil {
		return fmt.Errorf("SQLOrderRepository.Delete: exec: %w", err)
	}
	return nil
}

// --- 以下はこのパッケージ内のみで使う Sentinel ---

// ErrConnectionLost は DB 接続断を表す。
// エクスポート（Err プレフィックス）なので errsweep は検出する。
var ErrConnectionLost = errors.New("database connection lost")

// checkHealth は接続状態を確認する。
// このメソッドは OrderRepository インターフェースに含まれないため、
// interface 経由の呼び出しでは到達しない。
func (r *SQLOrderRepository) checkHealth(ctx context.Context) error {
	if err := r.db.PingContext(ctx); err != nil {
		return ErrConnectionLost
	}
	return nil
}
