package staking

import (
	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
)

// this is debug API, only executor has the right to call
func (s *Staking) DelegateStatFlushHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	state := env.GetState()
	statisticsList := &meter.DelegateStatList{}
	inJailList := &meter.InJailList{}

	state.SetDelegateStatList(statisticsList)
	state.SetInJailList(inJailList)
	return
}
