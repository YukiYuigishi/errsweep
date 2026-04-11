// sentinel-lsp はセンチネルエラー情報を提供するミニマルな LSP サーバー。
// gopls とは独立して動作し、textDocument/hover のみ実装する。
//
// 使い方:
//
//	sentinel-lsp [--sentinelfind PATH] [--workspace DIR]
//
// VS Code での設定例 (gopls と並列ではなく単体サーバーとして使う場合):
//
//	"go.alternateTools": {"gopls": "/path/to/sentinel-lsp"},
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/YukiYuigishi/errsweep/proxy"
)

// cacheLoader はキャッシュ構築関数。テストで差し替え可能にするためパッケージ変数にしてある。
var cacheLoader proxy.CacheLoader = proxy.BuildCache

func main() {
	sentinelfindPath := flag.String("sentinelfind", "sentinelfind", "sentinelfind バイナリのパス")
	workspace := flag.String("workspace", ".", "解析対象のワークスペースディレクトリ")
	flag.Parse()

	cache, err := cacheLoader(*sentinelfindPath, *workspace)
	if err != nil {
		log.Printf("sentinel-lsp: cache build failed (continuing without sentinels): %v", err)
		cache = proxy.NewCache()
	}
	log.Printf("sentinel-lsp: loaded %d entries", cache.Len())

	srv := &server{cache: cache}
	srv.run(os.Stdin, os.Stdout)
}

// --- JSON-RPC 2.0 ---

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// --- LSP server ---

type server struct {
	cache    proxy.Cache
	shutdown bool
}

func (s *server) run(r io.Reader, w io.Writer) {
	br := bufio.NewReader(r)
	for {
		raw, err := proxy.ReadMessage(br)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("sentinel-lsp: read: %v", err)
			}
			return
		}

		var msg rpcMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("sentinel-lsp: parse: %v", err)
			continue
		}

		// notification (id なし) → レスポンス不要
		if len(msg.ID) == 0 {
			if exit := s.handleNotif(msg.Method); exit {
				return
			}
			continue
		}

		result, rpcErr := s.handleRequest(msg.Method, msg.Params)
		if err := s.reply(w, msg.ID, result, rpcErr); err != nil {
			log.Printf("sentinel-lsp: reply: %v", err)
		}
	}
}

// handleNotif は通知を処理する。true を返すとサーバーを終了する。
func (s *server) handleNotif(method string) bool {
	switch method {
	case "exit":
		code := 1
		if s.shutdown {
			code = 0
		}
		os.Exit(code)
	}
	return false
}

func (s *server) handleRequest(method string, params json.RawMessage) (interface{}, *rpcError) {
	switch method {
	case "initialize":
		return s.handleInitialize()
	case "shutdown":
		s.shutdown = true
		return nil, nil
	case "textDocument/hover":
		return s.handleHover(params)
	default:
		return nil, &rpcError{Code: -32601, Message: "method not found: " + method}
	}
}

func (s *server) handleInitialize() (interface{}, *rpcError) {
	return map[string]interface{}{
		"capabilities": map[string]interface{}{
			"hoverProvider": true,
		},
		"serverInfo": map[string]interface{}{
			"name":    "sentinel-lsp",
			"version": "0.1.0",
		},
	}, nil
}

func (s *server) handleHover(params json.RawMessage) (interface{}, *rpcError) {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position struct {
			Line int `json:"line"` // 0-indexed
		} `json:"position"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: -32602, Message: "invalid params: " + err.Error()}
	}

	file := uriToPath(p.TextDocument.URI)
	line := p.Position.Line + 1 // 0-indexed → 1-indexed

	entry, ok := s.cache.Lookup(file, line)
	if !ok {
		// 定義行以外（呼び出し側）の hover は位置では引けないため、
		// コンテキスト中のシンボル名/hover contents から関数名でフォールバックする。
		var pName struct {
			Context struct {
				Symbol struct {
					Name string `json:"name"`
				} `json:"symbol"`
			} `json:"context"`
			Contents json.RawMessage `json:"contents"`
		}
		if err := json.Unmarshal(params, &pName); err == nil {
			name := pName.Context.Symbol.Name
			if name == "" {
				name = hoverFuncNameFromContents(pName.Contents)
			}
			if ssaName, simple := extractFuncNames(name); ssaName != "" {
				entry, ok = s.cache.LookupByFuncName(ssaName)
				if !ok && simple != "" {
					entry, ok = s.cache.LookupByFuncName(simple)
				}
			} else if simple != "" {
				entry, ok = s.cache.LookupByFuncName(simple)
			}
		}
	}
	if !ok {
		return nil, nil // null result = hover なし
	}

	return map[string]interface{}{
		"contents": map[string]string{
			"kind":  "markdown",
			"value": entry.Markdown(),
		},
	}, nil
}

var symbolNameRe = regexp.MustCompile(`(?:^|\.)(\w+)$`)
var hoverFuncRe = regexp.MustCompile(`\bfunc\s+(?:\(\w+\s+(\*?(?:\w+\.)?\w+)\)\s+)?(?:\w+\.)?(\w+)\s*[(\[]`)

// extractFuncNames はシンボル名から SSA スタイル名候補と単純名を返す。
// 例: "(*Service).Create" -> "(*Service).Create", "Create"
//
//	"repository.Find"    -> "", "Find"
func extractFuncNames(name string) (string, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ""
	}
	if strings.Contains(name, ").") {
		parts := strings.Split(name, ").")
		if len(parts) == 2 && parts[1] != "" {
			return name, parts[1]
		}
	}
	m := symbolNameRe.FindStringSubmatch(name)
	if len(m) == 2 {
		return "", m[1]
	}
	return "", name
}

func hoverFuncNameFromContents(contents json.RawMessage) string {
	if len(contents) == 0 {
		return ""
	}
	text := hoverContentsText(contents)
	if text == "" {
		return ""
	}
	m := hoverFuncRe.FindStringSubmatch(text)
	if len(m) < 3 {
		return ""
	}
	receiver, name := m[1], m[2]
	if receiver != "" {
		return "(" + receiver + ")." + name
	}
	return name
}

func hoverContentsText(contents json.RawMessage) string {
	var text string
	var markup struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(contents, &markup); err == nil && markup.Kind != "" {
		return markup.Value
	}
	if err := json.Unmarshal(contents, &text); err == nil && text != "" {
		return text
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(contents, &arr); err != nil || len(arr) == 0 {
		return ""
	}
	lines := make([]string, 0, len(arr))
	for _, raw := range arr {
		if err := json.Unmarshal(raw, &text); err == nil && text != "" {
			lines = append(lines, text)
			continue
		}
		var marked struct {
			Language string `json:"language"`
			Value    string `json:"value"`
		}
		if err := json.Unmarshal(raw, &marked); err == nil && marked.Value != "" {
			lines = append(lines, marked.Value)
		}
	}
	return strings.Join(lines, "\n")
}

func (s *server) reply(w io.Writer, id json.RawMessage, result interface{}, rpcErr *rpcError) error {
	var body []byte
	var err error
	if rpcErr != nil {
		body, err = json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"error":   rpcErr,
		})
	} else {
		body, err = json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  result,
		})
	}
	if err != nil {
		return fmt.Errorf("reply marshal: %w", err)
	}
	if err := proxy.WriteMessage(w, body); err != nil {
		return fmt.Errorf("reply write: %w", err)
	}
	return nil
}

// uriToPath は file:// URI をファイルパスに変換する。
func uriToPath(uri string) string {
	const prefix = "file://"
	if !strings.HasPrefix(uri, prefix) {
		return uri
	}
	return percentDecode(uri[len(prefix):])
}

// percentDecode は URL パーセントエンコードを簡易デコードする。
func percentDecode(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '%' && i+2 < len(s) {
			b, err := strconv.ParseUint(s[i+1:i+3], 16, 8)
			if err == nil {
				sb.WriteByte(byte(b))
				i += 3
				continue
			}
		}
		sb.WriteByte(s[i])
		i++
	}
	return sb.String()
}
