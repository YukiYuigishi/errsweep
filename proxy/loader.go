package proxy

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// CacheLoader はキャッシュを構築する関数型。
// テストや将来の実装で差し替えられるよう DI 可能な形にしてある。
type CacheLoader func(sentinelfindPath, workspace string) (Cache, error)

var buildCacheTimeout = 60 * time.Second

// SetBuildCacheTimeout は sentinelfind 実行時のタイムアウトを設定する。
func SetBuildCacheTimeout(timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	buildCacheTimeout = timeout
}

// BuildCache は sentinelfind -json を実行して Cache を構築する。
// exit code 3（診断あり）は正常終了として扱う。
func BuildCache(sentinelfindPath, workspace string) (Cache, error) {
	ctx, cancel := context.WithTimeout(context.Background(), buildCacheTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, sentinelfindPath, "-json", "./...")
	cmd.Dir = workspace
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return NewCache(), fmt.Errorf("BuildCache: sentinelfind timeout after %s (workspace=%s)", buildCacheTimeout, workspace)
		}
		// exit code 3 (diagnostics found) は正常
		var ee *exec.ExitError
		if !(errors.As(err, &ee) && ee.ExitCode() == 3) && len(out) == 0 {
			return NewCache(), fmt.Errorf("BuildCache: %w (workspace=%s)", err, workspace)
		}
	}
	if len(out) == 0 {
		return NewCache(), nil
	}
	return ParseSentinelfindJSON(out)
}
