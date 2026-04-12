package handler

import (
	"context"
	"errors"
	"fmt"
)

// ======================================================================
// パターン J: ミドルウェア / デコレータ（クロージャ経由の検出不可能パターン）
// ======================================================================

// ErrUnauthorized は認証エラー。
var ErrUnauthorized = errors.New("unauthorized")

// ErrRateLimited はレート制限エラー。
var ErrRateLimited = errors.New("rate limited")

// HandlerFunc はミドルウェアが受け取るハンドラ関数型。
type HandlerFunc func(ctx context.Context) error

// Middleware はハンドラを受け取り新しいハンドラを返すミドルウェア型。
type Middleware func(next HandlerFunc) HandlerFunc

// WithAuth は認証ミドルウェア。
//
// ■ errsweep 検出:
//   - ErrUnauthorized : 認証失敗（直接 return）
//
// ■ errsweep 非検出:
//   next(ctx) の呼び出し先はクロージャパラメータのため追跡不可能。
//   next が返す sentinel は errsweep の解析範囲外。
func WithAuth(next HandlerFunc) HandlerFunc {
	return func(ctx context.Context) error {
		token := ctx.Value("auth_token")
		if token == nil {
			return ErrUnauthorized
		}
		if err := next(ctx); err != nil {
			return fmt.Errorf("auth middleware: %w", err)
		}
		return nil
	}
}

// WithRateLimit はレート制限ミドルウェア。
//
// ■ errsweep 検出:
//   - ErrRateLimited : レート超過（直接 return）
//
// ■ errsweep 非検出:
//   next(ctx) のエラーは %w でラップしているが、next の実体が不明なため
//   sentinel は追跡不可能。
func WithRateLimit(maxRequests int) Middleware {
	count := 0
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context) error {
			count++
			if count > maxRequests {
				return ErrRateLimited
			}
			if err := next(ctx); err != nil {
				return fmt.Errorf("rate limit middleware: %w", err)
			}
			return nil
		}
	}
}

// Chain はミドルウェアを連結する。
//
// ■ errsweep 非検出:
//   Chain 自体は sentinel を返さないが、各ミドルウェアの sentinel は
//   クロージャチェーンの内部にあるため、最終的なハンドラの sentinel を
//   統合的に報告することはできない。
func Chain(handler HandlerFunc, middlewares ...Middleware) HandlerFunc {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}
