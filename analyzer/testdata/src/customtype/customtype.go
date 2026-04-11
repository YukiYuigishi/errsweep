package customtype

import (
	"errors"
	"fmt"
)

// ErrGone は従来の var Err* 形式の Sentinel（比較用）。
var ErrGone = errors.New("gone")

// NotFoundError は pointer receiver で error interface を実装する
// カスタムエラー型。value 生成 + ポインタ返却パターン。
type NotFoundError struct {
	ID int
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("not found: %d", e.ID)
}

// ValidationError は value receiver で error interface を実装する
// カスタムエラー型（非ポインタ）。
type ValidationError struct {
	Field string
}

func (e ValidationError) Error() string {
	return "invalid field: " + e.Field
}

// internalErr は unexported なので Sentinel 対象外。
type internalErr struct{}

func (internalErr) Error() string { return "internal" }

// Find はポインタ型のカスタムエラーを返す。
func Find(id int) error { // want `Find returns sentinels: \*customtype\.NotFoundError` Find:`SentinelFact\(\*customtype\.NotFoundError\)`
	if id <= 0 {
		return &NotFoundError{ID: id}
	}
	return nil
}

// Validate は値型のカスタムエラーを返す。
func Validate(field string) error { // want `Validate returns sentinels: customtype\.ValidationError` Validate:`SentinelFact\(customtype\.ValidationError\)`
	if field == "" {
		return ValidationError{Field: field}
	}
	return nil
}

// WrapCustom はカスタム型を fmt.Errorf %w でラップしても
// 内側の型が検出されることを確認する。
func WrapCustom(id int) error { // want `WrapCustom returns sentinels: \*customtype\.NotFoundError` WrapCustom:`SentinelFact\(\*customtype\.NotFoundError\)`
	return fmt.Errorf("wrap: %w", &NotFoundError{ID: id})
}

// Mixed は既存の var Err* とカスタム型の両方を返す。
// Phi 合流で union になるはず。
func Mixed(id int) error { // want `Mixed returns sentinels: \*customtype\.NotFoundError, customtype\.ErrGone` Mixed:`SentinelFact\(\*customtype\.NotFoundError, customtype\.ErrGone\)`
	if id < 0 {
		return ErrGone
	}
	return &NotFoundError{ID: id}
}

// Anonymous は errors.New を返す（匿名エラー）。
// カスタム型ではないので Sentinel として検出されない。
func Anonymous() error {
	return errors.New("anon")
}

// Internal は unexported なカスタム型を返す。
// Sentinel として検出されない。
func Internal() error {
	return &internalErr{}
}
