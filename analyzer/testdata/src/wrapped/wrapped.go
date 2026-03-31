package wrapped

import (
	"errors"
	"fmt"
)

var ErrDatabase = errors.New("database error")

func QueryDB(query string) error { // want `QueryDB returns sentinels: wrapped\.ErrDatabase` QueryDB:`SentinelFact\(wrapped\.ErrDatabase\)`
	err := doQuery(query)
	if err != nil {
		return fmt.Errorf("QueryDB: %w", ErrDatabase)
	}
	return nil
}

func doQuery(q string) error { // want `doQuery returns sentinels: wrapped\.ErrDatabase` doQuery:`SentinelFact\(wrapped\.ErrDatabase\)`
	if q == "" {
		return fmt.Errorf("doQuery: %w", ErrDatabase)
	}
	return nil
}
