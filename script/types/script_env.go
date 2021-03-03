// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package types

import (
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/tx"
	"github.com/dfinlab/meter/xenv"
)

//
type ScriptEnv struct {
	state  *state.State
	txCtx  *xenv.TransactionContext
	toAddr *meter.Address

	returnData []byte
	transfers  []*tx.Transfer
	events     []*tx.Event
}

func NewScriptEnv(state *state.State, txCtx *xenv.TransactionContext, to *meter.Address) *ScriptEnv {
	return &ScriptEnv{
		state:      state,
		txCtx:      txCtx,
		toAddr:     to,
		returnData: make([]byte, 0),
		transfers:  make([]*tx.Transfer, 0),
		events:     make([]*tx.Event, 0),
	}
}

func (env *ScriptEnv) GetState() *state.State             { return env.state }
func (env *ScriptEnv) GetTxCtx() *xenv.TransactionContext { return env.txCtx }
func (env *ScriptEnv) GetToAddr() *meter.Address          { return env.toAddr }

func (env *ScriptEnv) SetReturnData(data []byte) {
	env.returnData = data
}
func (env *ScriptEnv) GetReturnData() []byte {
	if env.returnData == nil || len(env.returnData) <= 0 {
		return nil
	}
	return env.returnData
}

func (env *ScriptEnv) AddTransfer(sender, recipient meter.Address, amount *big.Int, token byte) {
	env.transfers = append(env.transfers, &tx.Transfer{
		Sender:    sender,
		Recipient: recipient,
		Amount:    amount,
		Token:     token,
	})
}

func (env *ScriptEnv) AddEvent(address meter.Address, topics []meter.Bytes32, data []byte) {
	env.events = append(env.events, &tx.Event{
		Address: address,
		Topics:  topics,
		Data:    data,
	})
}

func (env *ScriptEnv) GetTransfers() tx.Transfers {
	return env.transfers
}

func (env *ScriptEnv) GetEvents() tx.Events {
	return env.events
}

func (env *ScriptEnv) GetOutput() *ScriptEngineOutput {
	return &ScriptEngineOutput{
		data:      env.GetReturnData(),
		transfers: env.transfers,
		events:    env.events,
	}
}
