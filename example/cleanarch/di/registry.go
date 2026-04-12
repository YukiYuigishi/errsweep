package di

import (
	"errors"
	"fmt"
)

// ======================================================================
// パターン F: リフレクション風の動的レジストリ（検出不可能）
// ======================================================================
// 実際のプロダクションでは uber/dig や google/wire を使うことが多いが、
// ここではその本質（型キーによる動的解決）を簡略化して示す。
//
// errsweep 観点:
//   map[string]any に格納された依存は SSA 上で型が不明になるため、
//   errsweep はレジストリから取り出した値のメソッド呼び出しを追跡できない。

// ErrServiceNotFound はレジストリに未登録のサービスを要求した場合のエラー。
var ErrServiceNotFound = errors.New("service not found in registry")

// ErrServiceTypeMismatch はレジストリから取り出した型が期待と一致しない場合のエラー。
var ErrServiceTypeMismatch = errors.New("service type mismatch")

// Registry は名前 → サービスインスタンスの動的マッピング。
type Registry struct {
	services map[string]any
}

// NewRegistry は空のレジストリを生成する。
func NewRegistry() *Registry {
	return &Registry{services: make(map[string]any)}
}

// Register はサービスを名前で登録する。
func (r *Registry) Register(name string, svc any) {
	r.services[name] = svc
}

// Resolve はサービスを名前で取得する。
//
// ■ errsweep 検出:
//   - ErrServiceNotFound : 未登録（直接 return）
//
// ■ errsweep 非検出:
//   戻り値は any 型のため、呼び出し側で型アサーション後に
//   メソッドを呼んでも errsweep は具象を追跡できない。
func (r *Registry) Resolve(name string) (any, error) {
	svc, ok := r.services[name]
	if !ok {
		return nil, fmt.Errorf("Registry.Resolve(%s): %w", name, ErrServiceNotFound)
	}
	return svc, nil
}

// MustResolve はサービスを名前で取得し、存在しない場合は panic する。
func (r *Registry) MustResolve(name string) any {
	svc, err := r.Resolve(name)
	if err != nil {
		panic(err)
	}
	return svc
}
