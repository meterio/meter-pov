// Copyright (c) 2020 The Meter.io developerslopers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package accountlock

import (
	"bytes"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
)

func (a *AccountLock) GetCurrentEpoch() uint32 {
	bestBlock := a.chain.BestBlock()
	return uint32(bestBlock.GetBlockEpoch())
}

func (a *AccountLock) IsExclusiveAccount(addr meter.Address, state *state.State) bool {
	// executor account
	executor := meter.BytesToAddress(builtin.Params.Native(state).Get(meter.KeyExecutorAddress).Bytes())
	if bytes.Compare(addr.Bytes(), executor.Bytes()) == 0 {
		return true
	}

	// DFL Accounts
	if bytes.Compare(addr.Bytes(), meter.InitialDFLTeamAccount1.Bytes()) == 0 {
		return true
	}
	if bytes.Compare(addr.Bytes(), meter.InitialDFLTeamAccount2.Bytes()) == 0 {
		return true
	}
	if bytes.Compare(addr.Bytes(), meter.InitialDFLTeamAccount3.Bytes()) == 0 {
		return true
	}
	if bytes.Compare(addr.Bytes(), meter.InitialDFLTeamAccount4.Bytes()) == 0 {
		return true
	}
	if bytes.Compare(addr.Bytes(), meter.InitialDFLTeamAccount5.Bytes()) == 0 {
		return true
	}
	if bytes.Compare(addr.Bytes(), meter.InitialDFLTeamAccount6.Bytes()) == 0 {
		return true
	}
	if bytes.Compare(addr.Bytes(), meter.InitialDFLTeamAccount7.Bytes()) == 0 {
		return true
	}
	if bytes.Compare(addr.Bytes(), meter.InitialDFLTeamAccount8.Bytes()) == 0 {
		return true
	}

	return false
}
