package accountlock

import (
	"errors"

	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
)

func (a *AccountLock) HandleAccountLockAdd(env *setypes.ScriptEnv, ab *AccountLockBody, gas uint64) (leftOverGas uint64, err error) {
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

	if p := pList.Get(ab.FromAddr); p != nil {
		err = errors.New("profile is already in state")
		a.logger.Error("profile is already in state", "addr", ab.FromAddr)
		return
	}

	p := meter.NewProfile(ab.FromAddr, ab.Memo, ab.LockEpoch, ab.ReleaseEpoch, ab.MeterAmount, ab.MeterGovAmount)
	pList.Add(p)

	state.SetProfileList(pList)
	return
}
