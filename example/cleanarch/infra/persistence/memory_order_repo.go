package persistence

import (
	"context"
	"fmt"
	"sync"

	"example.com/cleanarch/domain/order"
)

// MemoryOrderRepository はインメモリの OrderRepository 実装。
// エントリポイント（main.go）で DB なしに動作させるために使う。
type MemoryOrderRepository struct {
	mu     sync.RWMutex
	orders map[string]order.Order
}

func NewMemoryOrderRepository() *MemoryOrderRepository {
	return &MemoryOrderRepository{orders: make(map[string]order.Order)}
}

func (r *MemoryOrderRepository) FindByID(_ context.Context, id string) (order.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.orders[id]
	if !ok {
		return order.Order{}, order.ErrOrderNotFound
	}
	return o, nil
}

func (r *MemoryOrderRepository) Save(_ context.Context, o order.Order) error {
	if err := order.ValidateOrder(o); err != nil {
		return fmt.Errorf("MemoryOrderRepository.Save: %w", err)
	}
	r.mu.Lock()
	r.orders[o.ID] = o
	r.mu.Unlock()
	return nil
}

func (r *MemoryOrderRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.orders[id]; !ok {
		return order.ErrOrderNotFound
	}
	delete(r.orders, id)
	return nil
}

// Seed はデモ用の初期データを投入する。
func (r *MemoryOrderRepository) Seed(orders ...order.Order) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, o := range orders {
		r.orders[o.ID] = o
	}
}
