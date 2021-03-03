// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package accountlock

import (
	"errors"

	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
	setypes "github.com/dfinlab/meter/script/types"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/xenv"
	"github.com/inconshreveable/log15"
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

func (a *AccountLock) Start() error {

	log.Info("AccountLock module started")
	return nil
}

func (a *AccountLock) PrepareAccountLockHandler() (AccountLockHandler func([]byte, *meter.Address, *xenv.TransactionContext, uint64, *state.State) (*setypes.ScriptEngineOutput, uint64, error)) {

	AccountLockHandler = func(data []byte, to *meter.Address, txCtx *xenv.TransactionContext, gas uint64, state *state.State) (seOutput *setypes.ScriptEngineOutput, leftOverGas uint64, err error) {

		ab, err := AccountLockDecodeFromBytes(data)
		if err != nil {
			log.Error("Decode script message failed", "error", err)
			return nil, gas, err
		}

		env := NewAccountLockEnviroment(a, state, txCtx, to)
		if env == nil {
			panic("create AccountLock enviroment failed")
		}

		log.Debug("received AccountLock", "body", ab.ToString())
		log.Info("Entering AccountLock handler " + ab.GetOpName(ab.Opcode))
		switch ab.Opcode {
		case OP_ADDLOCK:
			if env.GetTxCtx().Origin.IsZero() == false {
				return nil, gas, errors.New("not from kblock")
			}
			leftOverGas, err = ab.HandleAccountLockAdd(env, gas)

		case OP_REMOVELOCK:
			if env.GetTxCtx().Origin.IsZero() == false {
				return nil, gas, errors.New("not form kblock")
			}
			leftOverGas, err = ab.HandleAccountLockRemove(env, gas)

		case OP_TRANSFER:
			if env.GetTxCtx().Origin != ab.FromAddr {
				return nil, gas, errors.New("from address is not the same from transaction")
			}
			leftOverGas, err = ab.HandleAccountLockTransfer(env, gas)

		case OP_GOVERNING:
			if env.GetToAddr().String() != AccountLockAddr.String() {
				return nil, gas, errors.New("to address is not the same from module address")
			}
			leftOverGas, err = ab.GoverningHandler(env, gas)

		default:
			log.Error("unknown Opcode", "Opcode", ab.Opcode)
			return nil, gas, errors.New("unknow AccountLock opcode")
		}
		log.Debug("Leaving script handler for operation", "op", ab.GetOpName(ab.Opcode))

		seOutput = env.GetOutput()
		return
	}
	return
}
