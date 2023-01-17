package staking

import (
	"errors"

	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
)

func (s *Staking) DelegateExitJailHandler(env *setypes.ScriptEnv, sb *StakingBody, gas uint64) (leftOverGas uint64, err error) {
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
	inJailList := state.GetInJailList()
	statisticsList := state.GetDelegateStatList()

	jailed := inJailList.Get(sb.CandAddr)
	if jailed == nil {
		log.Info("not in jail list ...", "address", sb.CandAddr, "name", sb.CandName)
		return
	}

	if state.GetBalance(jailed.Addr).Cmp(jailed.BailAmount) < 0 {
		log.Error("not enough balance for bail")
		err = errors.New("not enough balance for bail")
		return
	}

	// take actions
	if err = env.CollectBailMeterGov(jailed.Addr, jailed.BailAmount); err != nil {
		log.Error(err.Error())
		return
	}
	inJailList.Remove(jailed.Addr)
	statisticsList.Remove(jailed.Addr)

	log.Info("removed from jail list ...", "address", jailed.Addr, "name", jailed.Name)
	state.SetInJailList(inJailList)
	state.SetDelegateStatList(statisticsList)
	return
}
