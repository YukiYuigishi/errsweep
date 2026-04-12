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
	"sync"
	"time"

	"github.com/YukiYuigishi/errsweep/proxy"
)

// refreshRequest は LSP メッセージから抽出した再解析要求。
// full=true のとき pkgDirs は無視され、フル再解析される。
type refreshRequest struct {
	full    bool
	pkgDirs []string // 変更ファイルの親ディレクトリ（絶対パス）
}

// pendingRefresh は debounce 窓内に蓄積される再解析要求の集約状態。
// 複数の didSave を 1 回の sentinelfind 呼び出しにまとめるために使う。
type pendingRefresh struct {
	mu      sync.Mutex
	full    bool
	pkgDirs map[string]bool
}

func (pr *pendingRefresh) add(req refreshRequest) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if req.full {
		pr.full = true
	}
	if pr.pkgDirs == nil {
		pr.pkgDirs = make(map[string]bool)
	}
	for _, d := range req.pkgDirs {
		pr.pkgDirs[d] = true
	}
}

func (pr *pendingRefresh) drain() (bool, []string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	full := pr.full
	var pkgDirs []string
	for d := range pr.pkgDirs {
		pkgDirs = append(pkgDirs, d)
	}
	pr.full = false
	pr.pkgDirs = nil
	return full, pkgDirs
}

// cacheLoader はキャッシュ構築関数。テストで差し替え可能にするためパッケージ変数にしてある。
var cacheLoader proxy.CacheLoader = proxy.BuildCache

func main() {
	goplsPath := flag.String("gopls", "gopls", "gopls バイナリのパス")
	sentinelfindPath := flag.String("sentinelfind", "sentinelfind", "sentinelfind バイナリのパス")
	workspace := flag.String("workspace", ".", "解析対象のワークスペースディレクトリ")
	cacheTimeout := flag.Duration("cache-timeout", 60*time.Second, "sentinelfind キャッシュ構築のタイムアウト")
	cachePattern := flag.String("cache-pattern", "./...", "sentinelfind の解析対象パッケージパターン")
	cacheFile := flag.String("cache-file", "", "永続キャッシュファイルパス（未指定時: <workspace>/.errsweep/cache.gob）")
	cacheRefreshMinInterval := flag.Duration("cache-refresh-min-interval", 2*time.Second, "キャッシュ再構築の最小間隔")
	cacheFullRefreshInterval := flag.Duration("cache-full-refresh-interval", 5*time.Minute, "partial 再解析を full に昇格させる最小経過時間（0 で無効）")
	flag.Parse()
	proxy.SetBuildCacheTimeout(*cacheTimeout)
	proxy.SetBuildCachePattern(*cachePattern)
	proxy.SetBuildCacheFilePath(*cacheFile)
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
	pending := &pendingRefresh{}
	initialFullRefresh := time.Now()

	// 差分再解析: 保存/ファイル変更通知を受けたら debounce してキャッシュを再構築する。
	// 変更が .go ファイルのみなら該当パッケージだけ partial で再解析し、既存キャッシュに
	// マージする。go.mod/go.sum/go.work が変更されたら安全側に倒してフル再解析する。
	// safety net: partial が続いて一定時間 full が走っていないと、依存パッケージの
	// ホバーが drift する可能性があるため、経過 >= cacheFullRefreshInterval なら
	// partial を full に昇格させる。
	go func() {
		var lastRefreshAttempt time.Time
		lastFullRefresh := initialFullRefresh
		for range refreshCh {
			time.Sleep(300 * time.Millisecond) // debounce
			now := time.Now()
			if !shouldRunRefresh(now, lastRefreshAttempt, *cacheRefreshMinInterval) {
				continue
			}
			lastRefreshAttempt = now
			full, pkgDirs := pending.drain()
			upgraded := false
			if !full && len(pkgDirs) > 0 && shouldUpgradeToFull(now, lastFullRefresh, *cacheFullRefreshInterval) {
				full = true
				upgraded = true
			}
			refreshStart := time.Now()
			if full || len(pkgDirs) == 0 {
				newCache, err := cacheLoader(*sentinelfindPath, *workspace)
				refreshElapsed := time.Since(refreshStart)
				if err != nil {
					log.Printf("sentinel-lsp-proxy: cache refresh failed after %s: %v", refreshElapsed, err)
					continue
				}
				p.SetCache(newCache)
				lastFullRefresh = time.Now()
				if upgraded {
					log.Printf("sentinel-lsp-proxy: cache refreshed full upgraded (%d entries, %s, drift-pkgs=%d)", newCache.Len(), refreshElapsed, len(pkgDirs))
				} else {
					log.Printf("sentinel-lsp-proxy: cache refreshed full (%d entries, %s)", newCache.Len(), refreshElapsed)
				}
				continue
			}
			partial, err := proxy.BuildCachePartial(*sentinelfindPath, *workspace, pkgDirs)
			refreshElapsed := time.Since(refreshStart)
			if err != nil {
				log.Printf("sentinel-lsp-proxy: cache refresh partial failed after %s (pkgs=%d): %v", refreshElapsed, len(pkgDirs), err)
				continue
			}
			p.MergePartial(partial, pkgDirs)
			log.Printf("sentinel-lsp-proxy: cache refreshed partial (%d entries, %s, pkgs=%d)", p.CacheLen(), refreshElapsed, len(pkgDirs))
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
			if req, ok := parseRefreshRequest(raw, workspaceRoot); ok {
				pending.add(req)
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

// shouldUpgradeToFull は partial 再解析を full に昇格させるべきかを返す。
// interval <= 0 は safety net 無効（昇格しない）。
// lastFull が zero の場合は「full を一度も走らせていない」状態で、保守的に昇格させる。
func shouldUpgradeToFull(now, lastFull time.Time, interval time.Duration) bool {
	if interval <= 0 {
		return false
	}
	if lastFull.IsZero() {
		return true
	}
	return now.Sub(lastFull) >= interval
}

// parseRefreshRequest は LSP クライアントメッセージから再解析要求を抽出する。
// 何も抽出できなければ (zero, false) を返す。
// go.mod / go.sum / go.work の変更は依存ツリー全体に影響するので full=true にする。
// .go のみが変更された場合は、そのファイルの親ディレクトリを pkgDirs に入れて
// partial 再解析の対象にする。
func parseRefreshRequest(raw []byte, workspaceRoot string) (refreshRequest, bool) {
	var msg struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil || msg.Method == "" {
		return refreshRequest{}, false
	}
	uris := extractRefreshURIs(msg.Method, msg.Params)
	return refreshRequestFromURIs(uris, workspaceRoot)
}

// extractRefreshURIs は再解析トリガーになりうる LSP 通知からファイル URI を取り出す。
func extractRefreshURIs(method string, params json.RawMessage) []string {
	switch method {
	case "textDocument/didSave":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil
		}
		return []string{p.TextDocument.URI}
	case "workspace/didChangeWatchedFiles":
		var p struct {
			Changes []struct {
				URI string `json:"uri"`
			} `json:"changes"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil
		}
		out := make([]string, 0, len(p.Changes))
		for _, ch := range p.Changes {
			out = append(out, ch.URI)
		}
		return out
	case "workspace/didRenameFiles":
		var p struct {
			Files []struct {
				OldURI string `json:"oldUri"`
				NewURI string `json:"newUri"`
			} `json:"files"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil
		}
		out := make([]string, 0, 2*len(p.Files))
		for _, f := range p.Files {
			out = append(out, f.OldURI, f.NewURI)
		}
		return out
	case "workspace/didCreateFiles", "workspace/didDeleteFiles":
		var p struct {
			Files []struct {
				URI string `json:"uri"`
			} `json:"files"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil
		}
		out := make([]string, 0, len(p.Files))
		for _, f := range p.Files {
			out = append(out, f.URI)
		}
		return out
	default:
		return nil
	}
}

func refreshRequestFromURIs(uris []string, workspaceRoot string) (refreshRequest, bool) {
	var req refreshRequest
	ok := false
	seenDirs := make(map[string]bool)
	for _, uri := range uris {
		path, accepted := uriToWorkspacePath(uri, workspaceRoot)
		if !accepted {
			continue
		}
		lower := strings.ToLower(path)
		switch {
		case strings.HasSuffix(lower, "/go.mod"),
			strings.HasSuffix(lower, "/go.sum"),
			strings.HasSuffix(lower, "/go.work"):
			req.full = true
			ok = true
		case strings.HasSuffix(lower, ".go"):
			dir := filepath.Dir(path)
			if !seenDirs[dir] {
				seenDirs[dir] = true
				req.pkgDirs = append(req.pkgDirs, dir)
			}
			ok = true
		}
	}
	return req, ok
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
