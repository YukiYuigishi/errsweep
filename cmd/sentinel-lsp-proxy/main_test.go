package main_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YukiYuigishi/errsweep/proxy"
)

var (
	proxyBin       string
	sentinelfindBin string
	dummyGoplsBin  string
)

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "sentinel-lsp-proxy-e2e-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)

	proxyBin = filepath.Join(tmp, "sentinel-lsp-proxy")
	sentinelfindBin = filepath.Join(tmp, "sentinelfind")
	dummyGoplsBin = filepath.Join(tmp, "dummy-gopls")

	for _, target := range []struct {
		bin string
		pkg string
	}{
		{proxyBin, "github.com/YukiYuigishi/errsweep/cmd/sentinel-lsp-proxy"},
		{sentinelfindBin, "github.com/YukiYuigishi/errsweep/cmd/sentinelfind"},
		{dummyGoplsBin, "github.com/YukiYuigishi/errsweep/cmd/sentinel-lsp-proxy/testdata/dummy-gopls"},
	} {
		if out, err := exec.Command("go", "build", "-o", target.bin, target.pkg).CombinedOutput(); err != nil {
			panic("build failed (" + target.pkg + "): " + string(out))
		}
	}

	os.Exit(m.Run())
}

// exampleWorkspace はモジュールルートからの相対パスで example ディレクトリの絶対パスを返す。
func exampleWorkspace(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../../example")
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// lspFrame は LSP の Content-Length フレームを組み立てる。
func lspFrame(t *testing.T, msg interface{}) []byte {
	t.Helper()
	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := proxy.WriteMessage(&buf, body); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestE2E_HoverSentinelAppended は sentinel-lsp-proxy がキャッシュヒット時に
// Sentinel 情報を hover レスポンスへ追記することを確認する。
func TestE2E_HoverSentinelAppended(t *testing.T) {
	ws := exampleWorkspace(t)

	// sentinel-lsp-proxy を起動（gopls の代わりに dummy-gopls を使う）
	cmd := exec.Command(proxyBin,
		"--gopls="+dummyGoplsBin,
		"--sentinelfind="+sentinelfindBin,
		"--workspace="+ws,
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stdin.Close()
		cmd.Wait()
	})

	// hover リクエストを送信（usecase/user.go の GetUser: line 9, 0-indexed=8）
	hoverReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/hover",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file://" + ws + "/usecase/user.go",
			},
			"position": map[string]interface{}{
				"line":      8, // 0-indexed
				"character": 5,
			},
		},
	}
	if _, err := stdin.Write(lspFrame(t, hoverReq)); err != nil {
		t.Fatal(err)
	}

	// レスポンスを読む
	r := bufio.NewReader(stdout)
	raw, err := proxy.ReadMessage(r)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	var resp struct {
		Result struct {
			Contents struct {
				Kind  string `json:"kind"`
				Value string `json:"value"`
			} `json:"contents"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal response: %v\nbody: %s", err, raw)
	}

	value := resp.Result.Contents.Value
	if !strings.Contains(value, "Possible Sentinel Errors") {
		t.Errorf("hover response missing sentinel section\ngot:\n%s", value)
	}
	if !strings.Contains(value, "ErrNotFound") {
		t.Errorf("hover response missing ErrNotFound\ngot:\n%s", value)
	}
}

// TestE2E_HoverNoEntry はキャッシュに対応エントリがない行への hover が
// 元のレスポンスを変更せずに返すことを確認する。
func TestE2E_HoverNoEntry(t *testing.T) {
	ws := exampleWorkspace(t)

	cmd := exec.Command(proxyBin,
		"--gopls="+dummyGoplsBin,
		"--sentinelfind="+sentinelfindBin,
		"--workspace="+ws,
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stdin.Close()
		cmd.Wait()
	})

	// sentinel のない行（line 1: package 宣言）に hover
	hoverReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/hover",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file://" + ws + "/usecase/user.go",
			},
			"position": map[string]interface{}{
				"line":      0, // line 1 (package)
				"character": 5,
			},
		},
	}
	if _, err := stdin.Write(lspFrame(t, hoverReq)); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(stdout)
	raw, err := proxy.ReadMessage(r)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	if strings.Contains(string(raw), "Possible Sentinel Errors") {
		t.Errorf("hover on non-sentinel line should not have sentinel section\ngot:\n%s", raw)
	}
}
