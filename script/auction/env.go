// Copyright (c) 2020 The Meter.io developerslopers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auction

import (
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/xenv"
)

//
type AuctionEnviroment struct {
	auction *Auction
	state   *state.State
	txCtx   *xenv.TransactionContext
	toAddr  *meter.Address
}

func NewAuctionEnviroment(auction *Auction, state *state.State, txCtx *xenv.TransactionContext, to *meter.Address) *AuctionEnviroment {
	return &AuctionEnviroment{
		auction: auction,
		state:   state,
		txCtx:   txCtx,
		toAddr:  to,
	}
}

func (env *AuctionEnviroment) GetAuction() *Auction               { return env.auction }
func (env *AuctionEnviroment) GetState() *state.State             { return env.state }
func (env *AuctionEnviroment) GetTxCtx() *xenv.TransactionContext { return env.txCtx }
func (env *AuctionEnviroment) GetToAddr() *meter.Address          { return env.toAddr }
