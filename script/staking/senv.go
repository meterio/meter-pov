// Copyright (c) 2020 The Meter.io developerslopers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/xenv"
)

//
type StakingEnviroment struct {
	staking *Staking
	state   *state.State
	txCtx   *xenv.TransactionContext
	toAddr  *meter.Address
}

func NewStakingEnviroment(staking *Staking, state *state.State, txCtx *xenv.TransactionContext, to *meter.Address) *StakingEnviroment {
	return &StakingEnviroment{
		staking: staking,
		state:   state,
		txCtx:   txCtx,
		toAddr:  to,
	}
}

func (senv *StakingEnviroment) GetStaking() *Staking               { return senv.staking }
func (senv *StakingEnviroment) GetState() *state.State             { return senv.state }
func (senv *StakingEnviroment) GetTxCtx() *xenv.TransactionContext { return senv.txCtx }
func (senv *StakingEnviroment) GetToAddr() *meter.Address          { return senv.toAddr }
