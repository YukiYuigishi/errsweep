package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"example.com/myapp/repository"
)

// LoadAndApplyTag は外部I/Oを repository 層へ寄せた実運用寄りユースケース。
// usecase 層は orchestration のみを担い、詳細I/Oは repository に委譲する。
//
// 検出される Sentinel:
//   - io.EOF                           : repository.FetchTagNameFromUpstream
//   - database/sql.ErrNoRows           : repository.ResolveTagID
//   - context.Canceled/DeadlineExceeded: repository.FetchTagNameFromUpstream
//   - repository.ErrUpstreamUnavailable: 外部API障害
func LoadAndApplyTag(ctx context.Context, db *sql.DB, client *http.Client, url string) error {
	tagName, err := repository.FetchTagNameFromUpstream(ctx, client, url)
	if err != nil {
		return fmt.Errorf("LoadAndApplyTag: upstream: %w", err)
	}

	tagID, err := repository.ResolveTagID(ctx, db, tagName)
	if err != nil {
		return fmt.Errorf("LoadAndApplyTag: resolve: %w", err)
	}

	_ = tagID // ここでは適用処理を省略（例示目的）
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("LoadAndApplyTag: context: %w", err)
	}
	return nil
}
