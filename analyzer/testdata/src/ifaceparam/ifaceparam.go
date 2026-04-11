package ifaceparam

import (
	"errors"
	"fmt"
)

var (
	ErrA = errors.New("a")
	ErrB = errors.New("b")
)

type Finder interface {
	Find() error
}

type repoA struct{}
type repoB struct{}

func (repoA) Find() error { // want `Find returns sentinels: ifaceparam\.ErrA` Find:`SentinelFact\(ifaceparam\.ErrA\)`
	return ErrA
}

func (repoB) Find() error { // want `Find returns sentinels: ifaceparam\.ErrB` Find:`SentinelFact\(ifaceparam\.ErrB\)`
	return ErrB
}

func Select(flag bool) Finder {
	if flag {
		return repoA{}
	}
	return repoB{}
}

func Run(flag bool) error { // want `Run returns sentinels: ifaceparam\.ErrA, ifaceparam\.ErrB` `Run returns sentinels via repoA: ifaceparam\.ErrA` `Run returns sentinels via repoB: ifaceparam\.ErrB` Run:`SentinelFact\(ifaceparam\.ErrA, ifaceparam\.ErrB\)`
	f := Select(flag)
	return f.Find()
}

func RunWithLocal(flag bool) error { // want `RunWithLocal returns sentinels: ifaceparam\.ErrA, ifaceparam\.ErrB` `RunWithLocal returns sentinels via repoA: ifaceparam\.ErrA` `RunWithLocal returns sentinels via repoB: ifaceparam\.ErrB` RunWithLocal:`SentinelFact\(ifaceparam\.ErrA, ifaceparam\.ErrB\)`
	var f Finder
	if flag {
		f = repoA{}
	} else {
		f = repoB{}
	}
	return f.Find()
}

func WrapAndRun(flag bool) error { // want `WrapAndRun returns sentinels: ifaceparam\.ErrA, ifaceparam\.ErrB` WrapAndRun:`SentinelFact\(ifaceparam\.ErrA, ifaceparam\.ErrB\)`
	if err := Run(flag); err != nil {
		return fmt.Errorf("WrapAndRun: %w", err)
	}
	return nil
}
