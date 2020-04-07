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
	Addr     meter.Address
	Amount   *big.Int // total amont wei is unit
	Count    int
	Nonce    uint64
	LastTime uint64 //last auction time
}

func (a *AuctionTx) ToString() string {
	return fmt.Sprintf("AuctionTx(addr=%v, amount=%v, count=%v, nonce=%v, lastTime=%v)",
		a.Addr, a.Amount.Uint64(), a.Count, a.Nonce, fmt.Sprintln(time.Unix(int64(a.LastTime), 0)))
}

// auctionTx indicates the structure of a auctionTx
type AuctionCB struct {
	AuctionID   meter.Bytes32
	StartHeight uint64
	StartEpoch  uint64
	EndHeight   uint64
	EndEpoch    uint64
	RlsdMTRG    *big.Int
	RsvdPrice   *big.Int
	CreateTime  uint64

	//changed fields after auction start
	RcvdMTR    *big.Int
	AuctionTxs []*AuctionTx
}

//bucketID auctionTx .. are excluded
func (cb *AuctionCB) ID() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		cb.StartHeight,
		cb.StartEpoch,
		cb.EndHeight,
		cb.EndEpoch,
		cb.RlsdMTRG,
		cb.RsvdPrice,
		cb.CreateTime,
	})
	hw.Sum(hash[:0])
	return
}

func (cb *AuctionCB) AddAuctionTx(tx *AuctionTx) {
	cb.RcvdMTR = cb.RcvdMTR.Add(cb.RcvdMTR, tx.Amount)
	cb.AuctionTxs = append(cb.AuctionTxs, tx)
}

func (cb *AuctionCB) indexOf(addr meter.Address) (int, int) {
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
		cmp := bytes.Compare(addr.Bytes(), cb.AuctionTxs[m].Addr.Bytes())
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
	return cb.AuctionTxs[index]
}

func (cb *AuctionCB) Exist(addr meter.Address) bool {
	index, _ := cb.indexOf(addr)
	return index >= 0
}

func (cb *AuctionCB) Add(c *AuctionTx) error {
	index, insertIndex := cb.indexOf(c.Addr)
	if index < 0 {
		if len(cb.AuctionTxs) == 0 {
			cb.AuctionTxs = append(cb.AuctionTxs, c)
			return nil
		}
		newList := make([]*AuctionTx, insertIndex)
		copy(newList, cb.AuctionTxs[:insertIndex])
		newList = append(newList, c)
		newList = append(newList, cb.AuctionTxs[insertIndex:]...)
		cb.AuctionTxs = newList
	} else {
		cb.AuctionTxs[index] = c
	}

	return nil
}

func (cb *AuctionCB) Remove(addr meter.Address) error {
	index, _ := cb.indexOf(addr)
	if index >= 0 {
		cb.AuctionTxs = append(cb.AuctionTxs[:index], cb.AuctionTxs[index+1:]...)
	}
	return nil
}

func (cb *AuctionCB) Count() int {
	return len(cb.AuctionTxs)
}

func (cb *AuctionCB) ToString() string {
	if cb == nil || len(cb.AuctionTxs) == 0 {
		return "AuctionCB (size:0)"
	}
	s := []string{fmt.Sprintf("AuctionCB(ID=%v, StartHeight=%v, StartEpoch=%v, EndHeight=%v, EndEpoch=%v, RlsdMTRG:%.2e, RsvdPrice:%.2e, RcvdMTR:%.2e, CreateTime:%v)",
		cb.AuctionID, cb.StartHeight, cb.StartEpoch, cb.EndHeight, cb.EndEpoch, float64(cb.RlsdMTRG.Int64()), cb.RsvdPrice,
		float64(cb.RcvdMTR.Int64()), fmt.Sprintln(time.Unix(int64(cb.CreateTime), 0)))}
	for i, c := range cb.AuctionTxs {
		s = append(s, fmt.Sprintf("  %d.%v", i, c.ToString()))
	}
	s = append(s, "}")
	return strings.Join(s, "\n")
}

func (cb *AuctionCB) ToList() []AuctionTx {
	result := make([]AuctionTx, 0)
	for _, v := range cb.AuctionTxs {
		result = append(result, *v)
	}
	return result
}

func (cb *AuctionCB) IsActive() bool {
	return !cb.AuctionID.IsZero()
}

//  api routine interface
func GetActiveAuctionCB() (*AuctionCB, error) {
	auction := GetAuctionGlobInst()
	if auction == nil {
		log.Warn("auction is not initilized...")
		err := errors.New("auction is not initilized...")
		return nil, err
	}

	best := auction.chain.BestBlock()
	state, err := auction.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return nil, err
	}

	cb := auction.GetAuctionCB(state)
	return cb, nil
}
