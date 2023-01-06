// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package meter

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
)

type AuctionTx struct {
	TxID      Bytes32
	Address   Address
	Amount    *big.Int // total amont wei is unit
	Type      uint32   // USER_BID or AUTO_BID
	Timestamp uint64   //timestamp
	Nonce     uint64   //randomness
}

func (a *AuctionTx) ToString() string {
	return fmt.Sprintf("AuctionTx(addr=%v, amount=%v, type=%v, nonce=%v, Time=%v)",
		a.Address, a.Amount.String(), a.Type, a.Nonce, fmt.Sprintln(time.Unix(int64(a.Timestamp), 0)))
}

func (a *AuctionTx) ID() (hash Bytes32) {
	hw := NewBlake2b()
	err := rlp.Encode(hw, []interface{}{
		a.Address,
		a.Amount,
		a.Type,
		a.Timestamp,
		a.Nonce,
	})
	if err != nil {
		fmt.Printf("rlp encode failed, %s.\n", err.Error())
		return Bytes32{}
	}
	hw.Sum(hash[:0])
	return
}

func NewAuctionTx(addr Address, amount *big.Int, txtype uint32, time uint64, nonce uint64) *AuctionTx {
	tx := &AuctionTx{
		Address:   addr,
		Amount:    amount,
		Type:      txtype,
		Timestamp: time,
		Nonce:     nonce,
	}
	tx.TxID = tx.ID()
	return tx
}
