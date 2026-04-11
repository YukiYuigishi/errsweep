package analyzer

import (
	"go/token"
	"testing"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// TestKnownKeys は known.go に登録するキーの RelString 形式を確認する。
// 対象パッケージの全関数を走査し、名前が一致するものの RelString を出力する。
func TestKnownKeys(t *testing.T) {
	interest := map[string]bool{
		"ReadString": true, "ReadByte": true, "ReadRune": true,
		"ReadLine": true, "ReadBytes": true,
		"Read": true, "ReadAt": true, "ReadFile": true,
		"ReadFull": true, "Copy": true, "ReadAll": true,
		"Scan": true, "Next": true, "Err": true, "QueryRowContext": true,
	}
	targetPkgs := map[string]bool{
		"bufio": true, "os": true, "io": true, "database/sql": true,
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedTypesSizes,
		Fset: token.NewFileSet(),
	}
	pkgs, err := packages.Load(cfg, "bufio", "os", "io", "database/sql")
	if err != nil {
		t.Fatal(err)
	}
	prog, _ := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()

	seen := map[string]bool{}
	for fn := range ssautil.AllFunctions(prog) {
		if fn.Package() == nil || !targetPkgs[fn.Package().Pkg.Path()] {
			continue
		}
		if !interest[fn.Name()] {
			continue
		}
		key := fn.RelString(nil)
		if !seen[key] {
			seen[key] = true
			t.Logf("%q", key)
		}
	}
}

func TestKnownErrorMapCoverage(t *testing.T) {
	cases := []struct {
		key       string
		wantFirst string
	}{
		{"(*os.File).Read", "io.EOF"},
		{"os.ReadFile", "fs.ErrNotExist"},
		{"(*database/sql.Row).Scan", "sql.ErrNoRows"},
		{"(*database/sql.NullString).Scan", "sql.ErrNoRows"},
		{"(*net/http.Request).Cookie", "http.ErrNoCookie"},
		{"(*net/http.Request).FormFile", "http.ErrMissingFile"},
		{"(*net/http.Server).Serve", "http.ErrServerClosed"},
		{"(*net.TCPListener).Accept", "net.ErrClosed"},
	}
	for _, tc := range cases {
		v, ok := knownErrorMap[tc.key]
		if !ok {
			t.Fatalf("knownErrorMap[%q] missing", tc.key)
		}
		if len(v) == 0 {
			t.Fatalf("knownErrorMap[%q] is empty", tc.key)
		}
		if got := v[0].String(); got != tc.wantFirst {
			t.Fatalf("knownErrorMap[%q][0] = %q, want %q", tc.key, got, tc.wantFirst)
		}
	}
}
