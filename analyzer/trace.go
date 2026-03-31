package analyzer

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

const maxTraceDepth = 5

// traceCtx は後方探索の実行コンテキスト。
type traceCtx struct {
	visited      map[ssa.Value]bool
	visitedFuncs map[*ssa.Function]bool               // 関数間の循環呼び出し検出
	facts        func(types.Object, *SentinelFact) bool // ImportObjectFact
}

// traceValue は SSA値 v から後方に探索し、到達しうる Sentinel Error を返す。
// handled: Return, Global, Call, Phi, MakeInterface, ChangeInterface, UnOp, Extract, Const(nil)
func traceValue(v ssa.Value, depth int, ctx *traceCtx) []SentinelInfo {
	if depth > maxTraceDepth {
		return nil
	}
	if ctx.visited[v] {
		return nil
	}
	ctx.visited[v] = true

	switch x := v.(type) {
	case *ssa.UnOp:
		// *Global の deref → Sentinel
		if x.Op == '*' {
			if g, ok := x.X.(*ssa.Global); ok {
				return sentinelFromGlobal(g)
			}
		}
		return traceValue(x.X, depth+1, ctx)

	case *ssa.MakeInterface:
		return traceValue(x.X, depth+1, ctx)

	case *ssa.ChangeInterface:
		return traceValue(x.X, depth+1, ctx)

	case *ssa.Phi:
		var result []SentinelInfo
		for _, edge := range x.Edges {
			result = appendUniq(result, traceValue(edge, depth+1, ctx))
		}
		return result

	case *ssa.Call:
		// fmt.Errorf %w のアンラップ
		if wrapped := fmtErrorfWrappedArg(x); wrapped != nil {
			return traceValue(wrapped, depth+1, ctx)
		}
		// 静的呼び出し先を再帰探索
		if callee := x.Call.StaticCallee(); callee != nil {
			return sentinelFromCallee(callee, depth+1, ctx)
		}
		return nil

	case *ssa.Extract:
		// タプルの要素取得 → タプルを生成した命令を辿る
		return traceExtract(x, depth, ctx)

	case *ssa.Const:
		// nil return はスキップ
		return nil

	case *ssa.Global:
		return sentinelFromGlobal(x)

	default:
		return nil
	}
}

// sentinelFromGlobal はグローバル変数がパッケージレベルの Sentinel 宣言かを判定する。
func sentinelFromGlobal(g *ssa.Global) []SentinelInfo {
	if g.Package() == nil {
		return nil
	}
	name := g.Name()
	if len(name) < 3 {
		return nil
	}
	// パッケージレベル var Err* のみ対象
	if name[:3] != "Err" {
		return nil
	}
	pkgPath := g.Package().Pkg.Path()
	return []SentinelInfo{{PkgPath: pkgPath, Name: name}}
}

// sentinelFromCallee は呼び出し先関数の Sentinel を返す。
// 優先順位:
//  1. 既知マッピング（known.go）
//  2. ImportObjectFact キャッシュ（クロスパッケージや解析済み関数）
//  3. 同一モジュール内の関数ボディへ直接再帰（depth+1）
func sentinelFromCallee(callee *ssa.Function, depth int, ctx *traceCtx) []SentinelInfo {
	if callee.Package() == nil {
		return nil
	}

	// 1. 既知マッピング
	if known, ok := knownErrorMap[callee.RelString(nil)]; ok {
		return known
	}

	// 2. Fact キャッシュ（クロスパッケージ解析済み関数）
	if ctx.facts != nil {
		if obj := callee.Object(); obj != nil {
			var fact SentinelFact
			if ctx.facts(obj, &fact) {
				return fact.Errors
			}
		}
	}

	// 3. ボディへ直接再帰（同一モジュール内かつ未訪問）
	if depth > maxTraceDepth {
		return nil
	}
	if callee.Blocks == nil {
		return nil
	}
	if ctx.visitedFuncs[callee] {
		return nil
	}
	ctx.visitedFuncs[callee] = true

	// callee 内の全 Return から Sentinel を収集
	childCtx := &traceCtx{
		visited:      make(map[ssa.Value]bool),
		visitedFuncs: ctx.visitedFuncs,
		facts:        ctx.facts,
	}
	var result []SentinelInfo
	for _, block := range callee.Blocks {
		for _, instr := range block.Instrs {
			ret, ok := instr.(*ssa.Return)
			if !ok {
				continue
			}
			for _, v := range ret.Results {
				if !isErrorType(v.Type()) {
					continue
				}
				childCtx.visited = make(map[ssa.Value]bool)
				result = appendUniq(result, traceValue(v, depth, childCtx))
			}
		}
	}
	return result
}

// traceExtract は *ssa.Extract の生成元を辿る。
func traceExtract(x *ssa.Extract, depth int, ctx *traceCtx) []SentinelInfo {
	switch tup := x.Tuple.(type) {
	case *ssa.Call:
		// (val, err) = f() のような呼び出し
		// error インターフェースのインデックスに一致する場合のみ追跡
		if !isErrorType(x.Type()) {
			return nil
		}
		if wrapped := fmtErrorfWrappedArg(tup); wrapped != nil {
			return traceValue(wrapped, depth+1, ctx)
		}
		if callee := tup.Call.StaticCallee(); callee != nil {
			return sentinelFromCallee(callee, depth+1, ctx)
		}
	}
	return nil
}

// isErrorType は型が error インターフェースかを判定する。
func isErrorType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		iface, ok2 := t.Underlying().(*types.Interface)
		return ok2 && iface.NumMethods() == 1 && iface.Method(0).Name() == "Error"
	}
	_ = named
	return types.Implements(t, types.Universe.Lookup("error").Type().Underlying().(*types.Interface))
}

// appendUniq は重複なく SentinelInfo を追加する。
func appendUniq(dst []SentinelInfo, src []SentinelInfo) []SentinelInfo {
	for _, s := range src {
		found := false
		for _, d := range dst {
			if d == s {
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, s)
		}
	}
	return dst
}
