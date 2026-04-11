package thirdpartyframeworks

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"entgo.io/ent"
	"github.com/gin-gonic/gin"
	"github.com/sqlc-dev/sqlc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

var ErrDomain = errors.New("domain")

func ViaGin(token string) error { // want `ViaGin returns sentinels: gin\.ErrBadRequest, gin\.ErrUnauthorized` ViaGin:`SentinelFact\(gin\.ErrBadRequest, gin\.ErrUnauthorized\)`
	if token == "" {
		return gin.ErrUnauthorized
	}
	return gin.ErrBadRequest
}

func ViaConnect(version int) error { // want `ViaConnect returns sentinels: connect\.ErrNotModified` ViaConnect:`SentinelFact\(connect\.ErrNotModified\)`
	if version > 0 {
		return nil
	}
	return connect.ErrNotModified
}

func ViaGorm(name string) error { // want `ViaGorm returns sentinels: gorm\.ErrRecordNotFound` ViaGorm:`SentinelFact\(gorm\.ErrRecordNotFound\)`
	if name == "" {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func ViaEnt(id int64) error { // want `ViaEnt returns sentinels: ent\.ErrNotFound` ViaEnt:`SentinelFact\(ent\.ErrNotFound\)`
	if id <= 0 {
		return ent.ErrNotFound
	}
	return nil
}

func ViaSQLC(email string) error { // want `ViaSQLC returns sentinels: sqlc\.ErrNotFound` ViaSQLC:`SentinelFact\(sqlc\.ErrNotFound\)`
	if email == "" {
		return sqlc.ErrNotFound
	}
	return nil
}

func ViaGRPC(ctx context.Context, id int64) error { // want `ViaGRPC returns sentinels: context\.Canceled, context\.DeadlineExceeded, thirdpartyframeworks\.ErrDomain` ViaGRPC:`SentinelFact\(context\.Canceled, context\.DeadlineExceeded, thirdpartyframeworks\.ErrDomain\)`
	if id <= 0 {
		return ErrDomain
	}
	_ = status.Errorf(codes.NotFound, "id=%d", id)
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("ViaGRPC: %w", err)
	}
	return nil
}

func Aggregate(ctx context.Context, token, email string, id int64) error { // want `Aggregate returns sentinels: connect\.ErrNotModified, context\.Canceled, context\.DeadlineExceeded, ent\.ErrNotFound, gin\.ErrBadRequest, gin\.ErrUnauthorized, gorm\.ErrRecordNotFound, sqlc\.ErrNotFound, thirdpartyframeworks\.ErrDomain` Aggregate:`SentinelFact\(connect\.ErrNotModified, context\.Canceled, context\.DeadlineExceeded, ent\.ErrNotFound, gin\.ErrBadRequest, gin\.ErrUnauthorized, gorm\.ErrRecordNotFound, sqlc\.ErrNotFound, thirdpartyframeworks\.ErrDomain\)`
	if err := ViaGin(token); err != nil {
		return fmt.Errorf("Aggregate: gin: %w", err)
	}
	if err := ViaConnect(int(id)); err != nil {
		return fmt.Errorf("Aggregate: connect: %w", err)
	}
	if err := ViaGorm(email); err != nil {
		return fmt.Errorf("Aggregate: gorm: %w", err)
	}
	if err := ViaEnt(id); err != nil {
		return fmt.Errorf("Aggregate: ent: %w", err)
	}
	if err := ViaSQLC(email); err != nil {
		return fmt.Errorf("Aggregate: sqlc: %w", err)
	}
	if err := ViaGRPC(ctx, id); err != nil {
		return fmt.Errorf("Aggregate: grpc: %w", err)
	}
	return nil
}
