package order

import "time"

// Order は注文集約ルート。
type Order struct {
	ID        string
	UserID    string
	Items     []OrderItem
	Status    Status
	ChargeID  string // 決済ID（payment gateway が返す）
	CreatedAt time.Time
}

// OrderItem は注文明細。
type OrderItem struct {
	ProductID string
	Quantity  int
	UnitPrice int
}

// Status は注文ステータス。
type Status string

const (
	StatusPending   Status = "pending"
	StatusPaid      Status = "paid"
	StatusCancelled Status = "cancelled"
	StatusRefunded  Status = "refunded"
)

// TotalAmount は注文合計金額を返す。
func (o Order) TotalAmount() int {
	total := 0
	for _, item := range o.Items {
		total += item.UnitPrice * item.Quantity
	}
	return total
}
