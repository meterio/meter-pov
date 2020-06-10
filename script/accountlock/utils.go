package accountlock

import (
	"bytes"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
)

func (a *AccountLock) GetCurrentEpoch() uint32 {
	bestBlock := a.chain.BestBlock()
	return uint32(bestBlock.GetBlockEpoch())
}

func (a *AccountLock) IsExceptionalAccount(addr meter.Address, state *state.State) bool {
	// executor is exceptional
	executor := meter.BytesToAddress(builtin.Params.Native(state).Get(meter.KeyExecutorAddress).Bytes())
	if bytes.Compare(addr.Bytes(), executor.Bytes()) == 0 {
		return true
	}

	return false
}
