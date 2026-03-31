package analyzer

import (
	"fmt"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

// Analyzer は関数が返しうる Sentinel Error を報告する。
var Analyzer = &analysis.Analyzer{
	Name:      "sentinelfind",
	Doc:       "reports sentinel errors a function may return",
	Run:       run,
	Requires:  []*analysis.Analyzer{buildssa.Analyzer},
	FactTypes: []analysis.Fact{(*SentinelFact)(nil)},
}

func run(pass *analysis.Pass) (interface{}, error) {
	ssaResult := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)

	for _, fn := range ssaResult.SrcFuncs {
		if fn.Blocks == nil {
			continue
		}

		// エラーを返す関数のみ対象
		sig := fn.Signature
		if !returnsError(sig) {
			continue
		}

		sentinels := collectSentinels(fn, pass)

		if len(sentinels) == 0 {
			continue
		}

		// SentinelFact をエクスポート
		fact := &SentinelFact{Errors: sentinels}
		if obj := findFuncObject(pass, fn); obj != nil {
			pass.ExportObjectFact(obj, fact)
		}

		// 診断レポート
		pos := funcPos(fn)
		names := make([]string, len(sentinels))
		for i, s := range sentinels {
			names[i] = s.String()
		}
		sort.Strings(names)
		pass.Reportf(pos, "%s returns sentinels: %s", fn.Name(), strings.Join(names, ", "))
	}

	return nil, nil
}

// collectSentinels は関数内の全 Return 命令から Sentinel Error を収集する。
func collectSentinels(fn *ssa.Function, pass *analysis.Pass) []SentinelInfo {
	ctx := &traceCtx{
		visited: make(map[ssa.Value]bool),
		facts: func(obj types.Object, fact *SentinelFact) bool {
			if obj == nil {
				return false
			}
			return pass.ImportObjectFact(obj, fact)
		},
	}

	var result []SentinelInfo
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			ret, ok := instr.(*ssa.Return)
			if !ok {
				continue
			}
			for _, v := range ret.Results {
				if !isErrorType(v.Type()) {
					continue
				}
				ctx.visited = make(map[ssa.Value]bool) // reset per return value
				result = appendUniq(result, traceValue(v, 0, ctx))
			}
		}
	}
	return result
}

// returnsError はシグネチャが error 型を返すかを判定する。
func returnsError(sig *types.Signature) bool {
	results := sig.Results()
	errorIface := types.Universe.Lookup("error").Type()
	for i := 0; i < results.Len(); i++ {
		if types.Identical(results.At(i).Type(), errorIface) {
			return true
		}
	}
	return false
}

// findFuncObject は SSA 関数に対応する types.Object を返す。
func findFuncObject(pass *analysis.Pass, fn *ssa.Function) types.Object {
	if fn.Object() != nil {
		return fn.Object()
	}
	return nil
}

// funcPos は関数の位置を返す。
func funcPos(fn *ssa.Function) token.Pos {
	if fn.Pos().IsValid() {
		return fn.Pos()
	}
	if fn.Syntax() != nil {
		return fn.Syntax().Pos()
	}
	return token.NoPos
}

// sentinel の String 形式を生成するヘルパー（テスト用）
func sentinelString(s SentinelInfo) string {
	return fmt.Sprintf("%s.%s", pkgName(s.PkgPath), s.Name)
}

func pkgName(pkgPath string) string {
	for i := len(pkgPath) - 1; i >= 0; i-- {
		if pkgPath[i] == '/' {
			return pkgPath[i+1:]
		}
	}
	return pkgPath
}
