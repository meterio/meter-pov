// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package accountlock

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/rlp"
)

// Candidate indicates the structure of a candidate
type AccountLockBody struct {
	Opcode         uint32
	Version        uint32
	Option         uint32
	LockEpoch      uint32
	ReleaseEpoch   uint32
	FromAddr       meter.Address
	ToAddr         meter.Address
	MeterAmount    *big.Int
	MeterGovAmount *big.Int
	Memo           []byte
}

func (a *AccountLockBody) ToString() string {
	return fmt.Sprintf("AccountLockBody: Opcode=%v, Version=%v, Option=%v, LockedEpoch=%v, ReleaseEpoch=%v, FromAddr=%v, ToAddr=%v, MeterAmount=%v, MeterGovAmount=%v, Memo=%v",
		a.Opcode, a.Version, a.Option, a.LockEpoch, a.ReleaseEpoch, a.FromAddr, a.ToAddr, a.MeterAmount.String(), a.MeterGovAmount.String(), string(a.Memo))
}

func (a *AccountLockBody) GetOpName(op uint32) string {
	switch op {
	case OP_ADDLOCK:
		return "addlock"
	case OP_REMOVELOCK:
		return "removelock"
	case OP_TRANSFER:
		return "transfer"
	case OP_GOVERNING:
		return "governing"
	default:
		return "Unknown"
	}
}

func AccountLockEncodeBytes(sb *AccountLockBody) []byte {
	AccountLockBytes, err := rlp.EncodeToBytes(sb)
	if err != nil {
		log.Error("rlp encode failed", "error", err)
		return []byte{}
	}
	return AccountLockBytes
}

func AccountLockDecodeFromBytes(bytes []byte) (*AccountLockBody, error) {
	ab := AccountLockBody{}
	err := rlp.DecodeBytes(bytes, &ab)
	return &ab, err
}

func (ab *AccountLockBody) HandleAccountLockAdd(env *AccountLockEnviroment, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	AccountLock := env.GetAccountLock()
	state := env.GetState()
	pList := AccountLock.GetProfileList(state)

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

	p := NewProfile(ab.FromAddr, ab.Memo, ab.LockEpoch, ab.ReleaseEpoch, ab.MeterAmount, ab.MeterGovAmount)
	pList.Add(p)

	AccountLock.SetProfileList(pList, state)
	return
}

func (ab *AccountLockBody) HandleAccountLockRemove(env *AccountLockEnviroment, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	AccountLock := env.GetAccountLock()
	state := env.GetState()
	pList := AccountLock.GetProfileList(state)

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

	AccountLock.SetProfileList(pList, state)
	return
}

func (ab *AccountLockBody) HandleAccountLockTransfer(env *AccountLockEnviroment, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	AccountLock := env.GetAccountLock()
	state := env.GetState()
	pList := AccountLock.GetProfileList(state)

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
		p := NewProfile(ab.ToAddr, ab.Memo, ab.LockEpoch, ab.ReleaseEpoch, ab.MeterAmount, ab.MeterGovAmount)
		pList.Add(p)
	} else {
		var release uint32
		if ab.ReleaseEpoch < pFrom.ReleaseEpoch {
			release = pFrom.ReleaseEpoch
		} else {
			release = ab.ReleaseEpoch
		}
		p := NewProfile(ab.ToAddr, ab.Memo, ab.LockEpoch, release, ab.MeterAmount, ab.MeterGovAmount)
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
	AccountLock.SetProfileList(pList, state)
	return
}

func (ab *AccountLockBody) GoverningHandler(env *AccountLockEnviroment, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	AccountLock := env.GetAccountLock()
	state := env.GetState()
	pList := AccountLock.GetProfileList(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	toRemove := []meter.Address{}
	curEpoch := AccountLock.GetCurrentEpoch()
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
	AccountLock.SetProfileList(pList, state)
	return
}
