package analyzer

import (
	"go/constant"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// fmtErrorfWrappedArg は *ssa.Call が fmt.Errorf(%w, ...) 呼び出しの場合に
// %w に対応する引数を返す。fmt.Errorf でない場合または %w がない場合は nil を返す。
//
// Go SSA では varargs は []any のスライスとして渡される。例:
//   fmt.Errorf("msg: %w", err)
//   → args[0] = Const("msg: %w"), args[1] = Slice(Alloc [1]any)
// そのため、スライスの backing array から対象インデックスの値を取り出す。
func fmtErrorfWrappedArg(call *ssa.Call) ssa.Value {
	callee := call.Call.StaticCallee()
	if callee == nil {
		return nil
	}
	if callee.Package() == nil || callee.Package().Pkg.Path() != "fmt" || callee.Name() != "Errorf" {
		return nil
	}

	args := call.Call.Args
	if len(args) < 2 {
		return nil
	}

	// 第1引数がフォーマット文字列の定数
	fmtArg, ok := args[0].(*ssa.Const)
	if !ok {
		return nil
	}
	if fmtArg.Value == nil || fmtArg.Value.Kind() != constant.String {
		return nil
	}
	fmtStr := constant.StringVal(fmtArg.Value)

	// %w の位置（0-indexed）を探す
	wIndex := findWVerbIndex(fmtStr)
	if wIndex < 0 {
		return nil
	}

	// varargs は args[1] の *ssa.Slice として渡される
	return extractSliceElement(args[1], wIndex)
}

// extractSliceElement は SSA で生成された varargs スライスから index 番目の要素を返す。
// パターン: Slice(Alloc [N]any) の backing array への Store を探す。
func extractSliceElement(sliceVal ssa.Value, index int) ssa.Value {
	slice, ok := sliceVal.(*ssa.Slice)
	if !ok {
		// スライスでない場合はそのまま返す（非varargs呼び出しなど）
		return sliceVal
	}

	alloc, ok := slice.X.(*ssa.Alloc)
	if !ok {
		return nil
	}

	// alloc の参照元から IndexAddr[index] → Store の値を探す
	if alloc.Referrers() == nil {
		return nil
	}
	for _, ref := range *alloc.Referrers() {
		ia, ok := ref.(*ssa.IndexAddr)
		if !ok {
			continue
		}
		// インデックスが定数で wIndex と一致するか確認
		idxConst, ok := ia.Index.(*ssa.Const)
		if !ok {
			continue
		}
		if idxConst.Value == nil || idxConst.Value.Kind() != constant.Int {
			continue
		}
		if int(constant.Val(idxConst.Value).(int64)) != index {
			continue
		}
		// この IndexAddr への Store を探す
		if ia.Referrers() == nil {
			continue
		}
		for _, iaRef := range *ia.Referrers() {
			if store, ok := iaRef.(*ssa.Store); ok {
				return store.Val
			}
		}
	}
	return nil
}

// findWVerbIndex はフォーマット文字列中の %w の位置（0-indexed）を返す。
// %w が存在しない場合は -1 を返す。
func findWVerbIndex(format string) int {
	index := 0
	i := 0
	for i < len(format) {
		if format[i] != '%' {
			i++
			continue
		}
		i++ // skip '%'
		if i >= len(format) {
			break
		}
		// フラグ・幅・精度をスキップ
		for i < len(format) && strings.ContainsRune("+-# 0123456789.*", rune(format[i])) {
			if format[i] == '*' {
				index++ // * は引数を消費する
			}
			i++
		}
		if i >= len(format) {
			break
		}
		verb := format[i]
		i++
		if verb == 'w' {
			return index
		}
		if verb != '%' {
			index++ // 通常の動詞は引数を1つ消費
		}
	}
	return -1
}
