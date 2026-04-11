package main

import (
	"sort"
	"testing"
	"time"
)

func TestParseRefreshRequest(t *testing.T) {
	root := "/workspace"
	cases := []struct {
		name     string
		raw      string
		wantOK   bool
		wantFull bool
		wantDirs []string
	}{
		{
			name:     "didSave go triggers partial",
			raw:      `{"jsonrpc":"2.0","method":"textDocument/didSave","params":{"textDocument":{"uri":"file:///workspace/pkg/a.go"}}}`,
			wantOK:   true,
			wantDirs: []string{"/workspace/pkg"},
		},
		{
			name:   "didSave go outside workspace is ignored",
			raw:    `{"jsonrpc":"2.0","method":"textDocument/didSave","params":{"textDocument":{"uri":"file:///tmp/a.go"}}}`,
			wantOK: false,
		},
		{
			name:   "didSave markdown is ignored",
			raw:    `{"jsonrpc":"2.0","method":"textDocument/didSave","params":{"textDocument":{"uri":"file:///workspace/a.md"}}}`,
			wantOK: false,
		},
		{
			name:     "didChangeWatchedFiles go triggers partial",
			raw:      `{"jsonrpc":"2.0","method":"workspace/didChangeWatchedFiles","params":{"changes":[{"uri":"file:///workspace/pkg/a.go"},{"uri":"file:///workspace/other/b.go"}]}}`,
			wantOK:   true,
			wantDirs: []string{"/workspace/other", "/workspace/pkg"},
		},
		{
			name:     "didChangeWatchedFiles go.mod forces full",
			raw:      `{"jsonrpc":"2.0","method":"workspace/didChangeWatchedFiles","params":{"changes":[{"uri":"file:///workspace/go.mod"}]}}`,
			wantOK:   true,
			wantFull: true,
		},
		{
			name:     "mixed go.sum and .go forces full",
			raw:      `{"jsonrpc":"2.0","method":"workspace/didChangeWatchedFiles","params":{"changes":[{"uri":"file:///workspace/go.sum"},{"uri":"file:///workspace/pkg/a.go"}]}}`,
			wantOK:   true,
			wantFull: true,
			wantDirs: []string{"/workspace/pkg"},
		},
		{
			name:     "didRenameFiles go",
			raw:      `{"jsonrpc":"2.0","method":"workspace/didRenameFiles","params":{"files":[{"oldUri":"file:///workspace/pkg/a.tmp","newUri":"file:///workspace/pkg/a.go"}]}}`,
			wantOK:   true,
			wantDirs: []string{"/workspace/pkg"},
		},
		{
			name:     "didCreateFiles go.mod triggers full",
			raw:      `{"jsonrpc":"2.0","method":"workspace/didCreateFiles","params":{"files":[{"uri":"file:///workspace/go.mod"}]}}`,
			wantOK:   true,
			wantFull: true,
		},
		{
			name:   "didDeleteFiles png ignored",
			raw:    `{"jsonrpc":"2.0","method":"workspace/didDeleteFiles","params":{"files":[{"uri":"file:///workspace/a.png"}]}}`,
			wantOK: false,
		},
		{
			name:   "hover is not a refresh",
			raw:    `{"jsonrpc":"2.0","id":1,"method":"textDocument/hover","params":{}}`,
			wantOK: false,
		},
		{
			name:   "response is not a refresh",
			raw:    `{"jsonrpc":"2.0","id":1,"result":{}}`,
			wantOK: false,
		},
		{
			name:   "invalid json",
			raw:    `{"jsonrpc":`,
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, ok := parseRefreshRequest([]byte(tc.raw), root)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v want=%v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if req.full != tc.wantFull {
				t.Fatalf("full=%v want=%v", req.full, tc.wantFull)
			}
			gotDirs := append([]string(nil), req.pkgDirs...)
			sort.Strings(gotDirs)
			want := append([]string(nil), tc.wantDirs...)
			sort.Strings(want)
			if len(gotDirs) != len(want) {
				t.Fatalf("pkgDirs=%v want=%v", gotDirs, want)
			}
			for i := range gotDirs {
				if gotDirs[i] != want[i] {
					t.Fatalf("pkgDirs=%v want=%v", gotDirs, want)
				}
			}
		})
	}
}

func TestPendingRefresh_AddDrainMerges(t *testing.T) {
	var p pendingRefresh
	p.add(refreshRequest{pkgDirs: []string{"/ws/a", "/ws/b"}})
	p.add(refreshRequest{pkgDirs: []string{"/ws/b", "/ws/c"}})
	full, dirs := p.drain()
	if full {
		t.Fatal("expected full=false")
	}
	sort.Strings(dirs)
	if len(dirs) != 3 || dirs[0] != "/ws/a" || dirs[1] != "/ws/b" || dirs[2] != "/ws/c" {
		t.Fatalf("unexpected dirs: %v", dirs)
	}
	// drain 後は空であるべき
	full, dirs = p.drain()
	if full || len(dirs) != 0 {
		t.Fatalf("expected empty after drain, got full=%v dirs=%v", full, dirs)
	}
}

func TestPendingRefresh_FullOverridesPartial(t *testing.T) {
	var p pendingRefresh
	p.add(refreshRequest{pkgDirs: []string{"/ws/a"}})
	p.add(refreshRequest{full: true})
	full, dirs := p.drain()
	if !full {
		t.Fatal("expected full=true once a full request is added")
	}
	// 収集された pkgDirs は保持されても無視されても良い。呼び出し側は full を優先する。
	_ = dirs
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
