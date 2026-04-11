package analyzer

import (
	"go/types"

	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/ssa"
)

var analyzeRTA = rta.Analyze

// collectRuntimeConcreteTypes は RTA で到達可能なランタイム型を収集し、
// interface 実装候補として使える具象型（非 interface）へ正規化して返す。
func collectRuntimeConcreteTypes(srcFuncs []*ssa.Function, pkg *ssa.Package) []types.Type {
	roots := collectRTARoots(srcFuncs, pkg)
	if len(roots) == 0 {
		return nil
	}
	result := safeAnalyzeRTA(roots)
	if result == nil || result.RuntimeTypes.Len() == 0 {
		return nil
	}

	var out []types.Type
	result.RuntimeTypes.Iterate(func(key types.Type, _ any) {
		t := stripRTATypeWrapper(key)
		if t == nil {
			return
		}
		if _, ok := t.Underlying().(*types.Interface); ok {
			return
		}
		out = append(out, t)
	})
	return dedupConcreteTypes(out)
}

func safeAnalyzeRTA(roots []*ssa.Function) *rta.Result {
	var result *rta.Result
	defer func() {
		if recover() != nil {
			result = nil
		}
	}()
	result = analyzeRTA(roots, false)
	return result
}

func collectRTARoots(srcFuncs []*ssa.Function, pkg *ssa.Package) []*ssa.Function {
	seen := map[*ssa.Function]bool{}
	var roots []*ssa.Function
	add := func(fn *ssa.Function) {
		if fn == nil || seen[fn] {
			return
		}
		seen[fn] = true
		roots = append(roots, fn)
	}
	for _, fn := range srcFuncs {
		add(fn)
	}
	if pkg != nil {
		if initFn, ok := pkg.Members["init"].(*ssa.Function); ok {
			add(initFn)
		}
	}
	return roots
}

// stripRTATypeWrapper は RTA の RuntimeTypes に現れる wrapper を除去する。
// RTA は *T や interface 実体が混在するため、named concrete へ寄せる。
func stripRTATypeWrapper(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	if ptr, ok := t.(*types.Pointer); ok {
		if named, ok := ptr.Elem().(*types.Named); ok {
			return types.NewPointer(named)
		}
	}
	if named, ok := t.(*types.Named); ok {
		return named
	}
	return t
}
