package interprocedural

import (
	"errors"
	"fmt"
)

var ErrDB = errors.New("db error")
var ErrNotFound = errors.New("not found")

// repository は直接 Sentinel を返す。
func repository(id int) error { // want `repository returns sentinels: interprocedural\.ErrDB, interprocedural\.ErrNotFound` repository:`SentinelFact\(interprocedural\.ErrDB, interprocedural\.ErrNotFound\)`
	if id < 0 {
		return ErrDB
	}
	return ErrNotFound
}

// useCase は同一パッケージ内の関数を呼び出して error を返す。
func useCase(id int) error { // want `useCase returns sentinels: interprocedural\.ErrDB, interprocedural\.ErrNotFound` useCase:`SentinelFact\(interprocedural\.ErrDB, interprocedural\.ErrNotFound\)`
	if err := repository(id); err != nil {
		return err
	}
	return nil
}

// handler は useCase を経由して Sentinel を返す（2 段階）。
func handler(id int) error { // want `handler returns sentinels: interprocedural\.ErrDB, interprocedural\.ErrNotFound` handler:`SentinelFact\(interprocedural\.ErrDB, interprocedural\.ErrNotFound\)`
	return fmt.Errorf("handler: %w", useCase(id))
}
