package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// ifaceImpl はインターフェース型と、それを実装する具象型のペアを保持する。
// 同一インターフェースに対する複数実装は別エントリとして保持される。
type ifaceImpl struct {
	iface    types.Type // 通常 *types.Named（interface underlying）
	concrete types.Type // *T または T
}

// buildInterfaceImpls は pass のファイル・スコープ・インポートを走査し、
// インターフェース → 具象型のマップを構築する。
//
// 収集経路は 2 つ:
//  1. compile-time assertion（var _ Iface = (*Concrete)(nil) 形式）
//  2. オートディスカバリ（types.Implements によるスコープ走査）
//
// (1) は明示的な DI ヒント、(2) は assertion を書き忘れた具象でも
// 解析対象に含められるようにするための補完経路。
func buildInterfaceImpls(pass *analysis.Pass) []ifaceImpl {
	impls := implsFromAssertions(pass)
	impls = append(impls, implsFromAutoDiscovery(pass)...)
	return dedupImpls(impls)
}

// implsFromAssertions は compile-time assertion を走査する。
//
// 認識するパターン:
//
//	var _ Iface = (*Concrete)(nil)   // ポインタレシーバ実装
//	var _ Iface = Concrete{}         // 値レシーバ実装
//	var _ Iface = (Concrete)(nil)    // 値型（参照型）
func implsFromAssertions(pass *analysis.Pass) []ifaceImpl {
	var result []ifaceImpl
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				vspec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				if len(vspec.Names) != 1 || vspec.Names[0].Name != "_" {
					continue
				}
				if vspec.Type == nil || len(vspec.Values) != 1 {
					continue
				}
				tv, ok := pass.TypesInfo.Types[vspec.Type]
				if !ok {
					continue
				}
				if _, ok := tv.Type.Underlying().(*types.Interface); !ok {
					continue
				}
				vt, ok := pass.TypesInfo.Types[vspec.Values[0]]
				if !ok || vt.Type == nil {
					continue
				}
				result = append(result, ifaceImpl{
					iface:    tv.Type,
					concrete: vt.Type,
				})
			}
		}
	}
	return result
}

// implsFromAutoDiscovery は pass.Pkg とその直接インポートのスコープを走査し、
// types.Implements が真となる具象型を列挙する。
//
// 対象インターフェース: pass.Pkg の Scope 直下に定義された非空インターフェース。
// 具象候補: pass.Pkg 自身と pass.Pkg.Imports() のスコープ直下の named 型
// （インターフェース型は除外）。メソッド集合がポインタレシーバを含む場合は
// *T 形式で登録する。
func implsFromAutoDiscovery(pass *analysis.Pass) []ifaceImpl {
	ifaces := collectLocalInterfaces(pass.Pkg)
	if len(ifaces) == 0 {
		return nil
	}
	candidates := collectConcreteCandidates(pass.Pkg)
	if len(candidates) == 0 {
		return nil
	}

	var result []ifaceImpl
	for _, iface := range ifaces {
		ifaceUnder, ok := iface.Underlying().(*types.Interface)
		if !ok || ifaceUnder.NumMethods() == 0 {
			continue
		}
		for _, c := range candidates {
			if types.Identical(c, iface) {
				continue
			}
			// 値型で満たすか
			if types.Implements(c, ifaceUnder) {
				result = append(result, ifaceImpl{iface: iface, concrete: c})
				continue
			}
			// ポインタ型で満たすか
			ptr := types.NewPointer(c)
			if types.Implements(ptr, ifaceUnder) {
				result = append(result, ifaceImpl{iface: iface, concrete: ptr})
			}
		}
	}
	return result
}

// collectLocalInterfaces は pkg.Scope() 直下の named interface 型を集める。
func collectLocalInterfaces(pkg *types.Package) []types.Type {
	if pkg == nil {
		return nil
	}
	var result []types.Type
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		tn, ok := scope.Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		named, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		if _, ok := named.Underlying().(*types.Interface); !ok {
			continue
		}
		result = append(result, named)
	}
	return result
}

// collectConcreteCandidates は pkg 自身と直接インポート先のスコープから、
// 具象型候補（interface 以外の named 型）を集める。
func collectConcreteCandidates(pkg *types.Package) []types.Type {
	if pkg == nil {
		return nil
	}
	var result []types.Type
	walk := func(p *types.Package) {
		if p == nil {
			return
		}
		scope := p.Scope()
		for _, name := range scope.Names() {
			tn, ok := scope.Lookup(name).(*types.TypeName)
			if !ok {
				continue
			}
			named, ok := tn.Type().(*types.Named)
			if !ok {
				continue
			}
			if _, ok := named.Underlying().(*types.Interface); ok {
				continue
			}
			result = append(result, named)
		}
	}
	walk(pkg)
	for _, imp := range pkg.Imports() {
		walk(imp)
	}
	return result
}

// dedupImpls は (iface, concrete) の同一ペアを 1 つにまとめる。
func dedupImpls(impls []ifaceImpl) []ifaceImpl {
	if len(impls) <= 1 {
		return impls
	}
	var result []ifaceImpl
	for _, e := range impls {
		dup := false
		for _, kept := range result {
			if types.Identical(e.iface, kept.iface) && types.Identical(e.concrete, kept.concrete) {
				dup = true
				break
			}
		}
		if !dup {
			result = append(result, e)
		}
	}
	return result
}

// lookupImpls は与えられたインターフェース型に対応する具象型一覧を返す。
func lookupImpls(impls []ifaceImpl, iface types.Type) []types.Type {
	var result []types.Type
	for _, i := range impls {
		if types.Identical(i.iface, iface) {
			result = append(result, i.concrete)
		}
	}
	return result
}

// lookupMethod は具象型 t からメソッド名で *types.Func を取得する。
// 見つからない場合は nil を返す。
func lookupMethod(t types.Type, name string) *types.Func {
	obj, _, _ := types.LookupFieldOrMethod(t, true, nil, name)
	fn, _ := obj.(*types.Func)
	return fn
}
