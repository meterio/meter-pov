package accountlock

import (
	"errors"

	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
)

func (a *AccountLock) HandleAccountLockTransfer(env *setypes.ScriptEnv, ab *AccountLockBody, gas uint64) (leftOverGas uint64, err error) {
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

	// can not transfer to address which already has account lock
	if pTo := pList.Get(ab.ToAddr); pTo != nil {
		err = errors.New("profile of ToAddr is already in state")
		a.logger.Error("profile is already in state", "addr", ab.ToAddr)
		return
	}

	// from address should not have account lock, ONLY some exceptional addresses
	pFrom := pList.Get(ab.FromAddr)
	if pFrom != nil {
		if state.IsExclusiveAccount(ab.FromAddr) == false {
			err = errors.New("profile of FromAddr is already in state")
			a.logger.Error("profile is already in state", "addr", ab.FromAddr)
			return
		}
		a.logger.Info("profile is in state but is exceptional address, continue ...", "addr", ab.FromAddr)
	}

	// check have enough balance
	if ab.MeterAmount.Sign() != 0 {
		if state.GetEnergy(ab.FromAddr).Cmp(ab.MeterAmount) < 0 {
			err = errors.New("not enough meter balance")
			return
		}
	}
	if ab.MeterGovAmount.Sign() != 0 {
		if state.GetBalance(ab.FromAddr).Cmp(ab.MeterGovAmount) < 0 {
			err = errors.New("not enough meter-gov balance")
			return
		}
	}

	// sanity done!
	if pFrom == nil {
		p := meter.NewProfile(ab.ToAddr, ab.Memo, ab.LockEpoch, ab.ReleaseEpoch, ab.MeterAmount, ab.MeterGovAmount)
		pList.Add(p)
	} else {
		var release uint32
		if ab.ReleaseEpoch < pFrom.ReleaseEpoch {
			release = pFrom.ReleaseEpoch
		} else {
			release = ab.ReleaseEpoch
		}
		p := meter.NewProfile(ab.ToAddr, ab.Memo, ab.LockEpoch, release, ab.MeterAmount, ab.MeterGovAmount)
		pList.Add(p)
	}

	// already checked, so no need to check here
	if ab.MeterAmount.Sign() != 0 {
		state.SubEnergy(ab.FromAddr, ab.MeterAmount)
		state.AddEnergy(ab.ToAddr, ab.MeterAmount)
		env.AddTransfer(ab.FromAddr, ab.ToAddr, ab.MeterAmount, meter.MTR)
	}
	if ab.MeterGovAmount.Sign() != 0 {
		state.SubBalance(ab.FromAddr, ab.MeterGovAmount)
		state.AddBalance(ab.ToAddr, ab.MeterGovAmount)
		env.AddTransfer(ab.FromAddr, ab.ToAddr, ab.MeterGovAmount, meter.MTRG)
	}

	a.logger.Debug("account lock transfer", "from", ab.FromAddr, "to", ab.ToAddr, "meter", ab.MeterAmount, "meterGov", ab.MeterGovAmount)
	state.SetProfileList(pList)
	return
}
