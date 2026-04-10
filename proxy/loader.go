package proxy

import (
	"fmt"
	"os/exec"
)

// CacheLoader はキャッシュを構築する関数型。
// テストや将来の実装で差し替えられるよう DI 可能な形にしてある。
type CacheLoader func(sentinelfindPath, workspace string) (Cache, error)

// BuildCache は sentinelfind -json を実行して Cache を構築する。
// exit code 3（診断あり）は正常終了として扱う。
func BuildCache(sentinelfindPath, workspace string) (Cache, error) {
	cmd := exec.Command(sentinelfindPath, "-json", "./...")
	cmd.Dir = workspace
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 3 {
			// exit code 3 (diagnostics found) は正常
		} else if len(out) == 0 {
			return NewCache(), fmt.Errorf("BuildCache: %w (workspace=%s)", err, workspace)
		}
	}
	if len(out) == 0 {
		return NewCache(), nil
	}
	return ParseSentinelfindJSON(out)
}
