// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package accountlock

import (
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	setypes "github.com/dfinlab/meter/script/types"
	"github.com/dfinlab/meter/xenv"
)

//
type AccountLockEnviroment struct {
	*setypes.ScriptEnv
	accountLock *AccountLock
}

func NewAccountLockEnviroment(accountLock *AccountLock, state *state.State, txCtx *xenv.TransactionContext, to *meter.Address) *AccountLockEnviroment {
	return &AccountLockEnviroment{
		accountLock: accountLock,
		ScriptEnv: setypes.NewScriptEnv(state, txCtx, to),
	}
}

func (env *AccountLockEnviroment) GetAccountLock() *AccountLock       { return env.accountLock }
