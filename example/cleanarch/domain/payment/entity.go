package payment

// Card は決済カード情報の値オブジェクト。
type Card struct {
	Number   string
	ExpMonth int
	ExpYear  int
	CVC      string
}

// ChargeResult は決済結果の値オブジェクト。
type ChargeResult struct {
	ChargeID string
	Status   string
}

// RefundResult は返金結果の値オブジェクト。
type RefundResult struct {
	RefundID string
	Status   string
}
