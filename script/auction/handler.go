package auction

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	OP_START = uint32(1)
	OP_STOP  = uint32(2)
	OP_BID   = uint32(3)
)

func GetOpName(op uint32) string {
	switch op {
	case OP_START:
		return "Start"
	case OP_BID:
		return "Bid"
	case OP_STOP:
		return "Stop"
	default:
		return "Unknown"
	}
}

var (
	MinimumBidAmount = new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18))
	// AuctionReservedPrice = big.NewInt(5e17) // at least  1 MTRG settle down 0.5 MTR
)

// Candidate indicates the structure of a candidate
type AuctionBody struct {
	Opcode      uint32
	Version     uint32
	Option      uint32
	StartHeight uint64
	StartEpoch  uint64
	EndHeight   uint64
	EndEpoch    uint64
	AuctionID   meter.Bytes32
	Bidder      meter.Address
	Amount      *big.Int
	Token       byte   // meter or meter gov
	Timestamp   uint64 // timestamp
	Nonce       uint64 // nonce
}

func (ab *AuctionBody) ToString() string {
	return fmt.Sprintf("AuctionBody: Opcode=%v, Version=%v, Option=%v, StartHegiht=%v, StartEpoch=%v, EndHeight=%v, EndEpoch=%v, AuctionID=%v, Bidder=%v, Amount=%v, Token=%v, TimeStamp=%v, Nonce=%v",
		ab.Opcode, ab.Version, ab.Option, ab.StartHeight, ab.StartEpoch, ab.EndHeight, ab.EndEpoch, ab.AuctionID.AbbrevString(), ab.Bidder.String(), ab.Amount.String(), ab.Token, ab.Timestamp, ab.Nonce)
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

var (
	errNotEnoughMTR         = errors.New("not enough MTR balance")
	errLessThanBidThreshold = errors.New("amount less than bid threshold (" + big.NewInt(0).Div(MinimumBidAmount, big.NewInt(1e18)).String() + " MTR)")
	errInvalidNonce         = errors.New("invalid nonce (nonce in auction body and clause are the same)")
)

func AuctionEncodeBytes(sb *AuctionBody) []byte {
	auctionBytes, err := rlp.EncodeToBytes(sb)
	if err != nil {
		log.Error("rlp encode failed", "error", err)
		return []byte{}
	}
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

	auctionCB.StartHeight = ab.StartHeight
	auctionCB.StartEpoch = ab.StartEpoch
	auctionCB.EndHeight = ab.EndHeight
	auctionCB.EndEpoch = ab.EndEpoch
	auctionCB.RlsdMTRG = ab.Amount
	auctionCB.RsvdPrice = builtin.Params.Native(state).Get(meter.KeyAuctionReservedPrice)
	auctionCB.CreateTime = ab.Timestamp
	auctionCB.RcvdMTR = big.NewInt(0)
	auctionCB.AuctionTxs = make([]*AuctionTx, 0)
	auctionCB.AuctionID = auctionCB.ID()

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
	actualPrice, leftover, err := Auction.ClearAuction(auctionCB, state)
	if err != nil {
		log.Info("clear active auction failed failed")
		return
	}

	summary := &AuctionSummary{
		AuctionID:    auctionCB.AuctionID,
		StartHeight:  auctionCB.StartHeight,
		StartEpoch:   auctionCB.StartEpoch,
		EndHeight:    auctionCB.EndHeight,
		EndEpoch:     auctionCB.EndEpoch,
		RlsdMTRG:     auctionCB.RlsdMTRG,
		RsvdPrice:    auctionCB.RsvdPrice,
		CreateTime:   auctionCB.CreateTime,
		RcvdMTR:      auctionCB.RcvdMTR,
		ActualPrice:  actualPrice,
		LeftoverMTRG: leftover,
	}
	summaries := append(summaryList.Summaries, summary)

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
		log.Info("not enough meter balance", "bidder", ab.Bidder, "amount", ab.Amount)
		err = errNotEnoughMTR
		return
	}

	if ab.Amount.Cmp(MinimumBidAmount) < 0 {
		log.Info("amount lower than minimum bid threshold", "amount", ab.Amount, "minBid", MinimumBidAmount)
		err = errLessThanBidThreshold
		return
	}

	tx := auctionCB.Get(ab.Bidder)
	if tx == nil {
		tx = &AuctionTx{
			Addr:     ab.Bidder,
			Amount:   ab.Amount,
			Count:    1,
			Nonce:    ab.Nonce,
			LastTime: ab.Timestamp,
		}
		err = auctionCB.Add(tx)
		if err != nil {
			log.Info("add auctionTx failed")
			return
		}
	} else {
		if ab.Nonce == tx.Nonce {
			log.Info("Nonce error", "input nonce", ab.Nonce, "nonce in tx", tx.Nonce)
			err = errInvalidNonce
			return
		}
		tx.Nonce = ab.Nonce
		tx.Amount = tx.Amount.Add(tx.Amount, ab.Amount)
		tx.LastTime = ab.Timestamp
		tx.Count++
	}

	// Now update the total amount
	auctionCB.RcvdMTR = auctionCB.RcvdMTR.Add(auctionCB.RcvdMTR, ab.Amount)

	// transfer bidder's MTR to auction accout
	err = Auction.TransferMTRToAuction(ab.Bidder, ab.Amount, state)
	if err != nil {
		log.Error("not enough balance", "address", ab.Bidder)
		err = errNotEnoughMTR
		return
	}

	Auction.SetAuctionCB(auctionCB, state)
	return
}
