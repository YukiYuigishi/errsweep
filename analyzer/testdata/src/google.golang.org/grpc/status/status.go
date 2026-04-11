package status

import "errors"

func Errorf(_ any, _ string, _ ...any) error {
	return errors.New("grpc status")
}
