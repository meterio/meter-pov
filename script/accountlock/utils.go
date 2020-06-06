package accountlock

import (
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/vesting"
)

func (a *AccountLock) GetCurrentEpoch() uint32 {
	bestBlock := a.chain.BestBlock()
	return uint32(bestBlock.GetBlockEpoch())
}

func (a *AccountLock) RestrictByVestingPlan(addr meter.Address) bool {
	curEpoch := a.GetCurrentEpoch()
	return vesting.RestrictTransfer(addr, curEpoch)
}
