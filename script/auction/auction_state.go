package auction

import (
	"bytes"
	"errors"
	"math/big"
	"sort"
	"strings"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/runtime/statedb"
	"github.com/dfinlab/meter/state"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
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
		auctionCB := &AuctionCB{}

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), auctionCB)
			if err != nil {
				log.Warn("Error during decoding auction control block, set it as an empty ", "err", err)
				return err
			}
		}

		result = auctionCB
		return nil
	})
	return
}

func (a *Auction) SetAuctionCB(auctionCB *AuctionCB, state *state.State) {
	state.EncodeStorage(AuctionAccountAddr, AuctionCBKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(auctionCB)
	})
}

// summary List
func (a *Auction) GetSummaryList(state *state.State) (result *AuctionSummaryList) {
	state.DecodeStorage(AuctionAccountAddr, SummaryListKey, func(raw []byte) error {
		summaries := make([]*AuctionSummary, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), summaries)
			if err != nil {
				log.Warn("Error during decoding auction summary list, set it as an empty list", "err", err)
				return err
			}
		}

		result = NewAuctionSummaryList(summaries)
		return nil
	})
	return
}

func (a *Auction) SetSummaryList(summaryList *AuctionSummaryList, state *state.State) {
	sort.SliceStable(summaryList.Summaries, func(i, j int) bool {
		return bytes.Compare(summaryList.Summaries[i].AuctionID.Bytes(), summaryList.Summaries[j].AuctionID.Bytes()) <= 0
	})
	state.EncodeStorage(AuctionAccountAddr, SummaryListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(summaryList)
	})
}

//==================== account openation===========================
// from addr == > AuctionAccountAddr
func (a *Auction) TransferMTRToAuction(addr meter.Address, amount *big.Int, state *state.State) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterBalance := state.GetEnergy(addr)
	if meterBalance.Cmp(amount) < 0 {
		return errors.New("not enough meter")
	}

	state.AddEnergy(AuctionAccountAddr, amount)
	state.SubEnergy(addr, amount)
	return nil
}

func (a *Auction) SendMTRGToBidder(addr meter.Address, amount *big.Int, stateDB *statedb.StateDB) {
	if amount.Sign() == 0 {
		return
	}
	// in auction, MeterGov is mint action.
	stateDB.MintBalance(common.Address(addr), amount)
	return
}

// form AuctionAccountAddr ==> meter.ValidatorBenefitAddr
func (a *Auction) TransferMTRToValidatorBenefit(amount *big.Int, state *state.State) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterBalance := state.GetEnergy(AuctionAccountAddr)
	if meterBalance.Cmp(amount) < 0 {
		return errors.New("not enough meter")
	}

	state.AddEnergy(meter.ValidatorBenefitAddr, amount)
	state.SubEnergy(AuctionAccountAddr, amount)
	return nil
}

//==============================================
// when auction is over
func (a *Auction) ClearAuction(cb *AuctionCB, state *state.State) (*big.Int, *big.Int, error) {
	stateDB := statedb.New(state)
	ValidatorBenefitRatio := builtin.Params.Native(state).Get(meter.KeyValidatorBenefitRatio)

	actualPrice := big.NewInt(0)
	actualPrice = actualPrice.Div(cb.RcvdMTR, cb.RlsdMTRG)
	actualPrice = actualPrice.Mul(actualPrice, big.NewInt(1e18))
	if actualPrice.Cmp(cb.RsvdPrice) < 0 {
		actualPrice = cb.RsvdPrice
	}

	total := big.NewInt(0)
	for _, tx := range cb.AuctionTxs {
		mtrg := tx.Amount.Mul(tx.Amount, big.NewInt(1e18))
		mtrg = mtrg.Div(mtrg, actualPrice)
		a.SendMTRGToBidder(tx.Addr, mtrg, stateDB)
		total = total.Add(total, mtrg)
	}

	leftOver := new(big.Int).Sub(cb.RlsdMTRG, total)
	a.SendMTRGToBidder(AuctionAccountAddr, leftOver, stateDB)

	// 40% of received meter to AuctionValidatorBenefitAddr
	amount := new(big.Int).Mul(cb.RcvdMTR, ValidatorBenefitRatio)
	amount = amount.Div(amount, big.NewInt(1e18))
	a.TransferMTRToValidatorBenefit(amount, state)

	a.logger.Info("finished auctionCB clear...", "actualPrice", actualPrice.Uint64(), "leftOver", leftOver.Uint64(), "validatorBenefit", amount.Uint64())
	return actualPrice, leftOver, nil
}
