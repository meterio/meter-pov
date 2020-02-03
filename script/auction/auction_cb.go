package auction

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/rlp"
)

type AuctionTx struct {
	addr     meter.Address
	amount   *big.Int // total amont wei is unit
	count    int
	nonce    uint64
	lastTime uint64 //last auction time
}

func (a *AuctionTx) ToString() string {
	return fmt.Sprintf("AuctionTx(addr=%v, amount=%v%.2e, count=%v, nonce=%v, lastTime=%v)",
		a.addr, a.amount.Uint64(), a.count, a.nonce, fmt.Sprintln(time.Unix(int64(a.lastTime), 0)))
}

// auctionTx indicates the structure of a auctionTx
type AuctionCB struct {
	auctionID   meter.Bytes32
	startHeight uint64
	endHeight   uint64
	rlsdMTRG    *big.Int
	rsvdPrice   *big.Int
	createTime  uint64

	//changed fields after auction start
	rcvdMTR    *big.Int
	auctionTxs []*AuctionTx
}

//bucketID auctionTx .. are excluded
func (cb *AuctionCB) ID() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		cb.startHeight,
		cb.endHeight,
		cb.rlsdMTRG,
		cb.rsvdPrice,
		cb.createTime,
	})
	hw.Sum(hash[:0])
	return
}

func (cb *AuctionCB) AddAuctionTx(tx *AuctionTx) {
	cb.rcvdMTR = cb.rcvdMTR.Add(cb.rcvdMTR, tx.amount)
	cb.auctionTxs = append(cb.auctionTxs, tx)
}

func (cb *AuctionCB) indexOf(addr meter.Address) (int, int) {
	// return values:
	//     first parameter: if found, the index of the item
	//     second parameter: if not found, the correct insert index of the item
	if len(cb.auctionTxs) <= 0 {
		return -1, 0
	}
	l := 0
	r := len(cb.auctionTxs)
	for l < r {
		m := (l + r) / 2
		cmp := bytes.Compare(addr.Bytes(), cb.auctionTxs[m].addr.Bytes())
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

func (cb *AuctionCB) Get(addr meter.Address) *AuctionTx {
	index, _ := cb.indexOf(addr)
	if index < 0 {
		return nil
	}
	return cb.auctionTxs[index]
}

func (cb *AuctionCB) Exist(addr meter.Address) bool {
	index, _ := cb.indexOf(addr)
	return index >= 0
}

func (cb *AuctionCB) Add(c *AuctionTx) error {
	index, insertIndex := cb.indexOf(c.addr)
	if index < 0 {
		if len(cb.auctionTxs) == 0 {
			cb.auctionTxs = append(cb.auctionTxs, c)
			return nil
		}
		newList := make([]*AuctionTx, insertIndex)
		copy(newList, cb.auctionTxs[:insertIndex])
		newList = append(newList, c)
		newList = append(newList, cb.auctionTxs[insertIndex:]...)
		cb.auctionTxs = newList
	} else {
		cb.auctionTxs[index] = c
	}

	return nil
}

func (cb *AuctionCB) Remove(addr meter.Address) error {
	index, _ := cb.indexOf(addr)
	if index >= 0 {
		cb.auctionTxs = append(cb.auctionTxs[:index], cb.auctionTxs[index+1:]...)
	}
	return nil
}

func (cb *AuctionCB) Count() int {
	return len(cb.auctionTxs)
}

func (cb *AuctionCB) ToString() string {
	if cb == nil || len(cb.auctionTxs) == 0 {
		return "AuctionCB (size:0)"
	}
	s := []string{fmt.Sprintf("AuctionCB(ID=%v, startHeight=%v, EndHeight=%v, rlsdMTRG:%.2e, rsvdPrice:%.2e, rcvdMTR:%.2e, createTime:%v)",
		cb.auctionID, cb.startHeight, cb.endHeight, float64(cb.rlsdMTRG.Int64()), cb.rsvdPrice,
		float64(cb.rcvdMTR.Int64()), fmt.Sprintln(time.Unix(int64(cb.createTime), 0)))}
	for i, c := range cb.auctionTxs {
		s = append(s, fmt.Sprintf("  %d.%v", i, c.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}

func (cb *AuctionCB) ToList() []AuctionTx {
	result := make([]AuctionTx, 0)
	for _, v := range cb.auctionTxs {
		result = append(result, *v)
	}
	return result
}

func (cb *AuctionCB) IsActive() bool {
	return !cb.auctionID.IsZero()
}
