package deferred

import (
	"context"
	"errors"
)

// 関数内に defer があると compiler は return 値を一度 local *ssa.Alloc に
// Store してから rundefers を経て reload する。sentinel 検出がこの Store
// → rundefers → Load の経路を追える必要がある。

type resource struct{}

func (r *resource) close() {}

var ErrInvalid = errors.New("invalid")

func WithDefer(ctx context.Context, arg int) error { // want `WithDefer returns sentinels: deferred\.ErrInvalid` WithDefer:`SentinelFact\(deferred\.ErrInvalid\)`
	r := &resource{}
	defer r.close()
	if arg <= 0 {
		return ErrInvalid
	}
	return nil
}

var ErrMissing = errors.New("missing")
var ErrDenied = errors.New("denied")

// 複数の defer があり、Phi 合流のような分岐でも union されること。
func MultiDefer(ctx context.Context, id int) error { // want `MultiDefer returns sentinels: deferred\.ErrDenied, deferred\.ErrMissing` MultiDefer:`SentinelFact\(deferred\.ErrDenied, deferred\.ErrMissing\)`
	r := &resource{}
	defer r.close()
	s := &resource{}
	defer s.close()
	if id < 0 {
		return ErrDenied
	}
	if id == 0 {
		return ErrMissing
	}
	return nil
}
