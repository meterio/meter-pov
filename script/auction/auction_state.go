package auction

import (
	"bytes"
	"encoding/gob"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
)

// the global variables in auction
var (
	AuctionAccountAddr = meter.BytesToAddress([]byte("auction-account-address"))
	SummaryListKey     = meter.Blake2b([]byte("summary-list-key"))
	AuctionCBKey       = meter.Blake2b([]byte("auction-active-cb-key"))
)

// Candidate List
func (a *Auction) GetAuctionCB(state *state.State) (result *AuctionCB) {
	state.DecodeStorage(AuctionAccountAddr, AuctionCBKey, func(raw []byte) error {
		// fmt.Println("Loaded Raw Hex: ", hex.EncodeToString(raw))
		decoder := gob.NewDecoder(bytes.NewBuffer(raw))
		var auctionCB AuctionCB
		err := decoder.Decode(&auctionCB)
		if err != nil {
			a.logger.Warn("Error during decoding AuctionCB", "err", err)
			return err
		}
		result = &auctionCB
		return nil
	})
	return
}

func (a *Auction) SetAuctionCB(auctionCB *AuctionCB, state *state.State) {
	state.EncodeStorage(AuctionAccountAddr, AuctionCBKey, func() ([]byte, error) {
		buf := bytes.NewBuffer([]byte{})
		encoder := gob.NewEncoder(buf)
		err := encoder.Encode(auctionCB)
		return buf.Bytes(), err
	})
}

// summary List
func (a *Auction) GetSummaryList(state *state.State) (result *AuctionSummaryList) {
	state.DecodeStorage(AuctionAccountAddr, SummaryListKey, func(raw []byte) error {
		decoder := gob.NewDecoder(bytes.NewBuffer(raw))

		var summaries []*AuctionSummary
		err := decoder.Decode(&summaries)
		if err != nil {
			a.logger.Warn("Error during decoding auctionSummary list", "err", err)
			return nil
		}

		result = NewAuctionSummaryList(summaries)
		return nil
	})
	return
}

func (a *Auction) SetSummaryList(summaryList *AuctionSummaryList, state *state.State) {
	state.EncodeStorage(AuctionAccountAddr, SummaryListKey, func() ([]byte, error) {
		buf := bytes.NewBuffer([]byte{})
		encoder := gob.NewEncoder(buf)
		err := encoder.Encode(summaryList.summaries)
		return buf.Bytes(), err
	})
}

//==================== account openation===========================
func (a *Auction) TransferMTRToAuction(addr meter.Address, amount *big.Int, state *state.State) error {
	if amount.Sign() == 0 {
		return nil
	}
	var balance *big.Int

	balance = state.GetEnergy(addr)
	state.SetEnergy(meter.Address(addr), new(big.Int).Sub(balance, amount))

	balance = state.GetEnergy(AuctionAccountAddr)
	state.SetEnergy(AuctionAccountAddr, new(big.Int).Add(balance, amount))
	return nil
}

func (a *Auction) SendMTRGToBidder(addr meter.Address, amount *big.Int, state *state.State) error {
	if amount.Sign() == 0 {
		return nil
	}

	balance := state.GetBalance(addr)
	state.SetBalance(addr, new(big.Int).Add(balance, amount))
	return nil
}

//==============================================
// when auction is over
func (a *Auction) ClearAuction(cb *AuctionCB, state *state.State) (*big.Int, error) {
	actualPrice := cb.rlsdMTRG.Div(cb.rcvdMTR, cb.rlsdMTRG)
	if actualPrice.Cmp(cb.rsvdPrice) < 0 {
		actualPrice = cb.rsvdPrice
	}

	var total *big.Int
	for _, tx := range cb.auctionTxs {
		mtrg := tx.amount.Div(tx.amount, actualPrice)
		a.SendMTRGToBidder(tx.addr, mtrg, state)
		total = total.Add(total, mtrg)
	}

	leftOver := cb.rlsdMTRG.Sub(cb.rlsdMTRG, total)
	a.SendMTRGToBidder(AuctionAccountAddr, leftOver, state)
	a.logger.Info("finished auctionCB clear...", "actualPrice", actualPrice.Int64(), "leftOver", leftOver.Int64())
	return actualPrice, nil
}
