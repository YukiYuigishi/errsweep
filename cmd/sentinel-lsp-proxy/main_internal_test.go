package main

import "testing"

func TestShouldRefreshCache(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"didSave", `{"jsonrpc":"2.0","method":"textDocument/didSave","params":{}}`, true},
		{"didChangeWatchedFiles", `{"jsonrpc":"2.0","method":"workspace/didChangeWatchedFiles","params":{}}`, true},
		{"hover", `{"jsonrpc":"2.0","id":1,"method":"textDocument/hover","params":{}}`, false},
		{"response", `{"jsonrpc":"2.0","id":1,"result":{}}`, false},
		{"invalid", `{"jsonrpc":`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRefreshCache([]byte(tc.raw)); got != tc.want {
				t.Fatalf("shouldRefreshCache(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}
