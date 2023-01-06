// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package accountlock

import (
	"errors"
	"math/big"

	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
	"github.com/meterio/meter-pov/state"
)

var (
	AccountLockGlobInst *AccountLock
	log                 = log15.New("pkg", "accountlock")
)

// Candidate indicates the structure of a candidate
type AccountLock struct {
	chain        *chain.Chain
	stateCreator *state.Creator
	logger       log15.Logger
}

func GetAccountLockGlobInst() *AccountLock {
	return AccountLockGlobInst
}

func SetAccountLockGlobInst(inst *AccountLock) {
	AccountLockGlobInst = inst
}

func NewAccountLock(ch *chain.Chain, sc *state.Creator) *AccountLock {
	AccountLock := &AccountLock{
		chain:        ch,
		stateCreator: sc,
		logger:       log15.New("pkg", "AccountLock"),
	}
	SetAccountLockGlobInst(AccountLock)
	return AccountLock
}

func (a *AccountLock) AccountLockHandler(senv *setypes.ScriptEnv, payload []byte, to *meter.Address, gas uint64) (seOutput *setypes.ScriptEngineOutput, leftOverGas uint64, err error) {

	ab, err := AccountLockDecodeFromBytes(payload)
	if err != nil {
		log.Error("Decode script message failed", "error", err)
		return nil, gas, err
	}

	if senv == nil {
		panic("create AccountLock enviroment failed")
	}

	log.Debug("received AccountLock", "body", ab.ToString())
	log.Debug("Entering accountLock handler "+ab.GetOpName(ab.Opcode), "tx", senv.GetTxHash())
	switch ab.Opcode {
	case OP_ADDLOCK:
		if senv.GetTxOrigin().IsZero() == false {
			return nil, gas, errors.New("not from kblock")
		}
		leftOverGas, err = a.HandleAccountLockAdd(senv, ab, gas)

	case OP_REMOVELOCK:
		if senv.GetTxOrigin().IsZero() == false {
			return nil, gas, errors.New("not form kblock")
		}
		leftOverGas, err = a.HandleAccountLockRemove(senv, ab, gas)

	case OP_TRANSFER:
		if senv.GetTxOrigin() != ab.FromAddr {
			return nil, gas, errors.New("from address is not the same from transaction")
		}
		leftOverGas, err = a.HandleAccountLockTransfer(senv, ab, gas)

	case OP_GOVERNING:
		if to.String() != meter.AccountLockAddr.String() {
			return nil, gas, errors.New("to address is not the same from module address")
		}
		leftOverGas, err = a.GoverningHandler(senv, ab, gas)

	default:
		log.Error("unknown Opcode", "Opcode", ab.Opcode)
		return nil, gas, errors.New("unknow AccountLock opcode")
	}
	log.Debug("Leaving script handler for operation", "op", ab.GetOpName(ab.Opcode))

	seOutput = senv.GetOutput()
	return
}

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
		log.Error("profile is already in state", "addr", ab.FromAddr)
		return
	}

	p := meter.NewProfile(ab.FromAddr, ab.Memo, ab.LockEpoch, ab.ReleaseEpoch, ab.MeterAmount, ab.MeterGovAmount)
	pList.Add(p)

	state.SetProfileList(pList)
	return
}

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
		log.Error("profile is already in state", "addr", ab.ToAddr)
		return
	}

	// from address should not have account lock, ONLY some exceptional addresses
	pFrom := pList.Get(ab.FromAddr)
	if pFrom != nil {
		if state.IsExclusiveAccount(ab.FromAddr) == false {
			err = errors.New("profile of FromAddr is already in state")
			log.Error("profile is already in state", "addr", ab.FromAddr)
			return
		}
		log.Info("profile is in state but is exceptional address, continue ...", "addr", ab.FromAddr)
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

	log.Debug("account lock transfer", "from", ab.FromAddr, "to", ab.ToAddr, "meter", ab.MeterAmount, "meterGov", ab.MeterGovAmount)
	state.SetProfileList(pList)
	return
}

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

	log.Debug("account lock governing done...", "epoch", curEpoch)
	state.SetProfileList(pList)
	return
}

// api routine interface
func GetLatestProfileList() (*meter.ProfileList, error) {
	accountlock := GetAccountLockGlobInst()
	if accountlock == nil {
		log.Warn("accountlock is not initialized...")
		err := errors.New("accountlock is not initialized...")
		return meter.NewProfileList(nil), err
	}

	best := accountlock.chain.BestBlock()
	state, err := accountlock.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return meter.NewProfileList(nil), err
	}

	list := state.GetProfileList()
	return list, nil
}

func RestrictByAccountLock(addr meter.Address, state *state.State) (bool, *big.Int, *big.Int) {
	accountlock := GetAccountLockGlobInst()
	if accountlock == nil {
		//log.Debug("accountlock is not initialized...")
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
