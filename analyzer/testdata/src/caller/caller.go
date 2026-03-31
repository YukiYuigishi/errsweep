package caller

import "callee"

// Process は別パッケージの関数が返す Sentinel を伝播する。
func Process() error { // want `Process returns sentinels: callee\.ErrCallee` Process:`SentinelFact\(callee\.ErrCallee\)`
	return callee.Fetch()
}
