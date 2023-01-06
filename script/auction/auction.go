// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auction

import (
	"errors"
	"math/big"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
	"github.com/meterio/meter-pov/state"
)

var (
	AuctionGlobInst *Auction
	log             = log15.New("pkg", "auction")
)

// Candidate indicates the structure of a candidate
type Auction struct {
	chain        *chain.Chain
	stateCreator *state.Creator
	logger       log15.Logger
}

func GetAuctionGlobInst() *Auction {
	return AuctionGlobInst
}

func SetAuctionGlobInst(inst *Auction) {
	AuctionGlobInst = inst
}

func NewAuction(ch *chain.Chain, sc *state.Creator) *Auction {
	auction := &Auction{
		chain:        ch,
		stateCreator: sc,
		logger:       log15.New("pkg", "auction"),
	}
	SetAuctionGlobInst(auction)
	return auction
}

func (a *Auction) AuctionHandler(senv *setypes.ScriptEnv, payload []byte, to *meter.Address, gas uint64) (seOutput *setypes.ScriptEngineOutput, leftOverGas uint64, err error) {

	ab, err := AuctionDecodeFromBytes(payload)
	if err != nil {
		log.Error("Decode script message failed", "error", err)
		return nil, gas, err
	}

	if senv == nil {
		panic("create auction enviroment failed")
	}

	log.Debug("received auction", "body", ab.ToString())
	log.Debug("Entering auction handler "+ab.GetOpName(ab.Opcode), "tx", senv.GetTxHash())
	switch ab.Opcode {
	case OP_START:
		if senv.GetTxOrigin().IsZero() == false {
			return nil, gas, errors.New("not from kblock")
		}
		leftOverGas, err = a.StartAuctionCB(senv, ab, gas)

	case OP_STOP:
		if senv.GetTxOrigin().IsZero() == false {
			return nil, gas, errors.New("not form kblock")
		}
		leftOverGas, err = a.CloseAuctionCB(senv, ab, gas)

	case OP_BID:
		if ab.Option == AUTO_BID {
			if senv.GetTxOrigin().IsZero() == false {
				return nil, gas, errors.New("not from kblock")
			}
		} else {
			// USER_BID
			if senv.GetTxOrigin() != ab.Bidder {
				return nil, gas, errors.New("bidder address is not the same from transaction")
			}
		}
		leftOverGas, err = a.HandleAuctionTx(senv, ab, gas)

	default:
		log.Error("unknown Opcode", "Opcode", ab.Opcode)
		return nil, gas, errors.New("unknow auction opcode")
	}
	seOutput = senv.GetOutput()
	log.Debug("Leaving script handler for operation", "op", ab.GetOpName(ab.Opcode))
	return
}

func (a *Auction) StartAuctionCB(env *setypes.ScriptEnv, ab *AuctionBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	start := time.Now()
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
		log.Info("Auction start completed", "elapsed", meter.PrettyDuration(time.Since(start)))
	}()
	state := env.GetState()
	auctionCB := state.GetAuctionCB()

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
	auctionCB.AuctionTxs = make([]*meter.AuctionTx, 0)
	auctionCB.AuctionID = auctionCB.ID()

	state.SetAuctionCB(auctionCB)
	return
}

func (a *Auction) CloseAuctionCB(env *setypes.ScriptEnv, ab *AuctionBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	start := time.Now()
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
		log.Info("Auction close completed", "elapsed", meter.PrettyDuration(time.Since(start)))
	}()
	stub := time.Now()
	log.Info("Get auction", "elapsed", meter.PrettyDuration(time.Since(stub)))

	stub = time.Now()
	state := env.GetState()

	stub = time.Now()
	summaryList := state.GetSummaryList()
	log.Info("Get summary list", "elapsed", meter.PrettyDuration(time.Since(stub)))

	stub = time.Now()
	auctionCB := state.GetAuctionCB()
	log.Info("Get auction cb", "elapsed", meter.PrettyDuration(time.Since(stub)))

	stub = time.Now()
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
	log.Info("precheck completed", "elapsed", meter.PrettyDuration(time.Since(stub)))

	stub = time.Now()
	// clear the auction
	actualPrice, leftover, dist, err := a.ClearAuction(auctionCB, state, env)
	if err != nil {
		log.Info("clear active auction failed failed")
		return
	}
	log.Info("clear auction completed", "elapsed", meter.PrettyDuration(time.Since(stub)))

	stub = time.Now()
	summary := &meter.AuctionSummary{
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
	var summaries []*meter.AuctionSummary
	sumLen := len(summaryList.Summaries)
	if sumLen >= AUCTION_MAX_SUMMARIES {
		summaries = append(summaryList.Summaries[sumLen-AUCTION_MAX_SUMMARIES+1:], summary)
	} else {
		summaries = append(summaryList.Summaries, summary)
	}

	summaryList = meter.NewAuctionSummaryList(summaries)
	auctionCB = &meter.AuctionCB{}
	log.Info("append summary completed", "elapsed", meter.PrettyDuration(time.Since(stub)))

	stub = time.Now()
	state.SetSummaryList(summaryList)
	log.Info("set summary completed", "elapsed", meter.PrettyDuration(time.Since(stub)))

	stub = time.Now()
	state.SetAuctionCB(auctionCB)
	log.Info("set auction cb completed", "elapsed", meter.PrettyDuration(time.Since(stub)))

	return
}

func (a *Auction) HandleAuctionTx(env *setypes.ScriptEnv, ab *AuctionBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	start := time.Now()
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
		log.Info("Bid completed", "elapsed", meter.PrettyDuration(time.Since(start)))
	}()
	stub := time.Now()
	getAuctionTime := meter.PrettyDuration(time.Since(stub))

	stub = time.Now()
	state := env.GetState()
	getStateTime := meter.PrettyDuration(time.Since(stub))

	stub = time.Now()
	auctionCB := state.GetAuctionCB()
	getAuctionCBTime := meter.PrettyDuration(time.Since(stub))
	log.Info("Read completed. ", "getAuction", getAuctionTime, "getState", getStateTime, "getAuctionCB", getAuctionCBTime)

	stub = time.Now()
	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	if !auctionCB.IsActive() {
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

	tx := meter.NewAuctionTx(ab.Bidder, ab.Amount, ab.Option, ab.Timestamp, ab.Nonce)

	stub = time.Now()
	err = auctionCB.AddAuctionTx(tx)
	log.Debug("Auction tx added", "elapsed", meter.PrettyDuration(time.Since(stub)))
	if err != nil {
		log.Error("add auctionTx failed", "error", err)
		return
	}

	if ab.Option == AUTO_BID {
		// transfer bidder's autobid MTR directly from validator benefit address
		err = a.TransferAutobidMTRToAuction(ab.Bidder, ab.Amount, state, env)
	} else {
		// now transfer bidder's MTR to auction accout
		err = a.TransferMTRToAuction(ab.Bidder, ab.Amount, state, env)
	}
	if err != nil {
		log.Error("error happend during auction bid transfer", "address", ab.Bidder, "err", err)
		err = errNotEnoughMTR
		return
	}

	stub = time.Now()
	state.SetAuctionCB(auctionCB)
	log.Info("Save completed", "elapsed", meter.PrettyDuration(time.Since(stub)))
	return
}
