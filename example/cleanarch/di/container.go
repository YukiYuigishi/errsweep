// Package di はアプリケーション全体の依存関係を組み立てる。
//
// errsweep 観点:
//   DI コンテナのパターンには静的解決可能なものと不可能なものがある。
//   このファイルでは struct ベースの手動 DI を示す。
//   コンストラクタ内で具象型をインスタンス化するため、RTA が具象型を検出できる。
package di

import (
	"database/sql"

	"example.com/cleanarch/application"
	"example.com/cleanarch/domain/payment"
	"example.com/cleanarch/infra/gateway"
	"example.com/cleanarch/infra/persistence"
)

// Container はアプリケーション全体の依存関係を保持する DI コンテナ。
// struct のフィールドとして具象のユースケースを公開する。
//
// errsweep 観点:
//   Container の各フィールドは具象型（*application.PlaceOrderUseCase 等）なので、
//   Container 経由のメソッド呼び出しは静的に解決される。
type Container struct {
	PlaceOrder  *application.PlaceOrderUseCase
	CancelOrder *application.CancelOrderUseCase
	QueryOrder  *application.QueryOrderUseCase
}

// NewContainer は全依存関係を組み立てて Container を返す。
//
// errsweep 観点:
//   ここで具象型（SQLOrderRepository, StripeGateway 等）がインスタンス化される。
//   RTA はこの関数を起点に具象型を収集し、auto-discovery で
//   interface → concrete の対応を推論できる。
func NewContainer(db *sql.DB, paymentProvider string) *Container {
	// Repository の組み立て（デコレータパターン）
	sqlRepo := persistence.NewSQLOrderRepository(db)
	cachedRepo := persistence.NewCachedOrderRepository(sqlRepo)

	// Payment Gateway の選択
	// Payment Gateway の選択。
	// ここでは具象型を PaymentGateway interface に代入している。
	// compile-time assertion が application パッケージにあるため errsweep は解決可能。
	var gw payment.PaymentGateway
	switch paymentProvider {
	case "paypal":
		gw = gateway.NewPayPalGateway("client-id")
	default:
		gw = gateway.NewStripeGateway("sk-test-key")
	}

	// Query Service の組み立て
	sqlQuery := persistence.NewSQLOrderQueryService(db)
	cachedQuery := persistence.NewCachedOrderQueryService(sqlQuery)

	return &Container{
		PlaceOrder:  application.NewPlaceOrderUseCase(cachedRepo, gw),
		CancelOrder: application.NewCancelOrderUseCase(cachedRepo, gw),
		QueryOrder:  application.NewQueryOrderUseCase(cachedQuery),
	}
}
