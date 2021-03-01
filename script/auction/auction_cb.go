// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auction

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/rlp"
)

type AuctionTx struct {
	TxID      meter.Bytes32
	Address   meter.Address
	Amount    *big.Int // total amont wei is unit
	Type      uint32   // USER_BID or AUTO_BID
	Timestamp uint64   //timestamp
	Nonce     uint64   //randomness
}

func (a *AuctionTx) ToString() string {
	return fmt.Sprintf("AuctionTx(addr=%v, amount=%v, type=%v, nonce=%v, Time=%v)",
		a.Address, a.Amount.String(), a.Type, a.Nonce, fmt.Sprintln(time.Unix(int64(a.Timestamp), 0)))
}

func (a *AuctionTx) ID() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	err := rlp.Encode(hw, []interface{}{
		a.Address,
		a.Amount,
		a.Type,
		a.Timestamp,
		a.Nonce,
	})
	if err != nil {
		fmt.Printf("rlp encode failed, %s.\n", err.Error())
		return meter.Bytes32{}
	}
	hw.Sum(hash[:0])
	return
}

func NewAuctionTx(addr meter.Address, amount *big.Int, txtype uint32, time uint64, nonce uint64) *AuctionTx {
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

///////////////////////////////////////////////////
// auctionTx indicates the structure of a auctionTx
type AuctionCB struct {
	AuctionID   meter.Bytes32
	StartHeight uint64
	StartEpoch  uint64
	EndHeight   uint64
	EndEpoch    uint64
	Sequence    uint64
	RlsdMTRG    *big.Int //released mtrg
	RsvdMTRG    *big.Int // reserved mtrg
	RsvdPrice   *big.Int
	CreateTime  uint64

	//changed fields after auction start
	RcvdMTR    *big.Int
	AuctionTxs []*AuctionTx
}

//bucketID auctionTx .. are excluded
func (cb *AuctionCB) ID() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	err := rlp.Encode(hw, []interface{}{
		cb.StartHeight,
		cb.StartEpoch,
		cb.EndHeight,
		cb.EndEpoch,
		cb.RlsdMTRG,
		cb.RsvdMTRG,
		cb.RsvdPrice,
		cb.CreateTime,
	})
	if err != nil {
		fmt.Printf("rlp encode failed, %s.\n", err.Error())
		return meter.Bytes32{}
	}
	hw.Sum(hash[:0])
	return
}

func (cb *AuctionCB) AddAuctionTx(tx *AuctionTx) error {
	if cb.Get(tx.TxID) != nil {
		return errors.New("tx already exist")
	}

	cb.RcvdMTR = cb.RcvdMTR.Add(cb.RcvdMTR, tx.Amount)
	cb.Add(tx)
	return nil
}

// Actually Auction do not allow to cancel, so this func should not be called
func (cb *AuctionCB) RemoveAuctionTx(tx *AuctionTx) error {
	if cb.Get(tx.TxID) == nil {
		return errors.New("tx does not exist")
	}

	cb.RcvdMTR = cb.RcvdMTR.Sub(cb.RcvdMTR, tx.Amount)
	cb.Remove(tx.TxID)
	return nil
}

func (cb *AuctionCB) indexOf(id meter.Bytes32) (int, int) {
	// return values:
	//     first parameter: if found, the index of the item
	//     second parameter: if not found, the correct insert index of the item
	if len(cb.AuctionTxs) <= 0 {
		return -1, 0
	}
	l := 0
	r := len(cb.AuctionTxs)
	for l < r {
		m := (l + r) / 2
		cmp := bytes.Compare(id.Bytes(), cb.AuctionTxs[m].TxID.Bytes())
		if cmp < 0 {
			r = m
		} else if cmp > 0 {
			l = m + 1
		} else {
			return m, -1
		}
	}
	return -1, r
}

func (cb *AuctionCB) Get(id meter.Bytes32) *AuctionTx {
	index, _ := cb.indexOf(id)
	if index < 0 {
		return nil
	}
	return cb.AuctionTxs[index]
}

func (cb *AuctionCB) Exist(id meter.Bytes32) bool {
	index, _ := cb.indexOf(id)
	return index >= 0
}

func (cb *AuctionCB) Add(tx *AuctionTx) {
	index, insertIndex := cb.indexOf(tx.TxID)
	if index < 0 {
		if len(cb.AuctionTxs) == 0 {
			cb.AuctionTxs = append(cb.AuctionTxs, tx)
			return
		}
		newList := make([]*AuctionTx, insertIndex)
		copy(newList, cb.AuctionTxs[:insertIndex])
		newList = append(newList, tx)
		newList = append(newList, cb.AuctionTxs[insertIndex:]...)
		cb.AuctionTxs = newList
	} else {
		cb.AuctionTxs[index] = tx
	}

	return
}

func (cb *AuctionCB) Remove(id meter.Bytes32) {
	index, _ := cb.indexOf(id)
	if index >= 0 {
		cb.AuctionTxs = append(cb.AuctionTxs[:index], cb.AuctionTxs[index+1:]...)
	}
	return
}

func (cb *AuctionCB) Count() int {
	return len(cb.AuctionTxs)
}

func (cb *AuctionCB) ToString() string {
	if cb == nil || len(cb.AuctionTxs) == 0 {
		return "AuctionCB (size:0)"
	}
	s := []string{fmt.Sprintf("AuctionCB(ID=%v, StartHeight=%v, StartEpoch=%v, EndHeight=%v, EndEpoch=%v, Sequence=%v, RlsdMTRG:%v, RsvdMRTG:%v, RsvdPrice:%v, RcvdMTR:%v, CreateTime:%v)",
		cb.AuctionID, cb.StartHeight, cb.StartEpoch, cb.EndHeight, cb.EndEpoch, cb.Sequence, cb.RlsdMTRG, cb.RsvdMTRG,
		cb.RsvdPrice, cb.RcvdMTR, cb.CreateTime)}
	for i, c := range cb.AuctionTxs {
		s = append(s, fmt.Sprintf("  %d.%v", i, c.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}

func (cb *AuctionCB) IsActive() bool {
	return !cb.AuctionID.IsZero()
}

func (cb *AuctionCB) ToTxList() []*AuctionTx {
	result := make([]*AuctionTx, 0)
	for _, v := range cb.AuctionTxs {
		result = append(result, v)
	}
	return result
}
