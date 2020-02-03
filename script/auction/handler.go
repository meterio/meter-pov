package auction

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	OP_START = uint32(1)
	OP_STOP  = uint32(2)
	OP_BID   = uint32(3)
)

var (
	AuctionReservedPrice = big.NewInt(100) // at least  1 MTRG settle down 100 MTR
)

// Candidate indicates the structure of a candidate
type AuctionBody struct {
	Opcode      uint32
	Version     uint32
	Option      uint32
	StartHeight uint64
	EndHeight   uint64
	AuctionID   meter.Bytes32
	Bidder      meter.Address
	Amount      *big.Int
	Token       byte   // meter or meter gov
	Timestamp   uint64 // timestamp
	Nonce       uint64 // nonce
}

func (ab *AuctionBody) ToString() string {
	return fmt.Sprintf("AuctionBody: Opcode=%v, Version=%v, Option=%v, StartHegiht=%v, EndHeight=%v, AuctionID=%v, Bidder=%v, Amount=%v, Token=%v, TimeStamp=%v, Nonce=%v",
		ab.Opcode, ab.Version, ab.Option, ab.StartHeight, ab.EndHeight, ab.AuctionID.AbbrevString(), ab.Bidder.String(), ab.Amount.String(), ab.Token, ab.Timestamp, ab.Nonce)
}

func (ab *AuctionBody) GetOpName(op uint32) string {
	switch op {
	case OP_START:
		return "Start"
	case OP_STOP:
		return "Stop"
	case OP_BID:
		return "Bid"
	default:
		return "Unknown"
	}
}

func AuctionEncodeBytes(sb *AuctionBody) []byte {
	auctionBytes, _ := rlp.EncodeToBytes(sb)
	return auctionBytes
}

func AuctionDecodeFromBytes(bytes []byte) (*AuctionBody, error) {
	ab := AuctionBody{}
	err := rlp.DecodeBytes(bytes, &ab)
	return &ab, err
}

func (ab *AuctionBody) StartAuctionCB(env *AuctionEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
	Auction := env.GetAuction()
	state := env.GetState()
	auctionCB := Auction.GetAuctionCB(state)
	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}
	release, _, _ := calcRewardRange(ab.StartHeight, ab.EndHeight)

	auctionCB.startHeight = ab.StartHeight
	auctionCB.endHeight = ab.EndHeight
	auctionCB.rlsdMTRG = FloatToBigInt(release)
	auctionCB.rsvdPrice = AuctionReservedPrice
	auctionCB.createTime = ab.Timestamp
	auctionCB.rcvdMTR = big.NewInt(0)
	auctionCB.auctionTxs = make([]*AuctionTx, 0)
	auctionCB.auctionID = auctionCB.ID()

	Auction.SetAuctionCB(auctionCB, state)
	return
}

func (ab *AuctionBody) CloseAuctionCB(senv *AuctionEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
	Auction := senv.GetAuction()
	state := senv.GetState()
	summaryList := Auction.GetSummaryList(state)
	auctionCB := Auction.GetAuctionCB(state)
	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	// clear the auction
	actualPrice, err := Auction.ClearAuction(auctionCB, state)
	if err != nil {
		return
	}
	summary := &AuctionSummary{
		auctionID:   auctionCB.auctionID,
		startHeight: auctionCB.startHeight,
		endHeight:   auctionCB.endHeight,
		rlsdMTRG:    auctionCB.rlsdMTRG,
		rsvdPrice:   auctionCB.rsvdPrice,
		createTime:  auctionCB.createTime,
		rcvdMTR:     auctionCB.rcvdMTR,
		actualPrice: actualPrice,
	}
	summaries := append(summaryList.summaries, summary)

	summaryList = NewAuctionSummaryList(summaries)
	auctionCB = &AuctionCB{}
	Auction.SetSummaryList(summaryList, state)
	Auction.SetAuctionCB(auctionCB, state)
	return
}

func (ab *AuctionBody) HandleAuctionTx(senv *AuctionEnviroment, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
	}()
	Auction := senv.GetAuction()
	state := senv.GetState()
	auctionCB := Auction.GetAuctionCB(state)

	if state.GetEnergy(ab.Bidder).Cmp(ab.Amount) < 0 {
		err = errors.New("not enough meter balance")
	}

	tx := auctionCB.Get(ab.Bidder)
	if tx == nil {
		tx = &AuctionTx{
			addr:     ab.Bidder,
			amount:   ab.Amount,
			count:    1,
			nonce:    ab.Nonce,
			lastTime: ab.Timestamp,
		}
		err = auctionCB.Add(tx)
		if err != nil {
			fmt.Println("add auctionTx failed")
			return
		}
		auctionCB.rcvdMTR = auctionCB.rcvdMTR.Add(auctionCB.rcvdMTR, tx.amount)
	} else {
		if ab.Nonce <= tx.nonce {
			err = errors.New("Nonce error")
			return
		}
		tx.nonce = ab.Nonce
		tx.amount = tx.amount.Add(tx.amount, ab.Amount)
		tx.lastTime = ab.Timestamp
		tx.count++
	}

	// transfer bidder's MTR to auction accout
	Auction.TransferMTRToAuction(ab.Bidder, ab.Amount, state)
	Auction.SetAuctionCB(auctionCB, state)
	return
}
