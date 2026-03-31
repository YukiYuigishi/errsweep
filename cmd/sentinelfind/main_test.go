package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// binPath はテスト用にビルドしたバイナリのパス。
var binPath string

func TestMain(m *testing.M) {
	bin, err := os.CreateTemp("", "sentinelfind-*")
	if err != nil {
		panic(err)
	}
	bin.Close()
	binPath = bin.Name()

	build := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := build.CombinedOutput(); err != nil {
		panic("build failed: " + string(out))
	}

	code := m.Run()
	os.Remove(binPath)
	os.Exit(code)
}

// samplePkg はテスト用フィクスチャのパス。
func samplePkg(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("testdata/src/sample")
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func run(t *testing.T, args ...string) (stdout string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	out, err := cmd.CombinedOutput()
	stdout = string(out)
	if err == nil {
		return stdout, 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return stdout, ee.ExitCode()
	}
	t.Fatalf("unexpected error: %v", err)
	return
}

// TestFlag_Default は何もフラグを付けない場合に診断が出力され exit 3 になることを確認する。
func TestFlag_Default(t *testing.T) {
	out, code := run(t, samplePkg(t))
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
	if !strings.Contains(out, "Find returns sentinels: sample.ErrNotFound") {
		t.Errorf("expected diagnostic not found in output:\n%s", out)
	}
}

// TestFlag_JSON は -json フラグが有効な JSON を出力し、診断情報を含むことを確認する。
func TestFlag_JSON(t *testing.T) {
	out, _ := run(t, "-json", samplePkg(t))

	var result map[string]map[string][]struct {
		Posn    string `json:"posn"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}

	found := false
	for _, checks := range result {
		for _, diags := range checks {
			for _, d := range diags {
				if strings.Contains(d.Message, "Find returns sentinels: sample.ErrNotFound") {
					found = true
				}
			}
		}
	}
	if !found {
		t.Errorf("expected diagnostic not found in JSON output:\n%s", out)
	}
}

// TestFlag_JSON_Fields は -json 出力に posn / end / message フィールドが含まれることを確認する。
func TestFlag_JSON_Fields(t *testing.T) {
	out, _ := run(t, "-json", samplePkg(t))

	for _, field := range []string{`"posn"`, `"end"`, `"message"`} {
		if !strings.Contains(out, field) {
			t.Errorf("JSON output missing field %s:\n%s", field, out)
		}
	}
}

// TestFlag_Context は -c N フラグがソース行のコンテキストを出力に含めることを確認する。
func TestFlag_Context(t *testing.T) {
	outNo, _ := run(t, samplePkg(t))
	outWith, _ := run(t, "-c", "2", samplePkg(t))

	if len(outWith) <= len(outNo) {
		t.Errorf("-c 2 output should be longer than default output\ndefault:\n%s\nwith -c 2:\n%s", outNo, outWith)
	}
	// 診断行の前後 2 行が含まれることを確認
	if !strings.Contains(outWith, "ErrNotFound") {
		t.Errorf("-c 2 output should contain context lines with ErrNotFound:\n%s", outWith)
	}
}

// TestFlag_Context_Zero は -c 0 がコンテキストなしで通常と同じ出力になることを確認する。
func TestFlag_Context_Zero(t *testing.T) {
	outDefault, _ := run(t, samplePkg(t))
	outC0, _ := run(t, "-c", "0", samplePkg(t))

	// 診断メッセージ行は同じ
	defaultLines := diagnosticLines(outDefault)
	c0Lines := diagnosticLines(outC0)
	if len(defaultLines) != len(c0Lines) {
		t.Errorf("diagnostic line count: default=%d, -c 0=%d", len(defaultLines), len(c0Lines))
	}
}

// TestFlag_Test_False は -test=false がテストファイル内の関数を解析しないことを確認する。
func TestFlag_Test_False(t *testing.T) {
	out, _ := run(t, "-test=false", samplePkg(t))
	if strings.Contains(out, "ErrTestOnly") {
		t.Errorf("-test=false should not report diagnostics from _test.go files:\n%s", out)
	}
}

// TestFlag_Test_True は -test=true（デフォルト）がテストファイル内の関数も解析することを確認する。
func TestFlag_Test_True(t *testing.T) {
	out, _ := run(t, "-test=true", samplePkg(t))
	if !strings.Contains(out, "ErrTestOnly") {
		t.Errorf("-test=true should report diagnostics from _test.go files:\n%s", out)
	}
}

// TestFlag_Flags は -flags が JSON 配列を出力することを確認する。
func TestFlag_Flags(t *testing.T) {
	out, _ := run(t, "-flags")

	var flags []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &flags); err != nil {
		t.Fatalf("-flags output is not a valid JSON array: %v\n%s", err, out)
	}
	if len(flags) == 0 {
		t.Error("-flags output is empty JSON array")
	}
	// 各エントリに Name と Usage が含まれることを確認
	for _, f := range flags {
		if _, ok := f["Name"]; !ok {
			t.Errorf("flag entry missing 'Name' field: %v", f)
		}
		if _, ok := f["Usage"]; !ok {
			t.Errorf("flag entry missing 'Usage' field: %v", f)
		}
	}
}

// TestFlag_Version は -V=full がバージョン文字列を出力することを確認する。
func TestFlag_Version(t *testing.T) {
	out, code := run(t, "-V=full")
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "sentinelfind") {
		t.Errorf("-V=full output should contain binary name:\n%s", out)
	}
}

// TestFlag_ExitCode_NoFindings は診断がないパッケージで exit 0 になることを確認する。
func TestFlag_ExitCode_NoFindings(t *testing.T) {
	abs, err := filepath.Abs("testdata/src/noop")
	if err != nil {
		t.Fatal(err)
	}
	_, code := run(t, abs)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (no diagnostics)", code)
	}
}

// diagnosticLines は出力から診断行（"returns sentinels:" を含む行）を抽出する。
func diagnosticLines(out string) []string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "returns sentinels:") {
			lines = append(lines, line)
		}
	}
	return lines
}
