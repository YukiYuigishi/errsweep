package funcvar

import "errors"

var ErrFuncVar = errors.New("funcvar error")
var ErrMulti = errors.New("multi error")

// Loader は単一 error を返す関数型（DI でよく使われるパターン）。
type Loader func() error

// MultiLoader は複数値を返す関数型。
type MultiLoader func() (int, error)

// concreteLoader は直接 Sentinel を返す具体的な実装。
func concreteLoader() error { // want `concreteLoader returns sentinels: funcvar\.ErrFuncVar` concreteLoader:`SentinelFact\(funcvar\.ErrFuncVar\)`
	return ErrFuncVar
}

// concreteMulti は (int, error) を返す具体的な実装。
func concreteMulti() (int, error) { // want `concreteMulti returns sentinels: funcvar\.ErrMulti` concreteMulti:`SentinelFact\(funcvar\.ErrMulti\)`
	return 0, ErrMulti
}

// パッケージレベルの関数変数（DI パターン）。
var load Loader = concreteLoader
var multiLoad MultiLoader = concreteMulti

// RunDirect は関数変数を直接呼び出して error を返す（単一 error 返し）。
// load → concreteLoader → ErrFuncVar と辿れる必要がある。
func RunDirect() error { // want `RunDirect returns sentinels: funcvar\.ErrFuncVar` RunDirect:`SentinelFact\(funcvar\.ErrFuncVar\)`
	return load()
}

// RunMulti は複数値を返す関数変数を呼び出し、error を返す（Extract 経由）。
// multiLoad → concreteMulti → ErrMulti と辿れる必要がある。
func RunMulti() error { // want `RunMulti returns sentinels: funcvar\.ErrMulti` RunMulti:`SentinelFact\(funcvar\.ErrMulti\)`
	_, err := multiLoad()
	return err
}
