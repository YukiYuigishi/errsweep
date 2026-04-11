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
	ssaResult, ok := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	if !ok || ssaResult == nil {
		return nil, fmt.Errorf("run: missing buildssa result")
	}

	// var f FuncType = ConcreteFunc パターンのグローバル関数変数マップを事前構築（DI 解決用）
	globalFuncs := BuildGlobalFuncMap(ssaResult.SrcFuncs, ssaResult.Pkg)

	// RTA で実行時に到達しうる具象型を収集し、interface 実装候補の過剰推論を抑える。
	runtimeTypes := collectRuntimeConcreteTypes(ssaResult.SrcFuncs, ssaResult.Pkg)

	// var _ Iface = (*Concrete)(nil) の compile-time assertion から
	// インターフェース → 具象型のマップを構築（interface DI 解決用）
	ifaceImpls := buildInterfaceImpls(pass, runtimeTypes)

	for _, fn := range ssaResult.SrcFuncs {
		if fn.Blocks == nil {
			continue
		}

		// エラーを返す関数のみ対象
		sig := fn.Signature
		if !returnsError(sig) {
			continue
		}

		sentinels, breakdown := collectSentinels(fn, pass, globalFuncs, ifaceImpls)

		// どちらか片方でもあれば報告する（invoke 経由で具象ごとの
		// 内訳だけが存在するケースでも必ず診断を出したい）
		if len(sentinels) == 0 && len(breakdown.byConcrete) == 0 {
			continue
		}

		// 表示・Fact 双方で順序を安定させるためソート
		sort.Slice(sentinels, func(i, j int) bool {
			return sentinels[i].String() < sentinels[j].String()
		})

		// SentinelFact は union で保存（後方互換）
		if len(sentinels) > 0 {
			fact := &SentinelFact{Errors: sentinels}
			if obj := findFuncObject(pass, fn); obj != nil {
				pass.ExportObjectFact(obj, fact)
			}
		}

		pos := funcPos(fn)

		// 合算ライン: 関数が返しうる sentinel の union。
		// 多 concrete DI の場合でも Fact/LSP が union を拾えるように必ず emit する。
		// どの concrete が何を返すかの内訳は下の per-concrete 行で補足する。
		if len(sentinels) > 0 {
			names := make([]string, len(sentinels))
			for i, s := range sentinels {
				names[i] = s.String()
			}
			pass.Reportf(pos, "%s returns sentinels: %s", fn.Name(), strings.Join(names, ", "))
		}

		// 具象が複数ある場合は concrete ごとに報告
		if len(breakdown.byConcrete) > 1 {
			concretes := make([]string, 0, len(breakdown.byConcrete))
			for c, sens := range breakdown.byConcrete {
				if len(sens) == 0 {
					continue
				}
				concretes = append(concretes, c)
			}
			if len(concretes) <= 1 {
				continue
			}
			sort.Strings(concretes)
			for _, c := range concretes {
				sens := breakdown.byConcrete[c]
				strs := make([]string, len(sens))
				for i, s := range sens {
					strs[i] = s.String()
				}
				sort.Strings(strs)
				pass.Reportf(pos, "%s returns sentinels via %s: %s", fn.Name(), c, strings.Join(strs, ", "))
			}
		}
	}

	return nil, nil
}

// collectSentinels は関数内の全 Return 命令から Sentinel Error を収集する。
// 戻り値:
//  1. union の Sentinel 集合（Fact エクスポートと合算診断に使用）
//  2. 関数直下の invoke 呼び出しにおける concrete ごとの Sentinel 内訳
func collectSentinels(fn *ssa.Function, pass *analysis.Pass, globalFuncs map[*ssa.Global]*ssa.Function, ifaceImpls []ifaceImpl) ([]SentinelInfo, invokeBreakdown) {
	ctx := &traceCtx{
		visited:      make(map[ssa.Value]bool),
		visitedFuncs: map[*ssa.Function]bool{fn: true}, // fn 自身を既訪問としてセット
		facts: func(obj types.Object, fact *SentinelFact) bool {
			if obj == nil {
				return false
			}
			return pass.ImportObjectFact(obj, fact)
		},
		globalFuncs:  globalFuncs,
		ifaceImpls:   ifaceImpls,
		reportf:      pass.Reportf,
		warnedNoWrap: make(map[*ssa.Call]bool),
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

	bd := collectInvokeBreakdown(fn, ctx, qualifierFor(pass.Pkg))
	return result, bd
}

// qualifierFor は types.TypeString 用の quilfier を返す。
// 解析対象パッケージ内の型は修飾なし、それ以外はパッケージ名を付ける。
func qualifierFor(pkg *types.Package) types.Qualifier {
	return func(p *types.Package) string {
		if p == nil || p == pkg {
			return ""
		}
		return p.Name()
	}
}

// returnsError はシグネチャが error 型を返すかを判定する。
func returnsError(sig *types.Signature) bool {
	results := sig.Results()
	errorIface := types.Universe.Lookup("error").Type()
	for i := range results.Len() {
		if types.Identical(results.At(i).Type(), errorIface) {
			return true
		}
	}
	return false
}

// findFuncObject は SSA 関数に対応する types.Object を返す。
func findFuncObject(_ *analysis.Pass, fn *ssa.Function) types.Object {
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
