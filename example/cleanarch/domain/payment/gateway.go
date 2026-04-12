package payment

import "context"

// PaymentGateway は決済処理を抽象化するインターフェース。
// Stripe / PayPal など複数のプロバイダ実装を切り替え可能にする。
//
// errsweep 観点:
//   複数の具象実装が存在する場合、compile-time assertion があれば
//   全具象の sentinel を union として報告し、さらに concrete ごとの
//   内訳（breakdown）も表示する。
type PaymentGateway interface {
	Charge(ctx context.Context, amount int, card Card) (ChargeResult, error)
	Refund(ctx context.Context, chargeID string) (RefundResult, error)
}
