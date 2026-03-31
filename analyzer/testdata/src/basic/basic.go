package basic

import "errors"

var ErrNotFound = errors.New("not found")
var ErrPermission = errors.New("permission denied")

func FindUser(id int) error { // want `FindUser returns sentinels: basic\.ErrNotFound` FindUser:`SentinelFact\(basic\.ErrNotFound\)`
	if id <= 0 {
		return ErrNotFound
	}
	return nil
}

func GetItem(id int) (string, error) { // want `GetItem returns sentinels: basic\.ErrPermission` GetItem:`SentinelFact\(basic\.ErrPermission\)`
	if id < 0 {
		return "", ErrPermission
	}
	return "item", nil
}

func NoError() error {
	return nil
}
