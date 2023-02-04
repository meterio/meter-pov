package auction

import (
	"time"

	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
)

func (a *Auction) HandleAuctionTx(env *setypes.ScriptEnv, ab *AuctionBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	start := time.Now()
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
		a.logger.Debug("Bid completed", "elapsed", meter.PrettyDuration(time.Since(start)))
	}()
	stub := time.Now()
	getAuctionTime := meter.PrettyDuration(time.Since(stub))

	stub = time.Now()
	state := env.GetState()
	getStateTime := meter.PrettyDuration(time.Since(stub))

	stub = time.Now()
	auctionCB := state.GetAuctionCB()
	getAuctionCBTime := meter.PrettyDuration(time.Since(stub))
	a.logger.Debug("Read completed. ", "getAuction", getAuctionTime, "getState", getStateTime, "getAuctionCB", getAuctionCBTime)

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	if !auctionCB.IsActive() {
		a.logger.Info("HandleAuctionTx: auction not start")
		err = errNotStart
		return
	}

	if ab.Option == meter.AUTO_BID {
		// check bidder have enough meter balance?
		if state.GetEnergy(meter.ValidatorBenefitAddr).Cmp(ab.Amount) < 0 {
			a.logger.Info("not enough meter balance in validator benefit addr", "amount", ab.Amount, "bidder", ab.Bidder.String(), "vbalance", state.GetEnergy(meter.ValidatorBenefitAddr))
			err = errNotEnoughMTR
			return
		}
	} else {
		mtrBalance := state.GetEnergy(ab.Bidder)
		if mtrBalance.Cmp(ab.Amount) < 0 {
			a.logger.Info("not enough meter balance", "bidder", ab.Bidder, "amount", ab.Amount, "balance", mtrBalance)
			err = errNotEnoughMTR
			return
		}

		if ab.Amount.Cmp(MinimumBidAmount) < 0 {
			a.logger.Info("amount lower than minimum bid threshold", "amount", ab.Amount, "minBid", MinimumBidAmount)
			err = errLessThanBidThreshold
			return
		}
		// autobid assume the validator reward account have enough balance
	}

	number := env.GetBlockNum()
	ts := ab.Timestamp
	nonce := ab.Nonce
	if meter.IsTeslaFork7(number) {
		ts = env.GetBlockCtx().Time
		nonce = env.GetTxCtx().Nonce + uint64(env.GetClauseIndex())
	}
	tx := meter.NewAuctionTx(ab.Bidder, ab.Amount, ab.Option, ts, nonce)

	stub = time.Now()
	err = auctionCB.AddAuctionTx(tx)
	a.logger.Debug("Auction tx added", "elapsed", meter.PrettyDuration(time.Since(stub)))
	if err != nil {
		a.logger.Error("add auctionTx failed", "error", err)
		return
	}

	if ab.Option == meter.AUTO_BID {
		// transfer bidder's autobid MTR directly from validator benefit address
		err = env.TransferAutobidMTRToAuction(ab.Bidder, ab.Amount)
	} else {
		// now transfer bidder's MTR to auction accout
		err = env.TransferMTRToAuction(ab.Bidder, ab.Amount)
	}
	if err != nil {
		a.logger.Error("error happend during auction bid transfer", "address", ab.Bidder, "err", err)
		err = errNotEnoughMTR
		return
	}

	stub = time.Now()
	state.SetAuctionCB(auctionCB)
	a.logger.Debug("Save completed", "elapsed", meter.PrettyDuration(time.Since(stub)))
	return
}
