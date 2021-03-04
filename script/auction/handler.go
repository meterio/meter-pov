// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auction

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	// normal min amount is 10 mtr, autobid is 0.1 mtr
	MinimumBidAmount = new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18))
	AutobidMinAmount = big.NewInt(1e17)
	// AuctionReservedPrice = big.NewInt(5e17) // at least  1 MTRG settle down 0.5 MTR
)

// Candidate indicates the structure of a candidate
type AuctionBody struct {
	Opcode        uint32
	Version       uint32
	Option        uint32
	StartHeight   uint64
	StartEpoch    uint64
	EndHeight     uint64
	EndEpoch      uint64
	Sequence      uint64
	AuctionID     meter.Bytes32
	Bidder        meter.Address
	Amount        *big.Int
	ReserveAmount *big.Int
	Token         byte   // meter or meter gov
	Timestamp     uint64 // timestamp
	Nonce         uint64 // nonce
}

func (ab *AuctionBody) ToString() string {
	return fmt.Sprintf("AuctionBody: Opcode=%v, Version=%v, Option=%v, StartHegiht=%v, StartEpoch=%v, EndHeight=%v, EndEpoch=%v, Sequence=%v, AuctionID=%v, Bidder=%v, Amount=%v, ReserveAmount=%v, Token=%v, TimeStamp=%v, Nonce=%v",
		ab.Opcode, ab.Version, ab.Option, ab.StartHeight, ab.StartEpoch, ab.EndHeight, ab.EndEpoch, ab.Sequence, ab.AuctionID.AbbrevString(), ab.Bidder.String(), ab.Amount.String(), ab.ReserveAmount.String(), ab.Token, ab.Timestamp, ab.Nonce)
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
	errNotStart             = errors.New("Auction not start")
	errNotStop              = errors.New("An auction is active, stop first")
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

func (ab *AuctionBody) StartAuctionCB(env *AuctionEnv, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	Auction := env.GetAuction()
	state := env.GetState()
	auctionCB := Auction.GetAuctionCB(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	if auctionCB.IsActive() == true {
		log.Info("an auction is still active, stop first", "acution id", auctionCB.AuctionID)
		err = errNotStop
		return
	}

	auctionCB.StartHeight = ab.StartHeight
	auctionCB.StartEpoch = ab.StartEpoch
	auctionCB.EndHeight = ab.EndHeight
	auctionCB.EndEpoch = ab.EndEpoch
	auctionCB.Sequence = ab.Sequence
	auctionCB.RlsdMTRG = ab.Amount
	auctionCB.RsvdMTRG = ab.ReserveAmount
	auctionCB.RsvdPrice = builtin.Params.Native(state).Get(meter.KeyAuctionReservedPrice)
	auctionCB.CreateTime = ab.Timestamp
	auctionCB.RcvdMTR = big.NewInt(0)
	auctionCB.AuctionTxs = make([]*AuctionTx, 0)
	auctionCB.AuctionID = auctionCB.ID()

	Auction.SetAuctionCB(auctionCB, state)
	return
}

func (ab *AuctionBody) CloseAuctionCB(env *AuctionEnv, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	Auction := env.GetAuction()
	state := env.GetState()
	summaryList := Auction.GetSummaryList(state)
	auctionCB := Auction.GetAuctionCB(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	if auctionCB.IsActive() == false {
		log.Info("HandleAuctionTx: auction not start")
		err = errNotStart
		return
	}

	// clear the auction
	actualPrice, leftover, dist, err := Auction.ClearAuction(auctionCB, state, env)
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
		Sequence:     auctionCB.Sequence,
		RlsdMTRG:     auctionCB.RlsdMTRG,
		RsvdMTRG:     auctionCB.RsvdMTRG,
		RsvdPrice:    auctionCB.RsvdPrice,
		CreateTime:   auctionCB.CreateTime,
		RcvdMTR:      auctionCB.RcvdMTR,
		ActualPrice:  actualPrice,
		LeftoverMTRG: leftover,
		AuctionTxs:   auctionCB.AuctionTxs,
		DistMTRG:     dist,
	}

	// limit the summary list to AUCTION_MAX_SUMMARIES
	var summaries []*AuctionSummary
	sumLen := len(summaryList.Summaries)
	if sumLen >= AUCTION_MAX_SUMMARIES {
		summaries = append(summaryList.Summaries[sumLen-AUCTION_MAX_SUMMARIES+1:], summary)
	} else {
		summaries = append(summaryList.Summaries, summary)
	}

	summaryList = NewAuctionSummaryList(summaries)
	auctionCB = &AuctionCB{}
	Auction.SetSummaryList(summaryList, state)
	Auction.SetAuctionCB(auctionCB, state)
	return
}

func (ab *AuctionBody) HandleAuctionTx(env *AuctionEnv, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
	}()
	Auction := env.GetAuction()
	state := env.GetState()
	auctionCB := Auction.GetAuctionCB(state)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	if auctionCB.IsActive() == false {
		log.Info("HandleAuctionTx: auction not start")
		err = errNotStart
		return
	}

	if ab.Option == AUTO_BID {
		// check bidder have enough meter balance?
		if state.GetEnergy(meter.ValidatorBenefitAddr).Cmp(ab.Amount) < 0 {
			log.Info("not enough meter balance in validator benefit addr", "amount", ab.Amount, "bidder", ab.Bidder.String(), "vbalance", state.GetEnergy(meter.ValidatorBenefitAddr))
			err = errNotEnoughMTR
			return
		}
	} else {
		mtrBalance := state.GetEnergy(ab.Bidder)
		if mtrBalance.Cmp(ab.Amount) < 0 {
			log.Info("not enough meter balance", "bidder", ab.Bidder, "amount", ab.Amount, "balance", mtrBalance)
			err = errNotEnoughMTR
			return
		}

		if ab.Amount.Cmp(MinimumBidAmount) < 0 {
			log.Info("amount lower than minimum bid threshold", "amount", ab.Amount, "minBid", MinimumBidAmount)
			err = errLessThanBidThreshold
			return
		}
		// autobid assume the validator reward account have enough balance
	}

	tx := NewAuctionTx(ab.Bidder, ab.Amount, ab.Option, ab.Timestamp, ab.Nonce)
	err = auctionCB.AddAuctionTx(tx)
	if err != nil {
		log.Error("add auctionTx failed", "error", err)
		return
	}

	if ab.Option == AUTO_BID {
		// transfer bidder's autobid MTR directly from validator benefit address
		err = Auction.TransferAutobidMTRToAuction(ab.Bidder, ab.Amount, state, env)
	} else {
		// now transfer bidder's MTR to auction accout
		err = Auction.TransferMTRToAuction(ab.Bidder, ab.Amount, state, env)
	}
	if err != nil {
		log.Error("error happend during auction bid transfer", "address", ab.Bidder, "err", err)
		err = errNotEnoughMTR
		return
	}

	Auction.SetAuctionCB(auctionCB, state)
	return
}
