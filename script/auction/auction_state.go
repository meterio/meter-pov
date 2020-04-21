package auction

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/runtime/statedb"
	"github.com/dfinlab/meter/state"

	"github.com/ethereum/go-ethereum/common"
)

// the global variables in auction
var (
	// 0x74696f6e2d6163636f756e742d61646472657373
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
			if err.Error() == "EOF" && len(raw) == 0 {
				// empty raw, do nothing
			} else {
				log.Warn("Error during decoding auctionCB, set it as an empty list", "err", err)
			}
			result = &AuctionCB{}
			return nil

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
		result = NewAuctionSummaryList(summaries)
		if err != nil {
			if err.Error() == "EOF" && len(raw) == 0 {
				// empty raw, do nothing
			} else {
				log.Warn("Error during decoding auctionSummary list", "err", err)
			}
			return nil
		}
		return nil
	})
	return
}

func encode(obj interface{}) string {
	buf := bytes.NewBuffer([]byte{})
	encoder := gob.NewEncoder(buf)
	encoder.Encode(obj)
	return hex.EncodeToString(buf.Bytes())
}

func (a *Auction) SetSummaryList(summaryList *AuctionSummaryList, state *state.State) {
	h, _ := state.Stage().Hash()
	fmt.Println("Before set summary list:", h)
	state.EncodeStorage(AuctionAccountAddr, SummaryListKey, func() ([]byte, error) {

		buf := bytes.NewBuffer([]byte{})
		encoder := gob.NewEncoder(buf)
		err := encoder.Encode(summaryList.Summaries)
		if err != nil {
			fmt.Println("ERROR: ", err)
		}
		fmt.Println("summary list HEX: ", hex.EncodeToString(buf.Bytes()))
		// for i, s := range summaryList.Summaries {
		// b := bytes.NewBuffer([]byte{})
		// en := gob.NewEncoder(b)
		// err := en.Encode(s)
		// if err != nil {
		// 	fmt.Println("ERROR: ", err)
		// }
		// fmt.Println("AuctionID:", encode(s.AuctionID))
		// fmt.Println("StartHeight:", encode(s.StartHeight))
		// fmt.Println("StartEpoch:", encode(s.StartEpoch))
		// fmt.Println("EndHeight:", encode(s.EndHeight))
		// fmt.Println("EndEpoch:", encode(s.EndEpoch))
		// fmt.Println("RlsdMTRG:", encode(s.RlsdMTRG))
		// fmt.Println("RsvdPrice:", encode(s.RsvdPrice))
		// fmt.Println("CreateTime:", encode(s.CreateTime))
		// fmt.Println("RcvdMTR:", encode(s.RcvdMTR))
		// fmt.Println("ActualPrice:", encode(s.ActualPrice))
		// fmt.Println("LeftoverMTRG:", encode(s.LeftoverMTRG))
		// uint64s := []uint64{s.StartHeight, s.StartEpoch, s.EndHeight, s.EndEpoch, s.CreateTime}
		// bigInts := []*big.Int{s.RlsdMTRG, s.RsvdPrice, s.RcvdMTR, s.ActualPrice, s.LeftoverMTRG}
		// fmt.Println("Uint64 HEX #", i+1, encode(uint64s))
		// fmt.Println("Big Int HEX #", i+1, encode(bigInts))
		// fmt.Println("Summary HEX #", i+1, encode(s))
		// decoder := gob.NewDecoder(bytes.NewReader(b.Bytes()))
		// ss := &AuctionSummary{}
		// decoder.Decode(ss)
		// fmt.Println("Summary decode/encode Hex:", encode(ss))

		// fmt.Println("Summary #", i+1, s.ToString())
		// fmt.Println("Summary Hex #", i+1, hex.EncodeToString(b.Bytes()))
		// }
		return buf.Bytes(), err
	})
	rlsdMTRG := big.NewInt(0)
	rs, _ := hex.DecodeString("0373b7bce81846c00000")
	rlsdMTRG.SetBytes(rs)
	s := AuctionSummary{
		AuctionID:    meter.MustParseBytes32("0x7f9e08daab355e0f881570f74463ea1e8e276ab21f7d9dbc3540a47a0c9999e0"),
		StartHeight:  1,
		StartEpoch:   1,
		EndHeight:    3205,
		EndEpoch:     24,
		RlsdMTRG:     rlsdMTRG,
		RsvdPrice:    big.NewInt(500000000000000000),
		CreateTime:   1587162561,
		RcvdMTR:      big.NewInt(0),
		ActualPrice:  big.NewInt(500000000000000000),
		LeftoverMTRG: rlsdMTRG,
	}
	buf := bytes.NewBuffer([]byte{})
	en := gob.NewEncoder(buf)
	en.Encode(&s)
	fmt.Println("SUMMARY HEX:", hex.EncodeToString(buf.Bytes()))
	h, _ = state.Stage().Hash()
	fmt.Println("After set summary list:", h)
}

//==================== account openation===========================
func (a *Auction) TransferMTRToAuction(addr meter.Address, amount *big.Int, state *state.State) error {
	if amount.Sign() == 0 {
		return nil
	}
	var balance *big.Int

	balance = state.GetEnergy(addr)
	fmt.Println("Calling: SetEnergy", meter.Address(addr).String(), new(big.Int).Sub(balance, amount).String())
	state.SetEnergy(meter.Address(addr), new(big.Int).Sub(balance, amount))

	balance = state.GetEnergy(AuctionAccountAddr)
	fmt.Println("Calling: SetEnergy", AuctionAccountAddr.String(), new(big.Int).Add(balance, amount).String())
	state.SetEnergy(AuctionAccountAddr, new(big.Int).Add(balance, amount))
	return nil
}

func (a *Auction) SendMTRGToBidder(addr meter.Address, amount *big.Int, stateDB *statedb.StateDB) error {
	if amount.Sign() == 0 {
		return nil
	}

	// in auction, MeterGov is mint action.
	fmt.Println("Calling: MintBalance", common.Address(addr).String(), amount)
	stateDB.MintBalance(common.Address(addr), amount)
	return nil
}

//==============================================
// when auction is over
func (a *Auction) ClearAuction(cb *AuctionCB, state *state.State) (*big.Int, *big.Int, error) {
	h, _ := state.Stage().Hash()
	fmt.Println("Before clear auction:", h)
	stateDB := statedb.New(state)

	actualPrice := big.NewInt(0)
	actualPrice = actualPrice.Div(cb.RcvdMTR, cb.RlsdMTRG)
	actualPrice = actualPrice.Mul(actualPrice, big.NewInt(1e18))
	if actualPrice.Cmp(cb.RsvdPrice) < 0 {
		actualPrice = cb.RsvdPrice
	}

	total := big.NewInt(0)
	for _, tx := range cb.AuctionTxs {
		mtrg := tx.Amount.Div(tx.Amount, actualPrice)
		a.SendMTRGToBidder(tx.Addr, mtrg, stateDB)
		h, _ = state.Stage().Hash()
		fmt.Println("After Send MTRG to bidder:", h)
		total = total.Add(total, mtrg)
	}

	leftOver := big.NewInt(0)
	leftOver = leftOver.Sub(cb.RlsdMTRG, total)
	a.SendMTRGToBidder(AuctionAccountAddr, leftOver, stateDB)
	h, _ = state.Stage().Hash()
	fmt.Println("After Send MTRG to bidder:", h)

	a.logger.Info("finished auctionCB clear...", "actualPrice", actualPrice.Uint64(), "leftOver", leftOver.Uint64())
	return actualPrice, leftOver, nil
}
