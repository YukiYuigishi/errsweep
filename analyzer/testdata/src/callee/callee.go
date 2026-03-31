package callee

import "errors"

var ErrCallee = errors.New("callee error")

func Fetch() error { // want `Fetch returns sentinels: callee\.ErrCallee` Fetch:`SentinelFact\(callee\.ErrCallee\)`
	return ErrCallee
}
