package auction

import (
	"errors"

	"github.com/meterio/meter-pov/meter"
)

// api routine interface
func GetActiveAuctionCB() (*meter.AuctionCB, error) {
	auction := GetAuctionGlobInst()
	if auction == nil {
		log.Warn("auction is not initialized...")
		err := errors.New("auction is not initialized...")
		return nil, err
	}

	best := auction.chain.BestBlock()
	state, err := auction.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return nil, err
	}

	cb := state.GetAuctionCB()
	return cb, nil
}

func GetAuctionSummaryList() (*meter.AuctionSummaryList, error) {
	auction := GetAuctionGlobInst()
	if auction == nil {
		log.Error("auction is not initialized...")
		err := errors.New("aution is not initialized...")
		return meter.NewAuctionSummaryList(nil), err
	}

	best := auction.chain.BestBlock()
	state, err := auction.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return meter.NewAuctionSummaryList(nil), err
	}

	summaryList := state.GetSummaryList()
	if summaryList == nil {
		log.Error("no summaryList stored ...")
		return meter.NewAuctionSummaryList(nil), nil
	}
	return summaryList, nil
}
