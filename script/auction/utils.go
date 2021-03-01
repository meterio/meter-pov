package auction

import (
	"errors"

	"github.com/dfinlab/meter/block"
)

//  api routine interface
func GetActiveAuctionCB() (*AuctionCB, error) {
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

	cb := auction.GetAuctionCB(state)
	return cb, nil
}

func GetAuctionSummaryList() (*AuctionSummaryList, error) {
	auction := GetAuctionGlobInst()
	if auction == nil {
		log.Error("auction is not initialized...")
		err := errors.New("aution is not initialized...")
		return NewAuctionSummaryList(nil), err
	}

	best := auction.chain.BestBlock()
	state, err := auction.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return NewAuctionSummaryList(nil), err
	}

	summaryList := auction.GetSummaryList(state)
	if summaryList == nil {
		log.Error("no summaryList stored ...")
		return NewAuctionSummaryList(nil), nil
	}
	return summaryList, nil
}

// api routine interface
func GetAuctionSummaryListByHeader(header *block.Header) (*AuctionSummaryList, error) {
	auction := GetAuctionGlobInst()
	if auction == nil {
		log.Error("auction is not initialized...")
		err := errors.New("aution is not initialized...")
		return NewAuctionSummaryList(nil), err
	}

	h := header
	if header == nil {
		h = auction.chain.BestBlock().Header()
	}
	state, err := auction.stateCreator.NewState(h.StateRoot())
	if err != nil {
		return NewAuctionSummaryList(nil), err
	}

	summaryList := auction.GetSummaryList(state)
	if summaryList == nil {
		log.Error("no summaryList stored ...")
		return NewAuctionSummaryList(nil), nil
	}
	return summaryList, nil
}
