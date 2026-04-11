package usecase

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
)

// LoadAndApplyTag は「HTTP取得 + 1行読み込み + DBアクセス + context考慮」を
// 1つのユースケースにまとめた、現場寄りの統合例。
//
// 検出される Sentinel（現時点）:
//   - io.EOF                   : bufio.Reader.ReadString('\n')
//   - database/sql.ErrNoRows   : row.Scan(...)
//   - context.Canceled         : ctx.Err()
//   - context.DeadlineExceeded : ctx.Err()
func LoadAndApplyTag(ctx context.Context, db *sql.DB, client *http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("LoadAndApplyTag: request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("LoadAndApplyTag: do: %w", err)
	}
	defer resp.Body.Close()

	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil && err != io.EOF {
		return fmt.Errorf("LoadAndApplyTag: read: %w", err)
	}

	var tagID int64
	if err := db.QueryRowContext(ctx, "SELECT id FROM tags WHERE name = ?", line).Scan(&tagID); err != nil {
		return fmt.Errorf("LoadAndApplyTag: find tag: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
