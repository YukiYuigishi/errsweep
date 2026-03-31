package phi

import "errors"

var ErrNotFound = errors.New("not found")
var ErrTimeout = errors.New("timeout")

func Fetch(id int, fast bool) error { // want `Fetch returns sentinels: phi\.ErrNotFound, phi\.ErrTimeout` Fetch:`SentinelFact\(phi\.ErrNotFound, phi\.ErrTimeout\)`
	if id <= 0 {
		return ErrNotFound
	}
	if !fast {
		return ErrTimeout
	}
	return nil
}
