package order

import "context"

// OrderRepository は注文の永続化を抽象化するリポジトリインターフェース。
// Clean Architecture において domain 層はインターフェースのみを定義し、
// 具象実装は infra 層に配置する。
//
// errsweep がこのインターフェース経由の呼び出しを解決するには、
// 以下のいずれかが必要:
//   - compile-time assertion: var _ OrderRepository = (*ConcreteRepo)(nil)
//   - auto-discovery: 具象型が直接インポートのスコープに存在する
//   - RTA: ランタイムで具象型がインスタンス化されている
type OrderRepository interface {
	FindByID(ctx context.Context, id string) (Order, error)
	Save(ctx context.Context, order Order) error
	Delete(ctx context.Context, id string) error
}

// OrderQueryService は読み取り専用のクエリサービス（CQRS パターン）。
// 複数の具象実装を持つことで、errsweep の複数 concrete 解決を検証する。
type OrderQueryService interface {
	ListByUser(ctx context.Context, userID string) ([]Order, error)
	CountByStatus(ctx context.Context, status Status) (int, error)
}
