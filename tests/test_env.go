package tests

import (
	"github.com/meterio/meter-pov/runtime"
	"github.com/meterio/meter-pov/state"
)

type TestEnv struct {
	Runtime     *runtime.Runtime
	State       *state.State
	BktCreateTS uint64
	CurrentTS   uint64
	ChainTag    byte
}
