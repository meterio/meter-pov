package accountlock

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	OP_ADDLOCK    = uint32(1)
	OP_REMOVELOCK = uint32(2)
	OP_TRANSFER   = uint32(3)
	OP_GOVERNING  = uint32(100)

	TOKEN_METER     = byte(0)
	TOKEN_METER_GOV = byte(1)
)

var (
	MinimumTransferAmount = new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18))
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

func (ab *AccountLockBody) HandleAccountLockAdd(env *AccountLockEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
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

func (ab *AccountLockBody) HandleAccountLockRemove(env *AccountLockEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
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

func (ab *AccountLockBody) HandleAccountLockTransfer(env *AccountLockEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
	AccountLock := env.GetAccountLock()
	state := env.GetState()
	pList := AccountLock.GetProfileList(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	if pTo := pList.Get(ab.ToAddr); pTo != nil {
		err = errors.New("profile of ToAddr is already in state")
		log.Error("profile is already in state", "addr", ab.ToAddr)
		return
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
	pFrom := pList.Get(ab.FromAddr)
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
		state.AddEnergy(ab.ToAddr, ab.MeterAmount)
		state.SubEnergy(ab.FromAddr, ab.MeterAmount)
	}
	if ab.MeterGovAmount.Sign() != 0 {
		state.AddBalance(ab.ToAddr, ab.MeterGovAmount)
		state.SubBalance(ab.FromAddr, ab.MeterGovAmount)
	}

	log.Debug("account lock transfer", "from", ab.FromAddr, "to", ab.ToAddr, "meter", ab.MeterAmount, "meterGov", ab.MeterGovAmount)
	AccountLock.SetProfileList(pList, state)
	return
}

func (ab *AccountLockBody) GoverningHandler(env *AccountLockEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
	AccountLock := env.GetAccountLock()
	state := env.GetState()
	pList := AccountLock.GetProfileList(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	curEpoch := AccountLock.GetCurrentEpoch()
	for _, p := range pList.Profiles {
		if p.ReleaseEpoch >= curEpoch {
			pList.Remove(p.Addr)
		}
	}

	log.Debug("account lock governing done...", "epoch", curEpoch)
	AccountLock.SetProfileList(pList, state)
	return
}
