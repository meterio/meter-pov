// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package accountlock

import (
	"errors"

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

func (a *AccountLock) Handle(senv *setypes.ScriptEnv, payload []byte, to *meter.Address, gas uint64) (seOutput *setypes.ScriptEngineOutput, leftOverGas uint64, err error) {

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
		if to.String() != meter.AccountLockModuleAddr.String() {
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
