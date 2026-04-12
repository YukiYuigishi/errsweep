package application

import (
	"context"
	"errors"
	"fmt"

	"example.com/cleanarch/domain/order"
)

// ======================================================================
// パターン D: errsweep が検出できないパターン集
// ======================================================================
// このファイルは意図的に「検出不可能」なパターンを集めたもの。
// errsweep のユーザーがツールの限界を理解するために参照する。

// --- D-1: クロージャに渡された関数パラメータ ---
// 関数パラメータは SSA 上で静的に解決できないため、
// errsweep は呼び出し先の sentinel を追跡できない。

// ExecuteWithRetry は任意の関数をリトライ付きで実行する。
//
// ■ errsweep 非検出:
//   fn パラメータの実体は呼び出し側で決まるため、
//   errsweep は fn が返す sentinel を追跡できない。
func ExecuteWithRetry(ctx context.Context, maxRetries int, fn func(ctx context.Context) error) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := fn(ctx); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return fmt.Errorf("ExecuteWithRetry: max retries exceeded: %w", lastErr)
}

// --- D-2: ファクトリ関数が interface を返すパターン ---
// ファクトリの戻り値型が interface の場合、呼び出し側では具象型が不明。
// compile-time assertion は "この型はこの interface を実装する" という宣言であり、
// "この関数がこの具象を返す" という宣言ではない。

// PricingStrategyFactory は設定値に基づいて PricingStrategy を返す。
// errsweep は戻り値のインターフェースから具象を特定できない。
func PricingStrategyFactory(strategyType string) order.PricingStrategy {
	switch strategyType {
	case "discount":
		return &discountPricing{}
	case "premium":
		return &premiumPricing{}
	default:
		return &standardPricing{}
	}
}

// --- D-3: カスタムエラーラッパー（fmt.Errorf %w 以外） ---
// errsweep は fmt.Errorf の %w 動詞のみをラップとして認識する。
// 独自の Wrap 関数やサードパーティの errors.Wrap は追跡対象外。

// AppError はアプリケーション固有のエラーラッパー。
type AppError struct {
	Op    string // 操作名
	Cause error  // 元エラー
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %v", e.Op, e.Cause)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// wrapError はカスタムラッパー。
// Unwrap() を実装しているため errors.Is/As は動作するが、
// errsweep は fmt.Errorf %w 以外のラップを追跡しない。
func wrapError(op string, err error) error {
	return &AppError{Op: op, Cause: err}
}

// PlaceOrderWithCustomWrap は wrapError を使ってエラーをラップする。
//
// ■ errsweep 検出（union）:
//   - *AppError : wrapError が &AppError{} を返すため、カスタム型として検出
//
// ■ errsweep 検出（breakdown のみ — union には含まれない）:
//   repo.FindByID は interface invoke なので concrete ごとの内訳が報告される:
//   - via *SQLOrderRepository:    order.ErrOrderNotFound, sql.ErrNoRows
//   - via *CachedOrderRepository: persistence.ErrCacheCorrupted
//   ただし、これらは union sentinel には入らない。wrapError が fmt.Errorf %w
//   でないため、wrapError(err) の err を通じた sentinel 連鎖は切断される。
//
// ■ 重要な��見:
//   union と breakdown の乖離が発生する。breakdown は関数内の invoke を
//   網羅的に報告するが、union は return 文から逆方向に辿った結果のみ。
//   カスタムラッパーは union を断ち切るが、breakdown は残る。
func PlaceOrderWithCustomWrap(ctx context.Context, repo order.OrderRepository, orderID string) error {
	_, err := repo.FindByID(ctx, orderID)
	if err != nil {
		return wrapError("PlaceOrderWithCustomWrap", err)
	}
	return nil
}

// --- D-4: map ベースのディスパッチ ---
// map に格納された関数の呼び出し先は SSA 上で静的に解決できない。

// ErrActionNotFound はアクション未定義エラー。
var ErrActionNotFound = errors.New("action not found")

type orderAction func(ctx context.Context, orderID string) error

// OrderActionDispatcher はアクション名で処理を振り分ける。
type OrderActionDispatcher struct {
	actions map[string]orderAction
}

func NewOrderActionDispatcher() *OrderActionDispatcher {
	return &OrderActionDispatcher{
		actions: make(map[string]orderAction),
	}
}

func (d *OrderActionDispatcher) Register(name string, action orderAction) {
	d.actions[name] = action
}

// Dispatch はアクション名に対応する処理を実行する。
//
// ■ errsweep 検出:
//   - ErrActionNotFound : 未定義アクション（直接 return）
//
// ■ errsweep 非検出:
//   action(ctx, orderID) の呼び出し先は map から取得された関数変数であり、
//   パッケージレベル var への初期化ではないため errsweep は追跡できない。
func (d *OrderActionDispatcher) Dispatch(ctx context.Context, action string, orderID string) error {
	fn, ok := d.actions[action]
	if !ok {
		return ErrActionNotFound
	}
	if err := fn(ctx, orderID); err != nil {
		return fmt.Errorf("OrderActionDispatcher.Dispatch(%s): %w", action, err)
	}
	return nil
}

// --- D-5: メソッド値のコールバック ---
// obj.Method を関数値として渡すパターン。
// パッケージレベル var への代入でなければ errsweep は追跡できない。

// ProcessWithCallback はコールバック関数を受け取って実行する。
//
// ■ errsweep 非検出:
//   callback パラメータが実際にどのメソッドかは呼び出し側で決まる。
func ProcessWithCallback(ctx context.Context, callback func(ctx context.Context, id string) (order.Order, error), id string) (order.Order, error) {
	o, err := callback(ctx, id)
	if err != nil {
		return order.Order{}, fmt.Errorf("ProcessWithCallback: %w", err)
	}
	return o, nil
}

// --- D-6: fmt.Errorf %v による sentinel identity の喪失 ---
// %w ではなく %v を使うと errors.Is/As で元エラーを辿れなくなる。
// errsweep はこのパターンを検出して警告を出す（sentinel 追跡はスキップ）。

// PlaceOrderLoseIdentity は %v でエラーをラップする（非推奨パターン）。
//
// ■ errsweep の挙動:
//   fmt.Errorf の %v は元エラーの同一性が失われるため、
//   errsweep は "fmt.Errorf without %w loses sentinel identity" 警告を出し、
//   %v 経由の sentinel 追跡をスキップする。
//   ただし、breakdown は invoke 呼び出しを直接走査するため、
//   concrete ごとの内訳は報告される（union には入らない）。
//
// ■ errsweep 検出（breakdown のみ）:
//   - via *SQLOrderRepository:    order.ErrOrderNotFound, sql.ErrNoRows
//   - via *CachedOrderRepository: persistence.ErrCacheCorrupted
func PlaceOrderLoseIdentity(ctx context.Context, repo order.OrderRepository, orderID string) error {
	_, err := repo.FindByID(ctx, orderID)
	if err != nil {
		// %v → sentinel identity が失われる → errsweep は追跡しない
		return fmt.Errorf("PlaceOrderLoseIdentity: %v", err) //nolint:errorlint
	}
	return nil
}

// --- 内部実装: PricingStrategy の具象 ---
// これらは非エクスポートのため compile-time assertion を書けない。
// auto-discovery も非エクスポート型は対象外。

// ErrDiscountExpired は割引期限切れエラー。
var ErrDiscountExpired = errors.New("discount expired")

// ErrPremiumRequired はプレミアム会員限定エラー。
var ErrPremiumRequired = errors.New("premium membership required")

type standardPricing struct{}

func (p *standardPricing) Calculate(o order.Order) (int, error) {
	return o.TotalAmount(), nil
}

type discountPricing struct{}

func (p *discountPricing) Calculate(o order.Order) (int, error) {
	total := o.TotalAmount()
	if total < 1000 {
		return 0, ErrDiscountExpired
	}
	return total * 90 / 100, nil
}

type premiumPricing struct{}

func (p *premiumPricing) Calculate(o order.Order) (int, error) {
	if len(o.Items) == 0 {
		return 0, ErrPremiumRequired
	}
	return o.TotalAmount() * 80 / 100, nil
}
