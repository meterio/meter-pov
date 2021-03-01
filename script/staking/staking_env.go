// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/tx"
	"github.com/dfinlab/meter/xenv"
)

//
type StakingEnv struct {
	staking   *Staking
	state     *state.State
	txCtx     *xenv.TransactionContext
	toAddr    *meter.Address
	transfers []*tx.Transfer
	events    []*tx.Event
}

func NewStakingEnv(staking *Staking, state *state.State, txCtx *xenv.TransactionContext, to *meter.Address) *StakingEnv {
	return &StakingEnv{
		staking:   staking,
		state:     state,
		txCtx:     txCtx,
		toAddr:    to,
		transfers: make([]*tx.Transfer, 0),
		events:    make([]*tx.Event, 0),
	}
}

func (env *StakingEnv) GetStaking() *Staking               { return env.staking }
func (env *StakingEnv) GetState() *state.State             { return env.state }
func (env *StakingEnv) GetTxCtx() *xenv.TransactionContext { return env.txCtx }
func (env *StakingEnv) GetToAddr() *meter.Address          { return env.toAddr }

func (env *StakingEnv) AddTransfer(sender, recipient meter.Address, amount *big.Int, token byte) {
	env.transfers = append(env.transfers, &tx.Transfer{
		Sender:    sender,
		Recipient: recipient,
		Amount:    amount,
		Token:     token,
	})
}

func (env *StakingEnv) AddEvent(address meter.Address, topics []meter.Bytes32, data []byte) {
	env.events = append(env.events, &tx.Event{
		Address: address,
		Topics:  topics,
		Data:    data,
	})
}

func (env *StakingEnv) GetTransfers() tx.Transfers {
	return env.transfers
}

func (env *StakingEnv) GetEvents() tx.Events {
	return env.events
}
