package nonwrap

import (
	"errors"
	"fmt"
)

var ErrLost = errors.New("lost")

func ViaPercentV() error {
	return fmt.Errorf("wrapped with v: %v", ErrLost) // want `fmt\.Errorf without %w loses sentinel identity; sentinel tracing skipped`
}

func ViaNoVerb() error {
	return fmt.Errorf("plain message") // want `fmt\.Errorf without %w loses sentinel identity; sentinel tracing skipped`
}
