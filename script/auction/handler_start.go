package auction

import (
	"math/big"
	"time"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	setypes "github.com/meterio/meter-pov/script/types"
)

func (a *Auction) StartAuctionCB(env *setypes.ScriptEnv, ab *AuctionBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	start := time.Now()
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
		a.logger.Info("Auction start completed", "elapsed", meter.PrettyDuration(time.Since(start)))
	}()
	state := env.GetState()
	auctionCB := state.GetAuctionCB()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	if auctionCB.IsActive() {
		a.logger.Info("an auction is still active, stop first", "acution id", auctionCB.AuctionID)
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
