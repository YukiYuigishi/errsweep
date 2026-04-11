package proxy

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

// CacheLoader はキャッシュを構築する関数型。
// テストや将来の実装で差し替えられるよう DI 可能な形にしてある。
type CacheLoader func(sentinelfindPath, workspace string) (Cache, error)

var buildCacheTimeout = 60 * time.Second
var buildCachePattern = "./..."
var buildCacheFilePath = ""

// SetBuildCacheTimeout は sentinelfind 実行時のタイムアウトを設定する。
func SetBuildCacheTimeout(timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	buildCacheTimeout = timeout
}

// SetBuildCachePattern は sentinelfind 実行時のパッケージパターンを設定する。
func SetBuildCachePattern(pattern string) {
	if pattern == "" {
		return
	}
	buildCachePattern = pattern
}

// SetBuildCacheFilePath はローカルキャッシュファイルの保存先を設定する。
func SetBuildCacheFilePath(path string) {
	buildCacheFilePath = path
}

// BuildCache は sentinelfind -json を実行して Cache を構築する。
// exit code 3（診断あり）は正常終了として扱う。
func BuildCache(sentinelfindPath, workspace string) (Cache, error) {
	ctx, cancel := context.WithTimeout(context.Background(), buildCacheTimeout)
	defer cancel()
	cacheFilePath := resolveCacheFilePath(workspace)

	// #nosec G204 -- sentinelfindPath/pattern はローカル開発者が設定する解析対象コマンド引数。
	cmd := exec.CommandContext(ctx, sentinelfindPath, buildCacheArgs()...)
	cmd.Dir = workspace
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			if cached, loadErr := LoadCacheFromFile(cacheFilePath); loadErr == nil {
				return cached, nil
			}
			return NewCache(), fmt.Errorf("BuildCache: sentinelfind timeout after %s (workspace=%s)", buildCacheTimeout, workspace)
		}
		// exit code 3 (diagnostics found) は正常
		var ee *exec.ExitError
		if !(errors.As(err, &ee) && ee.ExitCode() == 3) && len(out) == 0 {
			if cached, loadErr := LoadCacheFromFile(cacheFilePath); loadErr == nil {
				return cached, nil
			}
			return NewCache(), fmt.Errorf("BuildCache: %w (workspace=%s)", err, workspace)
		}
	}
	if len(out) == 0 {
		if cached, loadErr := LoadCacheFromFile(cacheFilePath); loadErr == nil {
			return cached, nil
		}
		return NewCache(), nil
	}
	cache, parseErr := ParseSentinelfindJSON(out)
	if parseErr != nil {
		if cached, loadErr := LoadCacheFromFile(cacheFilePath); loadErr == nil {
			return cached, nil
		}
		return NewCache(), parseErr
	}
	_ = SaveCacheToFile(cache, cacheFilePath)
	return cache, nil
}

func buildCacheArgs() []string {
	return []string{"-json", buildCachePattern}
}

func resolveCacheFilePath(workspace string) string {
	if buildCacheFilePath != "" {
		return buildCacheFilePath
	}
	return filepath.Join(workspace, ".errsweep", "cache.gob")
}
