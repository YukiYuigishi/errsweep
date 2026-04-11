package thirdparty

import (
	"context"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	echo "github.com/labstack/echo/v4"
	redis "github.com/redis/go-redis/v9"
)

var ErrDecode = errors.New("decode failed")

func LoadProfile(ctx context.Context, rc *redis.Client, key string) error { // want `LoadProfile returns sentinels: thirdparty\.ErrDecode` LoadProfile:`SentinelFact\(thirdparty\.ErrDecode\)`
	if key == "" {
		return redis.Nil
	}
	cmd := rc.Get(ctx, key)
	if err := cmd.Err(); err != nil {
		return fmt.Errorf("LoadProfile: get: %w", err)
	}
	return ErrDecode
}

func FindUserByEmail(email string) error { // want `FindUserByEmail returns sentinels: sqlx\.ErrNotFound` FindUserByEmail:`SentinelFact\(sqlx\.ErrNotFound\)`
	if err := sqlx.Get(nil, "SELECT * FROM users WHERE email = ?", email); err != nil {
		return fmt.Errorf("FindUserByEmail: %w", err)
	}
	return nil
}

func BuildHTTPError(role string) error { // want `BuildHTTPError returns sentinels: v4\.ErrMethodNotAllowed, v4\.ErrNotFound, v4\.ErrUnauthorized` BuildHTTPError:`SentinelFact\(v4\.ErrMethodNotAllowed, v4\.ErrNotFound, v4\.ErrUnauthorized\)`
	if role == "" {
		return echo.ErrUnauthorized
	}
	if role == "guest" {
		return echo.ErrNotFound
	}
	return echo.ErrMethodNotAllowed
}

func HandleRequest(ctx context.Context, rc *redis.Client, email string) error { // want `HandleRequest returns sentinels: sqlx\.ErrNotFound, thirdparty\.ErrDecode, v4\.ErrMethodNotAllowed, v4\.ErrNotFound, v4\.ErrUnauthorized` HandleRequest:`SentinelFact\(sqlx\.ErrNotFound, thirdparty\.ErrDecode, v4\.ErrMethodNotAllowed, v4\.ErrNotFound, v4\.ErrUnauthorized\)`
	if err := LoadProfile(ctx, rc, email); err != nil {
		return fmt.Errorf("HandleRequest: profile: %w", err)
	}
	if err := FindUserByEmail(email); err != nil {
		return fmt.Errorf("HandleRequest: user: %w", err)
	}
	if err := BuildHTTPError(email); err != nil {
		return fmt.Errorf("HandleRequest: http: %w", err)
	}
	return nil
}
