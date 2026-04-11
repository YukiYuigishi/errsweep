package proxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		workspaceAbs = workspace
	}
	cacheFilePath := resolveCacheFilePath(workspace)
	sourceHash, _ := computeSourceHash(workspaceAbs)
	expectedMeta := CacheMetadata{
		FormatVersion: cacheFormatVersion,
		Workspace:     workspaceAbs,
		Pattern:       buildCachePattern,
		SourceHash:    sourceHash,
	}

	// #nosec G204 -- sentinelfindPath/pattern はローカル開発者が設定する解析対象コマンド引数。
	cmd := exec.CommandContext(ctx, sentinelfindPath, buildCacheArgs()...)
	cmd.Dir = workspaceAbs
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			if cached, loadErr := loadValidCache(cacheFilePath, expectedMeta); loadErr == nil {
				return cached, nil
			}
			return NewCache(), fmt.Errorf("BuildCache: sentinelfind timeout after %s (workspace=%s)", buildCacheTimeout, workspace)
		}
		// exit code 3 (diagnostics found) は正常
		var ee *exec.ExitError
		if !(errors.As(err, &ee) && ee.ExitCode() == 3) && len(out) == 0 {
			if cached, loadErr := loadValidCache(cacheFilePath, expectedMeta); loadErr == nil {
				return cached, nil
			}
			return NewCache(), fmt.Errorf("BuildCache: %w (workspace=%s)", err, workspace)
		}
	}
	if len(out) == 0 {
		if cached, loadErr := loadValidCache(cacheFilePath, expectedMeta); loadErr == nil {
			return cached, nil
		}
		return NewCache(), nil
	}
	cache, parseErr := ParseSentinelfindJSON(out)
	if parseErr != nil {
		if cached, loadErr := loadValidCache(cacheFilePath, expectedMeta); loadErr == nil {
			return cached, nil
		}
		return NewCache(), parseErr
	}
	_ = SaveCacheToFileWithMetadata(cache, cacheFilePath, expectedMeta)
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

func loadValidCache(path string, expected CacheMetadata) (Cache, error) {
	cached, meta, err := LoadCacheFromFileWithMetadata(path)
	if err != nil {
		return NewCache(), err
	}
	if !metadataMatches(meta, expected) {
		return NewCache(), fmt.Errorf("loadValidCache: metadata mismatch")
	}
	return cached, nil
}

// computeSourceHash は workspace 配下の Go 関連ファイル（.go / go.mod / go.sum / go.work）の
// 相対パス・サイズ・mtime を sha256 で畳み込んだ値を返す。
// 別プロセスでキャッシュが共有されたときに「ソースが変わったら無効化」するのが目的。
// .git / .errsweep / node_modules および root 以外の go.mod を持つネストモジュールは走査しない。
func computeSourceHash(workspace string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(workspace, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".errsweep", "node_modules":
				return fs.SkipDir
			}
			// ネストした go.mod は別モジュール（sentinelfind ./... の対象外）なので
			// ハッシュ計算から除外する。root 自身はチェックしない。
			if path != workspace {
				if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
					return fs.SkipDir
				}
			}
			return nil
		}
		name := d.Name()
		if !(strings.HasSuffix(name, ".go") || name == "go.mod" || name == "go.sum" || name == "go.work") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("dirEntryInfo: %w", err)
		}
		rel, err := filepath.Rel(workspace, path)
		if err != nil {
			rel = path
		}
		fmt.Fprintf(h, "%s\x00%d\x00%d\n", filepath.ToSlash(rel), info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("computeSourceHash: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
