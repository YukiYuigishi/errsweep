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
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/YukiYuigishi/errsweep/proxy"
)

func main() {
	goplsPath := flag.String("gopls", "gopls", "gopls バイナリのパス")
	sentinelfindPath := flag.String("sentinelfind", "sentinelfind", "sentinelfind バイナリのパス")
	workspace := flag.String("workspace", ".", "解析対象のワークスペースディレクトリ")
	flag.Parse()

	// flag.Args() には VS Code が渡してくる gopls サブコマンド・フラグ（"serve" など）が入る
	goplsSubArgs := flag.Args()

	cache, err := buildCache(*sentinelfindPath, *workspace)
	if err != nil {
		log.Printf("sentinel-lsp-proxy: cache build failed (continuing without sentinels): %v", err)
		cache = make(proxy.Cache)
	}
	log.Printf("sentinel-lsp-proxy: loaded %d entries from sentinelfind", len(cache))

	p := proxy.NewProxy(cache)

	// gopls を子プロセスとして起動
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
				if err != io.EOF {
					log.Printf("sentinel-lsp-proxy: read from editor: %v", err)
				}
				goplsIn.Close()
				return
			}
			if err := p.TrackRequest(raw); err != nil {
				log.Printf("sentinel-lsp-proxy: trackRequest: %v", err)
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
			if err != io.EOF {
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

// buildCache は sentinelfind -json を実行してキャッシュを構築する。
func buildCache(sentinelfindPath, workspace string) (proxy.Cache, error) {
	cmd := exec.Command(sentinelfindPath, "-json", "./...")
	cmd.Dir = workspace
	out, err := cmd.Output()
	if err != nil {
		// exit code 3 (diagnostics found) は正常
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 3 {
			// fall through
		} else if len(out) == 0 {
			return nil, fmt.Errorf("buildCache: %w (workspace=%s)", err, workspace)
		}
	}
	if len(out) == 0 {
		return make(proxy.Cache), nil
	}
	return proxy.ParseSentinelfindJSON(out)
}
