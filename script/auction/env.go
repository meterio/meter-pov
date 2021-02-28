// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auction

import (
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/tx"
	"github.com/dfinlab/meter/xenv"
)

//
type AuctionEnviroment struct {
	auction   *Auction
	state     *state.State
	txCtx     *xenv.TransactionContext
	toAddr    *meter.Address
	transfers []*tx.Transfer
	events    []*tx.Event
}

func NewAuctionEnviroment(auction *Auction, state *state.State, txCtx *xenv.TransactionContext, to *meter.Address) *AuctionEnviroment {
	return &AuctionEnviroment{
		auction:   auction,
		state:     state,
		txCtx:     txCtx,
		toAddr:    to,
		transfers: make([]*tx.Transfer, 0),
		events:    make([]*tx.Event, 0),
	}
}

func (env *AuctionEnviroment) GetAuction() *Auction               { return env.auction }
func (env *AuctionEnviroment) GetState() *state.State             { return env.state }
func (env *AuctionEnviroment) GetTxCtx() *xenv.TransactionContext { return env.txCtx }
func (env *AuctionEnviroment) GetToAddr() *meter.Address          { return env.toAddr }

func (env *AuctionEnviroment) AddTransfer(sender, recipient meter.Address, amount *big.Int, token byte) {
	env.transfers = append(env.transfers, &tx.Transfer{
		Sender:    sender,
		Recipient: recipient,
		Amount:    amount,
		Token:     token,
	})
}

func (env *AuctionEnviroment) AddEvent(address meter.Address, topics []meter.Bytes32, data []byte) {
	env.events = append(env.events, &tx.Event{
		Address: address,
		Topics:  topics,
		Data:    data,
	})
}

func (env *AuctionEnviroment) GetTranfers() tx.Transfers {
	return env.transfers
}

func (env *AuctionEnviroment) GetEvents() tx.Events {
	return env.events
}
