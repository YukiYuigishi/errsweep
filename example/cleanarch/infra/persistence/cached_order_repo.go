package persistence

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"example.com/cleanarch/domain/order"
)

// --- デコレータパターン ---
// CachedOrderRepository は OrderRepository のデコレータ実装。
// 内部にキャッシュを持ち、別の OrderRepository に委譲する。
//
// errsweep 観点:
//   - CachedOrderRepository 自身の sentinel (ErrCacheCorrupted) は検出可能
//   - inner (OrderRepository interface) への委譲は、inner の具象が
//     compile-time assertion / auto-discovery で解決できれば検出可能
//   - ただし、2段の interface invoke (UseCase→CachedRepo→inner) になるため、
//     maxTraceDepth の影響を受けやすい

// ErrCacheCorrupted はキャッシュ整合性エラー。
var ErrCacheCorrupted = errors.New("cache corrupted")

// CachedOrderRepository は OrderRepository をラップするキャッシュ付きデコレータ。
type CachedOrderRepository struct {
	inner order.OrderRepository // 委譲先（interface → 別の concrete へ）
	mu    sync.RWMutex
	cache map[string]order.Order
}

// NewCachedOrderRepository はデコレータを構築する。
func NewCachedOrderRepository(inner order.OrderRepository) *CachedOrderRepository {
	return &CachedOrderRepository{
		inner: inner,
		cache: make(map[string]order.Order),
	}
}

// FindByID はキャッシュ → 委譲先の順で注文を探す。
//
// errsweep 検出:
//   - ErrCacheCorrupted     : キャッシュ不整合（直接 return）
//   - order.ErrOrderNotFound: inner.FindByID 経由（interface invoke → 具象解決が必要）
//   - sql.ErrNoRows         : inner.FindByID 経由（同上）
//
// 注意: inner が interface フィールドのため、errsweep が具象を解決できなければ
// inner 経由の sentinel は報告されない。
func (r *CachedOrderRepository) FindByID(ctx context.Context, id string) (order.Order, error) {
	r.mu.RLock()
	cached, ok := r.cache[id]
	r.mu.RUnlock()

	if ok {
		if cached.ID != id {
			return order.Order{}, ErrCacheCorrupted
		}
		return cached, nil
	}

	o, err := r.inner.FindByID(ctx, id)
	if err != nil {
		return order.Order{}, fmt.Errorf("CachedOrderRepository.FindByID: %w", err)
	}

	r.mu.Lock()
	r.cache[id] = o
	r.mu.Unlock()
	return o, nil
}

// Save は委譲先に保存し、キャッシュも更新する。
func (r *CachedOrderRepository) Save(ctx context.Context, o order.Order) error {
	if err := r.inner.Save(ctx, o); err != nil {
		return fmt.Errorf("CachedOrderRepository.Save: %w", err)
	}
	r.mu.Lock()
	r.cache[o.ID] = o
	r.mu.Unlock()
	return nil
}

// Delete は委譲先から削除し、キャッシュも破棄する。
func (r *CachedOrderRepository) Delete(ctx context.Context, id string) error {
	if err := r.inner.Delete(ctx, id); err != nil {
		return fmt.Errorf("CachedOrderRepository.Delete: %w", err)
	}
	r.mu.Lock()
	delete(r.cache, id)
	r.mu.Unlock()
	return nil
}
