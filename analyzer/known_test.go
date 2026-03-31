package analyzer_test

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
