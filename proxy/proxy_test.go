package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// hoverResponse は textDocument/hover の典型的なレスポンス。
func hoverResponse(id int, file string, line int, contents string) []byte {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"contents": map[string]interface{}{
				"kind":  "markdown",
				"value": contents,
			},
			"range": map[string]interface{}{
				"start": map[string]interface{}{"line": line - 1, "character": 5},
				"end":   map[string]interface{}{"line": line - 1, "character": 12},
			},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

// hoverRequest は textDocument/hover リクエスト。
func hoverRequest(id int, file string, line, char int) []byte {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "textDocument/hover",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file://" + file},
			"position":     map[string]interface{}{"line": line - 1, "character": char},
		},
	}
	b, _ := json.Marshal(req)
	return b
}

func frameMessage(body []byte) string {
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}

// TestProxy_NonHover はホバー以外のメッセージが変更されずに通過することを確認する。
func TestProxy_NonHover(t *testing.T) {
	cache := make(Cache)
	p := NewProxy(cache)

	msg := []byte(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{}}}`)
	var out bytes.Buffer
	if err := p.processServerMessage(msg, &out); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	want := frameMessage(msg)
	if got != want {
		t.Errorf("non-hover message should pass through unchanged\ngot:  %q\nwant: %q", got, want)
	}
}

// TestProxy_HoverNoCache はキャッシュに対応エントリがない場合に元のレスポンスが変更されないことを確認する。
func TestProxy_HoverNoCache(t *testing.T) {
	cache := make(Cache)
	p := NewProxy(cache)

	// hover リクエストを先に登録
	reqBody := hoverRequest(1, "/workspace/foo.go", 10, 5)
	if err := p.trackRequest(reqBody); err != nil {
		t.Fatal(err)
	}

	respBody := hoverResponse(1, "/workspace/foo.go", 10, "```go\nfunc Foo()\n```")
	var out bytes.Buffer
	if err := p.processServerMessage(respBody, &out); err != nil {
		t.Fatal(err)
	}

	// キャッシュなし → 元のレスポンスがそのまま転送される
	outMsg, err := readMessage(bufio.NewReader(&out))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(outMsg, respBody) {
		t.Errorf("expected original response\ngot:  %s\nwant: %s", outMsg, respBody)
	}
}

// TestProxy_HoverWithCache はキャッシュヒット時に Sentinel 情報が Markdown に追記されることを確認する。
func TestProxy_HoverWithCache(t *testing.T) {
	cache := Cache{
		cacheKey{file: "/workspace/repository/user.go", line: 8}: {
			FuncName:  "FindByID",
			Sentinels: []string{"repository.ErrNotFound"},
		},
	}
	p := NewProxy(cache)

	reqBody := hoverRequest(2, "/workspace/repository/user.go", 8, 5)
	if err := p.trackRequest(reqBody); err != nil {
		t.Fatal(err)
	}

	respBody := hoverResponse(2, "/workspace/repository/user.go", 8, "```go\nfunc FindByID(id int) (string, error)\n```")
	var out bytes.Buffer
	if err := p.processServerMessage(respBody, &out); err != nil {
		t.Fatal(err)
	}

	outMsg, err := readMessage(bufio.NewReader(&out))
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(outMsg, &result); err != nil {
		t.Fatal(err)
	}
	var hoverResult struct {
		Contents struct {
			Value string `json:"value"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(result["result"], &hoverResult); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(hoverResult.Contents.Value, "repository.ErrNotFound") {
		t.Errorf("hover response should contain sentinel info\ngot: %s", hoverResult.Contents.Value)
	}
	if !strings.Contains(hoverResult.Contents.Value, "Possible Sentinel Errors") {
		t.Errorf("hover response should contain section header\ngot: %s", hoverResult.Contents.Value)
	}
}

// TestProxy_HoverStringContents は contents が文字列形式の hover にも対応することを確認する。
func TestProxy_HoverStringContents(t *testing.T) {
	cache := Cache{
		cacheKey{file: "/src/pkg.go", line: 3}: {
			FuncName:  "Do",
			Sentinels: []string{"pkg.ErrFoo"},
		},
	}
	p := NewProxy(cache)

	reqBody := hoverRequest(3, "/src/pkg.go", 3, 5)
	if err := p.trackRequest(reqBody); err != nil {
		t.Fatal(err)
	}

	// contents が MarkupContent ではなく文字列のケース
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"result": map[string]interface{}{
			"contents": "func Do() error",
		},
	}
	respBody, _ := json.Marshal(resp)

	var out bytes.Buffer
	if err := p.processServerMessage(respBody, &out); err != nil {
		t.Fatal(err)
	}

	outMsg, err := readMessage(bufio.NewReader(&out))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(outMsg), "pkg.ErrFoo") {
		t.Errorf("string-contents hover should also get sentinel info\ngot: %s", outMsg)
	}
}
