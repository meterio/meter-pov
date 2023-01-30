package auction

import (
	"bytes"
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/runtime/statedb"
	setypes "github.com/meterio/meter-pov/script/types"
)

func (a *Auction) CloseAuctionCB(env *setypes.ScriptEnv, ab *AuctionBody, gas uint64) (leftOverGas uint64, err error) {
	var ret []byte
	start := time.Now()
	defer func() {
		if err != nil {
			ret = []byte(err.Error())
		}
		env.SetReturnData(ret)
		a.logger.Info("Auction close completed", "elapsed", meter.PrettyDuration(time.Since(start)))
	}()

	state := env.GetState()
	summaryList := state.GetSummaryList()
	auctionCB := state.GetAuctionCB()

	if gas < meter.ClauseGas {
		leftOverGas = 0
	} else {
		leftOverGas = gas - meter.ClauseGas
	}

	if !auctionCB.IsActive() {
		a.logger.Error("auction not started yet")
		err = errNotStart
		return
	}
	// clear the auction
	validatorBenefitRatio := builtin.Params.Native(state).Get(meter.KeyValidatorBenefitRatio)

	actualPrice, leftover, dist, err := a.ClearAuction(env, auctionCB, validatorBenefitRatio)
	if err != nil {
		a.logger.Info("clear active auction failed failed")
		return
	}
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

	number := env.GetBlockNum()
	if meter.IsTeslaFork6(number) {
		for i := 0; i < len(summaries)-1; i++ {
			summaries[i].AuctionTxs = make([]*meter.AuctionTx, 0)
			summaries[i].DistMTRG = make([]*meter.DistMtrg, 0)
		}
	}
	summaryList = meter.NewAuctionSummaryList(summaries)
	auctionCB = &meter.AuctionCB{}

	state.SetSummaryList(summaryList)
	state.SetAuctionCB(auctionCB)

	return
}

func (a *Auction) MintMTRGToBidder(env *setypes.ScriptEnv, addr meter.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	state := env.GetState()
	stateDB := statedb.New(state)
	// in auction, MeterGov is mint action.
	stateDB.MintBalance(common.Address(addr), amount)
	env.AddTransfer(meter.ZeroAddress, addr, amount, meter.MTRG)
	return
}

// //////////////////////
// called when auction is over
func (a *Auction) ClearAuction(env *setypes.ScriptEnv, cb *meter.AuctionCB, validatorBenefitRatio *big.Int) (*big.Int, *big.Int, []*meter.DistMtrg, error) {

	start := time.Now()

	actualPrice := new(big.Int).Mul(cb.RcvdMTR, big.NewInt(1e18))
	if cb.RlsdMTRG.Cmp(big.NewInt(0)) > 0 {
		actualPrice = actualPrice.Div(actualPrice, cb.RlsdMTRG)
	} else {
		actualPrice = cb.RsvdPrice
	}
	if actualPrice.Cmp(cb.RsvdPrice) < 0 {
		actualPrice = cb.RsvdPrice
	}

	blockNum := env.GetBlockNum()
	total := big.NewInt(0)
	distMtrg := []*meter.DistMtrg{}
	if meter.IsTeslaFork3(blockNum) {

		groupTxMap := make(map[meter.Address]*big.Int)
		sortedAddresses := make([]meter.Address, 0)
		for _, tx := range cb.AuctionTxs {
			mtrg := new(big.Int).Mul(tx.Amount, big.NewInt(1e18))
			mtrg = new(big.Int).Div(mtrg, actualPrice)

			if _, ok := groupTxMap[tx.Address]; ok == true {
				groupTxMap[tx.Address] = new(big.Int).Add(groupTxMap[tx.Address], mtrg)
			} else {
				groupTxMap[tx.Address] = new(big.Int).Set(mtrg)
				sortedAddresses = append(sortedAddresses, tx.Address)
			}
		}

		sort.SliceStable(sortedAddresses, func(i, j int) bool {
			return bytes.Compare(sortedAddresses[i].Bytes(), sortedAddresses[j].Bytes()) <= 0
		})

		stub := time.Now()
		for _, addr := range sortedAddresses {
			mstart := time.Now()
			mtrg := groupTxMap[addr]
			a.MintMTRGToBidder(env, addr, mtrg)
			total = total.Add(total, mtrg)
			distMtrg = append(distMtrg, &meter.DistMtrg{Addr: addr, Amount: mtrg})
			a.logger.Debug("mint MTRG to bidder", "elapsed", meter.PrettyDuration(time.Since(mstart)), "addr", addr)
		}
		a.logger.Info("4. mint MTRG to bidder total", "elapsed", meter.PrettyDuration(time.Since(stub)), "count", len(sortedAddresses))
	} else {
		for _, tx := range cb.AuctionTxs {
			mtrg := new(big.Int).Mul(tx.Amount, big.NewInt(1e18))
			mtrg = new(big.Int).Div(mtrg, actualPrice)

			a.MintMTRGToBidder(env, tx.Address, mtrg)
			if (meter.IsMainNet() && blockNum < meter.TeslaFork3_MainnetAuctionDefectStartNum) || meter.IsTestNet() {
				total = total.Add(total, mtrg)
			}
			distMtrg = append(distMtrg, &meter.DistMtrg{Addr: tx.Address, Amount: mtrg})
		}

	}

	// sometimes accuracy cause negative value
	leftOver := new(big.Int).Sub(cb.RlsdMTRG, total)
	if leftOver.Sign() < 0 {
		leftOver = big.NewInt(0)
	}

	// send the remainings to accumulate accounts
	a.MintMTRGToBidder(env, meter.AuctionLeftOverAccount, cb.RsvdMTRG)
	a.MintMTRGToBidder(env, meter.AuctionLeftOverAccount, leftOver)

	// 40% of received meter to AuctionValidatorBenefitAddr
	amount := new(big.Int).Mul(cb.RcvdMTR, validatorBenefitRatio)
	amount = amount.Div(amount, big.NewInt(1e18))
	env.TransferMTRToValidatorBenefit(amount)

	a.logger.Info("Auction cleared", "actualPrice", actualPrice.String(), "leftOver", leftOver.String(), "validatorBenefit", amount.String(), "elapsed", meter.PrettyDuration(time.Since(start)))
	return actualPrice, leftOver, distMtrg, nil
}
