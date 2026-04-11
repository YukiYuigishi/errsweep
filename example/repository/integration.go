package repository

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var (
	ErrUpstreamUnavailable = errors.New("upstream unavailable")
	ErrInvalidPayload      = errors.New("invalid payload")
)

// FetchTagNameFromUpstream は外部HTTP APIからタグ名を読み取る。
//
// 検出される Sentinel:
//   - io.EOF                   : ReadString の終端
//   - context.Canceled         : ctx.Err()
//   - context.DeadlineExceeded : ctx.Err()
//   - ErrUpstreamUnavailable   : 4xx/5xx を業務エラーとして扱う
func FetchTagNameFromUpstream(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("FetchTagNameFromUpstream: request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("FetchTagNameFromUpstream: do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return "", ErrUpstreamUnavailable
	}
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("FetchTagNameFromUpstream: read: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return line, nil
}

// ResolveTagID はタグ名から DB のタグIDを引く。
//
// 検出される Sentinel:
//   - database/sql.ErrNoRows : DB に存在しない
func ResolveTagID(ctx context.Context, db *sql.DB, name string) (int64, error) {
	var tagID int64
	if err := db.QueryRowContext(ctx, "SELECT id FROM tags WHERE name = ?", name).Scan(&tagID); err != nil {
		return 0, fmt.Errorf("ResolveTagID: %w", err)
	}
	return tagID, nil
}
