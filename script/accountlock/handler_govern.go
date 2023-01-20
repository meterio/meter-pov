package accountlock

import (
	"math/big"

	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
	"github.com/meterio/meter-pov/state"
)

func (a *AccountLock) GoverningHandler(env *setypes.ScriptEnv, ab *AccountLockBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	state := env.GetState()
	pList := state.GetProfileList()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	toRemove := []meter.Address{}
	curEpoch := a.GetCurrentEpoch()
	for _, p := range pList.Profiles {
		if p.ReleaseEpoch <= curEpoch {
			toRemove = append(toRemove, p.Addr)
		}
	}
	// remove the released profiles
	for _, r := range toRemove {
		pList.Remove(r)
	}

	a.logger.Debug("account lock governing done...", "epoch", curEpoch)
	state.SetProfileList(pList)
	return
}

func RestrictByAccountLock(addr meter.Address, state *state.State) (bool, *big.Int, *big.Int) {
	accountlock := GetAccountLockGlobInst()
	if accountlock == nil {
		//a.logger.Debug("accountlock is not initialized...")
		return false, nil, nil
	}

	list := state.GetProfileList()
	if list == nil {
		log.Warn("get the accountlock profile failed")
		return false, nil, nil
	}

	p := list.Get(addr)
	if p == nil {
		return false, nil, nil
	}

	if accountlock.GetCurrentEpoch() >= p.ReleaseEpoch {
		return false, nil, nil
	}

	log.Debug("the Address is not allowed to do transfer", "address", addr,
		"meter", p.MeterAmount.String(), "meterGov", p.MeterGovAmount.String())
	return true, p.MeterAmount, p.MeterGovAmount
}
