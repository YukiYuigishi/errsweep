package interprocedural

// callerFirst は callee より先に定義されている（SrcFuncs の順序依存を確認するケース）。
func callerFirst() error { // want `callerFirst returns sentinels: interprocedural\.ErrDB` callerFirst:`SentinelFact\(interprocedural\.ErrDB\)`
	return calleeAfter()
}

func calleeAfter() error { // want `calleeAfter returns sentinels: interprocedural\.ErrDB` calleeAfter:`SentinelFact\(interprocedural\.ErrDB\)`
	return ErrDB
}
