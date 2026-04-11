package main

import (
	"testing"
	"time"
)

func TestShouldRefreshCache(t *testing.T) {
	root := "/workspace"
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"didSave go in workspace", `{"jsonrpc":"2.0","method":"textDocument/didSave","params":{"textDocument":{"uri":"file:///workspace/a.go"}}}`, true},
		{"didSave go outside workspace", `{"jsonrpc":"2.0","method":"textDocument/didSave","params":{"textDocument":{"uri":"file:///tmp/a.go"}}}`, false},
		{"didSave markdown", `{"jsonrpc":"2.0","method":"textDocument/didSave","params":{"textDocument":{"uri":"file:///workspace/a.md"}}}`, false},
		{"didChangeWatchedFiles go", `{"jsonrpc":"2.0","method":"workspace/didChangeWatchedFiles","params":{"changes":[{"uri":"file:///workspace/a.go"}]}}`, true},
		{"didChangeWatchedFiles txt", `{"jsonrpc":"2.0","method":"workspace/didChangeWatchedFiles","params":{"changes":[{"uri":"file:///workspace/a.txt"}]}}`, false},
		{"didRenameFiles go", `{"jsonrpc":"2.0","method":"workspace/didRenameFiles","params":{"files":[{"oldUri":"file:///workspace/a.tmp","newUri":"file:///workspace/a.go"}]}}`, true},
		{"didCreateFiles gomod", `{"jsonrpc":"2.0","method":"workspace/didCreateFiles","params":{"files":[{"uri":"file:///workspace/go.mod"}]}}`, true},
		{"didDeleteFiles png", `{"jsonrpc":"2.0","method":"workspace/didDeleteFiles","params":{"files":[{"uri":"file:///workspace/a.png"}]}}`, false},
		{"hover", `{"jsonrpc":"2.0","id":1,"method":"textDocument/hover","params":{}}`, false},
		{"response", `{"jsonrpc":"2.0","id":1,"result":{}}`, false},
		{"invalid", `{"jsonrpc":`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRefreshCache([]byte(tc.raw), root); got != tc.want {
				t.Fatalf("shouldRefreshCache(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestShouldRefreshByURI(t *testing.T) {
	root := "/workspace"
	cases := []struct {
		uri  string
		want bool
	}{
		{"file:///workspace/a.go", true},
		{"file:///workspace/GO.MOD", true},
		{"file:///workspace/go.sum", true},
		{"file:///workspace/go.work", true},
		{"file:///workspace/a.md", false},
		{"file:///tmp/a.go", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := shouldRefreshByURI(tc.uri, root); got != tc.want {
			t.Fatalf("shouldRefreshByURI(%q) = %v, want %v", tc.uri, got, tc.want)
		}
	}
}

func TestURIToWorkspacePath(t *testing.T) {
	root := "/workspace"
	if _, ok := uriToWorkspacePath("file:///workspace/a.go", root); !ok {
		t.Fatal("expected in-workspace URI to be accepted")
	}
	if _, ok := uriToWorkspacePath("file:///tmp/a.go", root); ok {
		t.Fatal("expected out-of-workspace URI to be rejected")
	}
	if _, ok := uriToWorkspacePath("not-a-file-uri", root); ok {
		t.Fatal("expected non-file URI to be rejected")
	}
}

func TestShouldRunRefresh(t *testing.T) {
	base := time.Unix(100, 0)
	cases := []struct {
		name        string
		now         time.Time
		last        time.Time
		minInterval time.Duration
		want        bool
	}{
		{"first run", base, time.Time{}, 2 * time.Second, true},
		{"disabled interval", base, base, 0, true},
		{"under interval", base.Add(1500 * time.Millisecond), base, 2 * time.Second, false},
		{"equal interval", base.Add(2 * time.Second), base, 2 * time.Second, true},
		{"over interval", base.Add(3 * time.Second), base, 2 * time.Second, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRunRefresh(tc.now, tc.last, tc.minInterval); got != tc.want {
				t.Fatalf("shouldRunRefresh(...) = %v, want %v", got, tc.want)
			}
		})
	}
}
