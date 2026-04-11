// sentinel-lsp-proxy は gopls と エディタの間に挟まる LSP プロキシ。
// textDocument/hover レスポンスに Sentinel Error 情報を追記する。
//
// 使い方:
//
//	sentinel-lsp-proxy [--gopls PATH] [--sentinelfind PATH] [--workspace DIR] [gopls args...]
//
// VS Code での設定例:
//
//	"go.alternateTools": {"gopls": "/path/to/sentinel-lsp-proxy"},
//	"go.languageServerFlags": [
//	  "--gopls=/path/to/gopls",
//	  "--sentinelfind=/path/to/sentinelfind",
//	  "--workspace=/path/to/project"
//	]
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/YukiYuigishi/errsweep/proxy"
)

// cacheLoader はキャッシュ構築関数。テストで差し替え可能にするためパッケージ変数にしてある。
var cacheLoader proxy.CacheLoader = proxy.BuildCache

func main() {
	goplsPath := flag.String("gopls", "gopls", "gopls バイナリのパス")
	sentinelfindPath := flag.String("sentinelfind", "sentinelfind", "sentinelfind バイナリのパス")
	workspace := flag.String("workspace", ".", "解析対象のワークスペースディレクトリ")
	cacheTimeout := flag.Duration("cache-timeout", 60*time.Second, "sentinelfind キャッシュ構築のタイムアウト")
	cacheRefreshMinInterval := flag.Duration("cache-refresh-min-interval", 2*time.Second, "キャッシュ再構築の最小間隔")
	flag.Parse()
	proxy.SetBuildCacheTimeout(*cacheTimeout)
	workspaceRoot, err := filepath.Abs(*workspace)
	if err != nil {
		log.Fatalf("sentinel-lsp-proxy: resolve workspace path: %v", err)
	}

	// flag.Args() には VS Code が渡してくる gopls サブコマンド・フラグ（"serve" など）が入る
	goplsSubArgs := flag.Args()
	initialBuildStart := time.Now()
	cache, err := cacheLoader(*sentinelfindPath, *workspace)
	initialBuildElapsed := time.Since(initialBuildStart)
	if err != nil {
		log.Printf("sentinel-lsp-proxy: cache build failed after %s (continuing without sentinels): %v", initialBuildElapsed, err)
		cache = proxy.NewCache()
	}
	log.Printf("sentinel-lsp-proxy: loaded %d entries from sentinelfind in %s", cache.Len(), initialBuildElapsed)
	p := proxy.NewProxy(cache)
	refreshCh := make(chan struct{}, 1)

	// 差分再解析: 保存/ファイル変更通知を受けたら debounce してキャッシュを再構築する。
	go func() {
		var lastRefreshAttempt time.Time
		for range refreshCh {
			time.Sleep(300 * time.Millisecond) // debounce
			for len(refreshCh) > 0 {
				<-refreshCh
			}
			now := time.Now()
			if !shouldRunRefresh(now, lastRefreshAttempt, *cacheRefreshMinInterval) {
				continue
			}
			lastRefreshAttempt = now
			refreshStart := time.Now()
			cache, err := cacheLoader(*sentinelfindPath, *workspace)
			refreshElapsed := time.Since(refreshStart)
			if err != nil {
				log.Printf("sentinel-lsp-proxy: cache refresh failed after %s: %v", refreshElapsed, err)
				continue
			}
			p.SetCache(cache)
			log.Printf("sentinel-lsp-proxy: cache refreshed (%d entries, %s)", cache.Len(), refreshElapsed)
		}
	}()

	// gopls を子プロセスとして起動
	// #nosec G204 -- goplsPath/goplsSubArgs はローカル開発者が明示指定するツール実行用引数。
	gopls := exec.Command(*goplsPath, goplsSubArgs...)
	goplsIn, err := gopls.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	goplsOut, err := gopls.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	gopls.Stderr = os.Stderr
	if err := gopls.Start(); err != nil {
		log.Fatalf("sentinel-lsp-proxy: failed to start %s: %v", *goplsPath, err)
	}

	// エディタ → gopls へのパイプ（リクエストのトラッキング付き）
	go func() {
		editorReader := bufio.NewReader(os.Stdin)
		for {
			raw, err := proxy.ReadMessage(editorReader)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					log.Printf("sentinel-lsp-proxy: read from editor: %v", err)
				}
				goplsIn.Close()
				return
			}
			if err := p.TrackRequest(raw); err != nil {
				log.Printf("sentinel-lsp-proxy: trackRequest: %v", err)
			}
			if shouldRefreshCache(raw, workspaceRoot) {
				select {
				case refreshCh <- struct{}{}:
				default:
				}
			}
			if err := proxy.WriteMessage(goplsIn, raw); err != nil {
				log.Printf("sentinel-lsp-proxy: write to gopls: %v", err)
				return
			}
		}
	}()

	// gopls → エディタへのパイプ（hover インターセプト付き）
	goplsReader := bufio.NewReader(goplsOut)
	for {
		raw, err := proxy.ReadMessage(goplsReader)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("sentinel-lsp-proxy: read from gopls: %v", err)
			}
			break
		}
		if err := p.ProcessServerMessage(raw, os.Stdout); err != nil {
			log.Printf("sentinel-lsp-proxy: processServerMessage: %v", err)
		}
	}

	if err := gopls.Wait(); err != nil {
		log.Printf("sentinel-lsp-proxy: gopls exited: %v", err)
	}
}

func shouldRunRefresh(now, last time.Time, minInterval time.Duration) bool {
	if minInterval <= 0 || last.IsZero() {
		return true
	}
	return now.Sub(last) >= minInterval
}

func shouldRefreshCache(raw []byte, workspaceRoot string) bool {
	var msg struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil || msg.Method == "" {
		return false
	}
	switch msg.Method {
	case "textDocument/didSave":
		var params struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return false
		}
		return shouldRefreshByURI(params.TextDocument.URI, workspaceRoot)
	case "workspace/didChangeWatchedFiles":
		var params struct {
			Changes []struct {
				URI string `json:"uri"`
			} `json:"changes"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return false
		}
		for _, ch := range params.Changes {
			if shouldRefreshByURI(ch.URI, workspaceRoot) {
				return true
			}
		}
		return false
	case "workspace/didRenameFiles":
		var params struct {
			Files []struct {
				OldURI string `json:"oldUri"`
				NewURI string `json:"newUri"`
			} `json:"files"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return false
		}
		for _, f := range params.Files {
			if shouldRefreshByURI(f.OldURI, workspaceRoot) || shouldRefreshByURI(f.NewURI, workspaceRoot) {
				return true
			}
		}
		return false
	case "workspace/didCreateFiles", "workspace/didDeleteFiles":
		var params struct {
			Files []struct {
				URI string `json:"uri"`
			} `json:"files"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return false
		}
		for _, f := range params.Files {
			if shouldRefreshByURI(f.URI, workspaceRoot) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func shouldRefreshByURI(uri, workspaceRoot string) bool {
	if uri == "" {
		return false
	}
	path, ok := uriToWorkspacePath(uri, workspaceRoot)
	if !ok {
		return false
	}
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".go") ||
		strings.HasSuffix(lower, "/go.mod") ||
		strings.HasSuffix(lower, "/go.sum") ||
		strings.HasSuffix(lower, "/go.work")
}

func uriToWorkspacePath(uri, workspaceRoot string) (string, bool) {
	const fileScheme = "file://"
	if !strings.HasPrefix(uri, fileScheme) {
		return "", false
	}
	u, err := url.Parse(uri)
	if err != nil {
		return "", false
	}
	p, err := url.PathUnescape(u.Path)
	if err != nil {
		return "", false
	}
	if p == "" {
		return "", false
	}
	absPath := filepath.Clean(p)
	root := filepath.Clean(workspaceRoot)
	if root == "" {
		return "", false
	}
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return "", false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(absPath), true
}
