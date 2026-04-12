package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/YukiYuigishi/errsweep/proxy"
)

// frame は LSP Content-Length フレームを作る。
func frame(body []byte) []byte {
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	return append([]byte(header), body...)
}

// readResponse は LSP フレームから JSON 本文を読む。
func readResponse(t *testing.T, buf *bytes.Buffer) map[string]json.RawMessage {
	t.Helper()
	body, err := proxy.ReadMessage(bufio.NewReader(buf))
	if err != nil {
		t.Fatalf("readResponse: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("readResponse unmarshal: %v", err)
	}
	return m
}

// runOne はメッセージ1件を処理して out を返す。
// bytes.NewReader が EOF を返した時点で run が終了するのを利用し、同期的に実行する。
func runOne(t *testing.T, s *server, reqBody []byte) *bytes.Buffer {
	t.Helper()
	var out bytes.Buffer
	s.run(bytes.NewReader(frame(reqBody)), &out)
	return &out
}

func TestServer_Initialize(t *testing.T) {
	s := &server{cache: proxy.NewCache()}

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]interface{}{},
	}
	reqBody, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal initialize request: %v", err)
	}

	out := runOne(t, s, reqBody)
	resp := readResponse(t, out)

	if _, ok := resp["result"]; !ok {
		t.Fatalf("no result in response: %s", out.String())
	}

	var result struct {
		Capabilities struct {
			HoverProvider bool `json:"hoverProvider"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatal(err)
	}
	if !result.Capabilities.HoverProvider {
		t.Error("hoverProvider should be true")
	}
}

func TestServer_HoverHit(t *testing.T) {
	cache, err := proxy.ParseErrsweepJSON([]byte(`{
		"pkg": {
			"errsweep": [{
				"posn": "/src/foo.go:5:1",
				"message": "DoSomething returns sentinels: pkg.ErrFoo"
			}]
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}

	s := &server{cache: cache}

	hoverReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "textDocument/hover",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file:///src/foo.go"},
			"position":     map[string]interface{}{"line": 4, "character": 0}, // 0-indexed → line 5
		},
	}
	reqBody, err := json.Marshal(hoverReq)
	if err != nil {
		t.Fatalf("marshal hover request: %v", err)
	}

	out := runOne(t, s, reqBody)
	resp := readResponse(t, out)

	if _, ok := resp["result"]; !ok {
		t.Fatalf("no result: %s", out.String())
	}

	var result struct {
		Contents struct {
			Kind  string `json:"kind"`
			Value string `json:"value"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatal(err)
	}
	if result.Contents.Kind != "markdown" {
		t.Errorf("kind = %q, want markdown", result.Contents.Kind)
	}
	if !strings.Contains(result.Contents.Value, "pkg.ErrFoo") {
		t.Errorf("hover value should contain pkg.ErrFoo:\n%s", result.Contents.Value)
	}
}

func TestServer_HoverMiss(t *testing.T) {
	s := &server{cache: proxy.NewCache()}

	hoverReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "textDocument/hover",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file:///src/bar.go"},
			"position":     map[string]interface{}{"line": 0, "character": 0},
		},
	}
	reqBody, err := json.Marshal(hoverReq)
	if err != nil {
		t.Fatalf("marshal hover miss request: %v", err)
	}

	out := runOne(t, s, reqBody)
	resp := readResponse(t, out)

	// キャッシュミス → result は null
	if string(resp["result"]) != "null" {
		t.Errorf("expected null result, got %s", resp["result"])
	}
}

func TestServer_HoverFallbackBySymbolName(t *testing.T) {
	cache, err := proxy.ParseErrsweepJSON([]byte(`{
		"pkg": {
			"errsweep": [{
				"posn": "/src/repo.go:30:1",
				"message": "(*Service).Create returns sentinels: pkg.ErrCreate"
			}]
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	s := &server{cache: cache}

	hoverReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "textDocument/hover",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file:///src/callsite.go"},
			"position":     map[string]interface{}{"line": 1, "character": 0},
			"context": map[string]interface{}{
				"symbol": map[string]interface{}{"name": "(*Service).Create"},
			},
		},
	}
	reqBody, err := json.Marshal(hoverReq)
	if err != nil {
		t.Fatalf("marshal hover request: %v", err)
	}

	out := runOne(t, s, reqBody)
	resp := readResponse(t, out)
	if string(resp["result"]) == "null" {
		t.Fatalf("expected fallback hover result, got null: %s", out.String())
	}
	var result struct {
		Contents struct {
			Value string `json:"value"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Contents.Value, "pkg.ErrCreate") {
		t.Fatalf("expected pkg.ErrCreate in hover contents:\n%s", result.Contents.Value)
	}
}

func TestExtractFuncNames(t *testing.T) {
	cases := []struct {
		in       string
		wantSSA  string
		wantName string
	}{
		{"(*Service).Create", "(*Service).Create", "Create"},
		{"repository.Find", "", "Find"},
		{"Find", "", "Find"},
		{"", "", ""},
	}
	for _, tc := range cases {
		ssa, name := extractFuncNames(tc.in)
		if ssa != tc.wantSSA || name != tc.wantName {
			t.Fatalf("extractFuncNames(%q) = (%q, %q), want (%q, %q)", tc.in, ssa, name, tc.wantSSA, tc.wantName)
		}
	}
}

func TestHoverFuncNameFromContents(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"markup", `{"kind":"markdown","value":"func repository.FindXXX(id string) error"}`, "FindXXX"},
		{"marked-array", `["package b", {"language":"go","value":"func FindXXX(id string) error"}]`, "FindXXX"},
		{"method", `{"kind":"markdown","value":"func (r *Repo) FindXXX(id string) error"}`, "(*Repo).FindXXX"},
		{"none", `"not a func signature"`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hoverFuncNameFromContents(json.RawMessage(tc.raw))
			if got != tc.want {
				t.Fatalf("hoverFuncNameFromContents(%s) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestServer_UnknownMethod(t *testing.T) {
	s := &server{cache: proxy.NewCache()}

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "textDocument/completion",
		"params":  map[string]interface{}{},
	}
	reqBody, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal unknown request: %v", err)
	}

	out := runOne(t, s, reqBody)
	resp := readResponse(t, out)

	if _, ok := resp["error"]; !ok {
		t.Error("expected error for unknown method")
	}
}
