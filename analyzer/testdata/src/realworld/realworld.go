package realworld

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrUpstreamUnavailable = errors.New("upstream unavailable")
	ErrInvalidPayload      = errors.New("invalid payload")
)

type upstreamUser struct {
	Name string `json:"name"`
}

// SyncUser は実運用に近い複合ケース:
// - http.Client.Do                 -> *url.Error
// - context.Context.Err()          -> context.Canceled / context.DeadlineExceeded
// - bufio.Reader.ReadString('\n')  -> io.EOF
// - database/sql.Row.Scan          -> sql.ErrNoRows
// - ドメイン sentinel（ErrUpstreamUnavailable / ErrInvalidPayload）
func SyncUser(ctx context.Context, db *sql.DB, client *http.Client, endpoint string) error { // want `SyncUser returns sentinels: \*url\.Error, context\.Canceled, context\.DeadlineExceeded, io\.EOF, realworld\.ErrInvalidPayload, realworld\.ErrUpstreamUnavailable, sql\.ErrNoRows` SyncUser:`SentinelFact\(\*url\.Error, context\.Canceled, context\.DeadlineExceeded, io\.EOF, realworld\.ErrInvalidPayload, realworld\.ErrUpstreamUnavailable, sql\.ErrNoRows\)`
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("SyncUser: new request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("SyncUser: do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return ErrUpstreamUnavailable
	}

	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil {
		return fmt.Errorf("SyncUser: read line: %w", err)
	}

	var payload upstreamUser
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return fmt.Errorf("SyncUser: decode: %w", ErrInvalidPayload)
	}

	var name string
	if err := db.QueryRowContext(ctx, "SELECT name FROM users WHERE name = ?", payload.Name).Scan(&name); err != nil {
		return fmt.Errorf("SyncUser: lookup: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
