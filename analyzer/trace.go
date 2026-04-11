package analyzer

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

const maxTraceDepth = 5

// traceCtx は後方探索の実行コンテキスト。
type traceCtx struct {
	visited      map[ssa.Value]bool
	visitedFuncs map[*ssa.Function]bool                 // 関数間の循環呼び出し検出
	facts        func(types.Object, *SentinelFact) bool // ImportObjectFact
	// globalFuncs は var f FuncType = Concrete の初期値マップ（関数変数 DI 解決用）。
	// analyzer.go の run() で SrcFuncs と pkg.Members["init"] を走査して構築する。
	globalFuncs map[*ssa.Global]*ssa.Function
	// ifaceImpls は compile-time assertion から抽出したインターフェース実装マップ。
	// interface 経由 DI の呼び出し先解決に使う。
	ifaceImpls []ifaceImpl
}

// traceValue は SSA値 v から後方に探索し、到達しうる Sentinel Error を返す。
// handled: Return, Global, Call, Phi, MakeInterface, ChangeInterface, UnOp, Alloc(via stores), Extract, Const(nil)
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
		// token.MUL (14) が SSA の dereference 演算子。'*' (rune 42) とは別物。
		if x.Op == token.MUL {
			if g, ok := x.X.(*ssa.Global); ok {
				return sentinelFromGlobal(g)
			}
			// defer + rundefers が絡むと named-return 風に *ssa.Alloc へ一度 Store してから
			// reload した値が返される。Alloc への全 Store を遡って辿る。
			if alloc, ok := x.X.(*ssa.Alloc); ok {
				return traceAllocStores(alloc, depth, ctx)
			}
		}
		return traceValue(x.X, depth+1, ctx)

	case *ssa.MakeInterface:
		// まず既存の探索（Global 変数や wrap 越しの Sentinel を優先）
		if inner := traceValue(x.X, depth+1, ctx); len(inner) > 0 {
			return inner
		}
		// カスタムエラー型としてそのまま Sentinel 化
		// 例: return &NotFoundError{ID: id}
		if info, ok := customErrorType(x.X.Type()); ok {
			return []SentinelInfo{*info}
		}
		return nil

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
		// 関数変数経由の呼び出し（DI パターン: var f FuncType = ConcreteFunc）
		if callee := resolveIndirectCallee(x.Call, ctx); callee != nil {
			return sentinelFromCallee(callee, depth+1, ctx)
		}
		// インターフェース経由の呼び出し（DI パターン: compile-time assertion 解決）
		if x.Call.IsInvoke() {
			return sentinelFromInvoke(x.Call, depth+1, ctx)
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

// traceAllocStores は *ssa.Alloc に書き込まれた全ての値を後方に辿る。
// 主に defer + rundefers による named-return 風の Store → rundefers → Load パターンで、
// 直前に Store された sentinel を拾い上げるために使う。
func traceAllocStores(alloc *ssa.Alloc, depth int, ctx *traceCtx) []SentinelInfo {
	refs := alloc.Referrers()
	if refs == nil {
		return nil
	}
	var result []SentinelInfo
	for _, ref := range *refs {
		store, ok := ref.(*ssa.Store)
		if !ok || store.Addr != alloc {
			continue
		}
		result = appendUniq(result, traceValue(store.Val, depth+1, ctx))
	}
	return result
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
	return []SentinelInfo{{PkgPath: pkgPath, Name: name, Kind: KindVar}}
}

// customErrorType は t が「ユーザー定義のエラー型」かを判定する。
// 対象とする条件:
//   - named 型 or *named
//   - error interface を実装
//   - エクスポートされた型名
//   - errors / fmt パッケージの primitive 型（errorString, wrapError 等）でない
//
// これらを満たす場合、型名ベースの Sentinel として返す。
func customErrorType(t types.Type) (*SentinelInfo, bool) {
	pointer := false
	elem := t
	if ptr, ok := t.(*types.Pointer); ok {
		pointer = true
		elem = ptr.Elem()
	}
	named, ok := elem.(*types.Named)
	if !ok {
		return nil, false
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return nil, false
	}
	if !obj.Exported() {
		return nil, false
	}
	pkgPath := obj.Pkg().Path()
	if pkgPath == "errors" || pkgPath == "fmt" {
		return nil, false
	}
	errorIface, ok := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	if !ok {
		return nil, false
	}
	check := types.Type(elem)
	if pointer {
		check = types.NewPointer(elem)
	}
	if !types.Implements(check, errorIface) {
		return nil, false
	}
	return &SentinelInfo{
		PkgPath: pkgPath,
		Name:    obj.Name(),
		Kind:    KindType,
		Pointer: pointer,
	}, true
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
		globalFuncs:  ctx.globalFuncs,
		ifaceImpls:   ctx.ifaceImpls,
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
		// 関数変数経由の呼び出し（DI パターン）
		if callee := resolveIndirectCallee(tup.Call, ctx); callee != nil {
			return sentinelFromCallee(callee, depth+1, ctx)
		}
		// インターフェース経由の呼び出し（DI パターン）
		if tup.Call.IsInvoke() {
			return sentinelFromInvoke(tup.Call, depth+1, ctx)
		}
	}
	return nil
}

// sentinelFromInvoke はインターフェース呼び出し (IsInvoke) の Sentinel を解決する。
// compile-time assertion (var _ I = (*T)(nil)) またはオートディスカバリで
// 得た具象型を引き、対応するメソッドを ImportObjectFact で参照する。
// 戻り値は全具象型の union。
func sentinelFromInvoke(call ssa.CallCommon, depth int, ctx *traceCtx) []SentinelInfo {
	if !call.IsInvoke() {
		return nil
	}
	if depth > maxTraceDepth {
		return nil
	}
	// call.Value はインターフェース値、その型がインターフェース型。
	ifaceType := call.Value.Type()
	method := call.Method
	if method == nil {
		return nil
	}
	concretes := lookupImpls(ctx.ifaceImpls, ifaceType)
	if len(concretes) == 0 {
		return nil
	}
	var result []SentinelInfo
	for _, ct := range concretes {
		result = appendUniq(result, sentinelsForConcreteMethod(ct, method.Name(), ctx))
	}
	return result
}

// sentinelsForConcreteMethod は具象型 ct の methodName メソッドから
// Sentinel を解決する（known → Fact の順）。
func sentinelsForConcreteMethod(ct types.Type, methodName string, ctx *traceCtx) []SentinelInfo {
	if known, ok := knownMethodSentinels(ct, methodName); ok {
		return known
	}
	fn := lookupMethod(ct, methodName)
	if fn == nil {
		return nil
	}
	if ctx.facts != nil {
		var fact SentinelFact
		if ctx.facts(fn, &fact) {
			return fact.Errors
		}
	}
	return nil
}

// invokeBreakdown は関数直下の invoke 呼び出しを具象型ごとに分解した結果。
// 複数の invoke 呼び出し・複数具象にまたがる Sentinel を concrete 名で集約する。
type invokeBreakdown struct {
	// byConcrete のキーは types.TypeString(concrete, qualifier) の結果
	// （例: "*repository.TagRepository"）。具象が 1 つも解決できなかった
	// invoke は記録されない。
	byConcrete map[string][]SentinelInfo
}

// collectInvokeBreakdown は fn のブロックを直接走査し、
// IsInvoke な呼び出しについて concrete ごとの Sentinel 内訳を構築する。
// ネストした callee 内の invoke は対象外（トップレベルの内訳のみ）。
func collectInvokeBreakdown(fn *ssa.Function, ctx *traceCtx, qualifier types.Qualifier) invokeBreakdown {
	bd := invokeBreakdown{byConcrete: map[string][]SentinelInfo{}}
	if fn == nil || fn.Blocks == nil {
		return bd
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			call, ok := instr.(*ssa.Call)
			if !ok || !call.Call.IsInvoke() {
				continue
			}
			method := call.Call.Method
			if method == nil {
				continue
			}
			ifaceType := call.Call.Value.Type()
			concretes := lookupImpls(ctx.ifaceImpls, ifaceType)
			for _, ct := range concretes {
				name := types.TypeString(ct, qualifier)
				sens := sentinelsForConcreteMethod(ct, method.Name(), ctx)
				bd.byConcrete[name] = appendUniq(bd.byConcrete[name], sens)
			}
		}
	}
	return bd
}

// knownMethodSentinels は concreteType.methodName を known.go の形式キーで検索する。
// "(*pkg.Type).Method" 形式。
func knownMethodSentinels(concrete types.Type, methodName string) ([]SentinelInfo, bool) {
	// RelString 互換の key を作る: "(*pkg.Type).Method" or "(pkg.Type).Method"
	var key string
	switch t := concrete.(type) {
	case *types.Pointer:
		named, ok := t.Elem().(*types.Named)
		if !ok {
			return nil, false
		}
		obj := named.Obj()
		if obj.Pkg() == nil {
			return nil, false
		}
		key = "(*" + obj.Pkg().Path() + "." + obj.Name() + ")." + methodName
	case *types.Named:
		obj := t.Obj()
		if obj.Pkg() == nil {
			return nil, false
		}
		key = "(" + obj.Pkg().Path() + "." + obj.Name() + ")." + methodName
	default:
		return nil, false
	}
	v, ok := knownErrorMap[key]
	return v, ok
}

// resolveIndirectCallee は静的解決できない呼び出しに対して、
// ctx.globalFuncs から具体的な *ssa.Function を返す。
// 解決できない場合（引数・ローカル変数経由など）は nil を返す。
func resolveIndirectCallee(call ssa.CallCommon, ctx *traceCtx) *ssa.Function {
	if call.StaticCallee() != nil {
		return nil // 静的呼び出しは呼び出し元で処理済み
	}
	if ctx.globalFuncs == nil {
		return nil
	}
	// 関数変数のロード: t = *globalVar
	// token.MUL (14) が dereference 演算子。'*' (rune 42) とは別物なので注意。
	unop, ok := call.Value.(*ssa.UnOp)
	if !ok || unop.Op != token.MUL {
		return nil
	}
	g, ok := unop.X.(*ssa.Global)
	if !ok {
		return nil
	}
	return ctx.globalFuncs[g]
}

// BuildGlobalFuncMap は SSA の全関数（SrcFuncs + pkg.Members["init"]）を走査し、
// var f FuncType = ConcreteFunc パターンで初期化されたグローバル変数のマップを構築する。
//
// var f FuncType = ConcreteFunc の SSA 表現（init 内）:
//
//	t4 = changetype FuncType <- func(...) (ConcreteFunc)  ; *ssa.ChangeType
//	*f = t4                                                ; *ssa.Store
func BuildGlobalFuncMap(srcFuncs []*ssa.Function, pkg *ssa.Package) map[*ssa.Global]*ssa.Function {
	m := make(map[*ssa.Global]*ssa.Function)

	searchFn := func(fn *ssa.Function) {
		if fn == nil || fn.Blocks == nil {
			return
		}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				store, ok := instr.(*ssa.Store)
				if !ok {
					continue
				}
				g, ok := store.Addr.(*ssa.Global)
				if !ok {
					continue
				}
				if f := funcFromSSAValue(store.Val); f != nil {
					m[g] = f
				}
			}
		}
	}

	for _, fn := range srcFuncs {
		searchFn(fn)
	}
	// pkg.Members["init"] はパッケージレベル var の初期化 Store を含む
	if pkg != nil {
		if initFn, ok := pkg.Members["init"].(*ssa.Function); ok {
			searchFn(initFn)
		}
	}
	return m
}

// funcFromSSAValue は SSA 値から *ssa.Function を取り出す。
// var f FuncType = Concrete では ChangeType でラップされるため再帰的に剥がす。
func funcFromSSAValue(v ssa.Value) *ssa.Function {
	switch x := v.(type) {
	case *ssa.Function:
		return x
	case *ssa.ChangeType:
		// 具名関数型への変換: changetype FuncType <- func() error (concrete)
		return funcFromSSAValue(x.X)
	case *ssa.MakeClosure:
		if fn, ok := x.Fn.(*ssa.Function); ok {
			return fn
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
