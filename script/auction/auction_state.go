// Copyright (c) 2020 The Meter.io developerslopers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auction

import (
	"bytes"
	"errors"
	"math/big"
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
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding auction control block", "err", err)
					return err
				}
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
			err := rlp.Decode(bytes.NewReader(raw), &summaries)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding auction summary list", "err", err)
					return err
				}
			}
		}

		result = NewAuctionSummaryList(summaries)
		return nil
	})
	return
}

func (a *Auction) SetSummaryList(summaryList *AuctionSummaryList, state *state.State) {
	/**** Do not need sort here, it is automatically sorted by Epoch
	sort.SliceStable(summaryList.Summaries, func(i, j int) bool {
		return bytes.Compare(summaryList.Summaries[i].AuctionID.Bytes(), summaryList.Summaries[j].AuctionID.Bytes()) <= 0
	})
	****/
	state.EncodeStorage(AuctionAccountAddr, SummaryListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(summaryList.Summaries)
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
func (a *Auction) ClearAuction(cb *AuctionCB, state *state.State) (*big.Int, *big.Int, []*DistMtrg, error) {
	stateDB := statedb.New(state)
	ValidatorBenefitRatio := builtin.Params.Native(state).Get(meter.KeyValidatorBenefitRatio)

	actualPrice := new(big.Int).Mul(cb.RcvdMTR, big.NewInt(1e18))
	actualPrice = actualPrice.Div(actualPrice, cb.RlsdMTRG)
	if actualPrice.Cmp(cb.RsvdPrice) < 0 {
		actualPrice = cb.RsvdPrice
	}

	total := big.NewInt(0)
	distMtrg := []*DistMtrg{}
	for _, tx := range cb.AuctionTxs {
		mtrg := tx.Amount.Div(tx.Amount, actualPrice)
		mtrg = mtrg.Mul(mtrg, big.NewInt(1e18))
		a.SendMTRGToBidder(tx.Addr, mtrg, stateDB)
		total = total.Add(total, mtrg)
		distMtrg = append(distMtrg, &DistMtrg{Addr: tx.Addr, Amount: mtrg})
	}

	// sometimes accuracy cause negative value
	leftOver := new(big.Int).Sub(cb.RlsdMTRG, total)
	if leftOver.Sign() < 0 {
		leftOver = big.NewInt(0)
	}

	// send the remainings to accumulate accounts
	a.SendMTRGToBidder(meter.AuctionLeftOverAccount, cb.RsvdMTRG, stateDB)
	a.SendMTRGToBidder(meter.AuctionLeftOverAccount, leftOver, stateDB)

	// 40% of received meter to AuctionValidatorBenefitAddr
	amount := new(big.Int).Mul(cb.RcvdMTR, ValidatorBenefitRatio)
	amount = amount.Div(amount, big.NewInt(1e18))
	a.TransferMTRToValidatorBenefit(amount, state)

	a.logger.Info("finished auctionCB clear...", "actualPrice", actualPrice.String(), "leftOver", leftOver.String(), "validatorBenefit", amount.String())
	return actualPrice, leftOver, distMtrg, nil
}
