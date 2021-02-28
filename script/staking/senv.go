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
type StakingEnviroment struct {
	staking   *Staking
	state     *state.State
	txCtx     *xenv.TransactionContext
	toAddr    *meter.Address
	transfers []*tx.Transfer
	events    []*tx.Event
}

func NewStakingEnviroment(staking *Staking, state *state.State, txCtx *xenv.TransactionContext, to *meter.Address) *StakingEnviroment {
	return &StakingEnviroment{
		staking:   staking,
		state:     state,
		txCtx:     txCtx,
		toAddr:    to,
		transfers: make([]*tx.Transfer, 0),
		events:    make([]*tx.Event, 0),
	}
}

func (senv *StakingEnviroment) GetStaking() *Staking               { return senv.staking }
func (senv *StakingEnviroment) GetState() *state.State             { return senv.state }
func (senv *StakingEnviroment) GetTxCtx() *xenv.TransactionContext { return senv.txCtx }
func (senv *StakingEnviroment) GetToAddr() *meter.Address          { return senv.toAddr }

func (senv *StakingEnviroment) AddTransfer(sender, recipient meter.Address, amount *big.Int, token byte) {
	senv.transfers = append(senv.transfers, &tx.Transfer{
		Sender:    sender,
		Recipient: recipient,
		Amount:    amount,
		Token:     token,
	})
}

func (senv *StakingEnviroment) AddEvent(address meter.Address, topics []meter.Bytes32, data []byte) {
	senv.events = append(senv.events, &tx.Event{
		Address: address,
		Topics:  topics,
		Data:    data,
	})
}

func (senv *StakingEnviroment) GetTranfers() tx.Transfers {
	return senv.transfers
}

func (senv *StakingEnviroment) GetEvents() tx.Events {
	return senv.events
}
