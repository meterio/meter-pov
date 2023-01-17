package accountlock

import (
	"errors"

	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
)

func (a *AccountLock) HandleAccountLockRemove(env *setypes.ScriptEnv, ab *AccountLockBody, gas uint64) (leftOverGas uint64, err error) {
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

	p := pList.Get(ab.FromAddr)
	if p == nil {
		err = errors.New("profile is no in state")
		log.Error("profile is not in state", "addr", ab.FromAddr)
		return
	}

	pList.Remove(ab.FromAddr)

	state.SetProfileList(pList)
	return
}
