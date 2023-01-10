package auction

import (
	"bytes"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/runtime/statedb"
	setypes "github.com/meterio/meter-pov/script/types"
)

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

	actualPrice := new(big.Int).Mul(cb.RcvdMTR, big.NewInt(1e18))
	if cb.RlsdMTRG.Cmp(big.NewInt(0)) > 0 {
		actualPrice = actualPrice.Div(actualPrice, cb.RlsdMTRG)
	} else {
		actualPrice = cb.RsvdPrice
	}
	if actualPrice.Cmp(cb.RsvdPrice) < 0 {
		actualPrice = cb.RsvdPrice
	}

	blockNum := env.GetTxCtx().BlockRef.Number()
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

		for _, addr := range sortedAddresses {
			mtrg := groupTxMap[addr]
			a.MintMTRGToBidder(env, addr, mtrg)
			total = total.Add(total, mtrg)
			distMtrg = append(distMtrg, &meter.DistMtrg{Addr: addr, Amount: mtrg})
		}
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

	log.Info("finished auctionCB clear...", "actualPrice", actualPrice.String(), "leftOver", leftOver.String(), "validatorBenefit", amount.String())
	return actualPrice, leftOver, distMtrg, nil
}
