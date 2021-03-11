// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auction

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/runtime/statedb"
	"github.com/dfinlab/meter/state"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
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
//from meter.ValidatorBenefitAddr ==> AuctionAccountAddr
func (a *Auction) TransferAutobidMTRToAuction(addr meter.Address, amount *big.Int, state *state.State, env *AuctionEnv) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterBalance := state.GetEnergy(meter.ValidatorBenefitAddr)
	if meterBalance.Cmp(amount) < 0 {
		return fmt.Errorf("not enough meter balance in validator benefit address, balance:%v amount:%v", meterBalance, amount)
	}

	a.logger.Info("transfer autobid MTR", "bidder", addr, "amount", amount)
	state.SubEnergy(meter.ValidatorBenefitAddr, amount)
	state.AddEnergy(AuctionAccountAddr, amount)
	env.AddTransfer(meter.ValidatorBenefitAddr, AuctionAccountAddr, amount, meter.MTR)
	return nil
}

// from addr == > AuctionAccountAddr
func (a *Auction) TransferMTRToAuction(addr meter.Address, amount *big.Int, state *state.State, env *AuctionEnv) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterBalance := state.GetEnergy(addr)
	if meterBalance.Cmp(amount) < 0 {
		return errors.New("not enough meter")
	}

	a.logger.Info("transfer userbid MTR", "bidder", addr, "amount", amount)
	state.SubEnergy(addr, amount)
	state.AddEnergy(AuctionAccountAddr, amount)
	env.AddTransfer(addr, AuctionAccountAddr, amount, meter.MTR)
	return nil
}

func (a *Auction) SendMTRGToBidder(addr meter.Address, amount *big.Int, stateDB *statedb.StateDB, env *AuctionEnv) {
	if amount.Sign() == 0 {
		return
	}
	// in auction, MeterGov is mint action.
	stateDB.MintBalance(common.Address(addr), amount)
	env.AddTransfer(meter.ZeroAddress, addr, amount, meter.MTRG)
	return
}

// form AuctionAccountAddr ==> meter.ValidatorBenefitAddr
func (a *Auction) TransferMTRToValidatorBenefit(amount *big.Int, state *state.State, env *AuctionEnv) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterBalance := state.GetEnergy(AuctionAccountAddr)
	if meterBalance.Cmp(amount) < 0 {
		return errors.New("not enough meter")
	}

	state.SubEnergy(AuctionAccountAddr, amount)
	state.AddEnergy(meter.ValidatorBenefitAddr, amount)
	env.AddTransfer(AuctionAccountAddr, meter.ValidatorBenefitAddr, amount, meter.MTR)

	return nil
}

////////////////////////
// called when auction is over
func (a *Auction) ClearAuction(cb *AuctionCB, state *state.State, env *AuctionEnv) (*big.Int, *big.Int, []*DistMtrg, error) {
	stateDB := statedb.New(state)
	ValidatorBenefitRatio := builtin.Params.Native(state).Get(meter.KeyValidatorBenefitRatio)

	actualPrice := new(big.Int).Mul(cb.RcvdMTR, big.NewInt(1e18))
	if cb.RlsdMTRG.Cmp(big.NewInt(0)) > 0 {
		actualPrice = actualPrice.Div(actualPrice, cb.RlsdMTRG)
	} else {
		actualPrice = cb.RsvdPrice
	}
	if actualPrice.Cmp(cb.RsvdPrice) < 0 {
		actualPrice = cb.RsvdPrice
	}

	total := big.NewInt(0)
	distMtrg := []*DistMtrg{}
	for _, tx := range cb.AuctionTxs {
		mtrg := new(big.Int).Mul(tx.Amount, big.NewInt(1e18))
		mtrg = new(big.Int).Div(mtrg, actualPrice)

		a.SendMTRGToBidder(tx.Address, mtrg, stateDB, env)
		total = total.Add(total, mtrg)
		distMtrg = append(distMtrg, &DistMtrg{Addr: tx.Address, Amount: mtrg})
	}

	// sometimes accuracy cause negative value
	leftOver := new(big.Int).Sub(cb.RlsdMTRG, total)
	if leftOver.Sign() < 0 {
		leftOver = big.NewInt(0)
	}

	// send the remainings to accumulate accounts
	a.SendMTRGToBidder(meter.AuctionLeftOverAccount, cb.RsvdMTRG, stateDB, env)
	a.SendMTRGToBidder(meter.AuctionLeftOverAccount, leftOver, stateDB, env)

	// 40% of received meter to AuctionValidatorBenefitAddr
	amount := new(big.Int).Mul(cb.RcvdMTR, ValidatorBenefitRatio)
	amount = amount.Div(amount, big.NewInt(1e18))
	a.TransferMTRToValidatorBenefit(amount, state, env)

	a.logger.Info("finished auctionCB clear...", "actualPrice", actualPrice.String(), "leftOver", leftOver.String(), "validatorBenefit", amount.String())
	return actualPrice, leftOver, distMtrg, nil
}
