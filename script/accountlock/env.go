// Copyright (c) 2020 The Meter.io developerslopers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package accountlock

import (
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/xenv"
)

//
type AccountLockEnviroment struct {
	AccountLock *AccountLock
	state       *state.State
	txCtx       *xenv.TransactionContext
	toAddr      *meter.Address
}

func NewAccountLockEnviroment(AccountLock *AccountLock, state *state.State, txCtx *xenv.TransactionContext, to *meter.Address) *AccountLockEnviroment {
	return &AccountLockEnviroment{
		AccountLock: AccountLock,
		state:       state,
		txCtx:       txCtx,
		toAddr:      to,
	}
}

func (env *AccountLockEnviroment) GetAccountLock() *AccountLock       { return env.AccountLock }
func (env *AccountLockEnviroment) GetState() *state.State             { return env.state }
func (env *AccountLockEnviroment) GetTxCtx() *xenv.TransactionContext { return env.txCtx }
func (env *AccountLockEnviroment) GetToAddr() *meter.Address          { return env.toAddr }
