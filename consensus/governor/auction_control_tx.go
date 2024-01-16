// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package governor

import (
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/script/auction"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
)

func BuildAuctionControlTx(height, epoch uint64, chainTag byte, bestNum uint32, s *state.State, chain *chain.Chain) *tx.Transaction {
	// check current active auction first if there is one
	var currentActive bool
	auctionCB := s.GetAuctionCB()
	summaryList := s.GetSummaryList()
	if auctionCB.IsActive() {
		currentActive = true
	}
	// reservedPrice := builtin.Params.Native(s).Get(meter.KeyAuctionReservedPrice)

	// r := builtin.Params.Native(s).Get(meter.KeyAuctionInitRelease)
	// r = r.Div(r, big.NewInt(1e09))
	// fr := new(big.Float).SetInt(r)
	// initialRelease, accuracy := fr.Float64()
	// initialRelease = initialRelease / (1e09)

	baseSequence := builtin.Params.Native(s).Get(meter.KeyBaseSequence_AfterFork11)

	// now start a new auction
	var lastEndHeight, lastEndEpoch, lastSequence uint64
	if currentActive {
		lastEndHeight = auctionCB.EndHeight
		lastEndEpoch = auctionCB.EndEpoch
		lastSequence = auctionCB.Sequence
	} else {
		size := len(summaryList.Summaries)
		if size != 0 {
			lastEndHeight = summaryList.Summaries[size-1].EndHeight
			lastEndEpoch = summaryList.Summaries[size-1].EndEpoch
			lastSequence = summaryList.Summaries[size-1].Sequence
		} else {
			if meter.IsTesla(uint32(height)) {
				lastEndHeight = meter.TeslaMainnetStartNum
				ep, err := chain.FindEpochOnBlock(uint32(lastEndHeight))
				if err != nil {
					// something wrong to get this epoch
					lastEndEpoch = 0
				} else {
					lastEndEpoch = ep
				}
				lastSequence = 0
			} else {
				lastEndHeight = 0
				lastEndEpoch = 0
				lastSequence = 0
			}
		}
	}

	if shouldAuctionStart(epoch, lastEndEpoch) == false {
		log.Debug("no auction control txs needed", "height", height, "epoch", epoch, "lastEndEpoch", lastEndEpoch)
		return nil
	} else {
		log.Info("build auction control txs", "epoch", epoch, "lastEndEpoch", lastEndEpoch)
	}

	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestNum)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 10). // buffer for builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(12345678)

	if currentActive {
		builder.Clause(
			tx.NewClause(&meter.AuctionModuleAddr).
				WithValue(big.NewInt(0)).
				WithToken(meter.MTRG).
				WithData(buildAuctionStopData(auctionCB.StartHeight, auctionCB.StartEpoch, auctionCB.EndHeight, auctionCB.EndEpoch, auctionCB.Sequence, &auctionCB.AuctionID)))
	}

	startData := make([]byte, 0)
	if meter.IsTeslaFork11(uint32(height)) && lastSequence+1 > baseSequence.Uint64() {
		log.Info("Build Auction Start Data with Emission Curve")
		startData = buildAuctionStartDataAfterFork11(lastEndHeight+1, lastEndEpoch+1, height, epoch, lastSequence+1, auctionCB, baseSequence)
	}

	if len(startData) <= 0 {
		log.Info("Build Auction Start Data with Inflation")
		startData = buildAuctionStartData(lastEndHeight+1, lastEndEpoch+1, height, epoch, lastSequence+1, auctionCB)
	}

	builder.Clause(tx.NewClause(&meter.AuctionModuleAddr).WithValue(big.NewInt(0)).WithToken(meter.MTRG).WithData(startData))

	return builder.Build()
}

/*
**************
clear;
N =30;
Year = 100;
Len = Year*365;
Halving = 4*365;
AuctionPrice=zeros(1,Len);
DailyReward=zeros(1,Len);
ActualReward=zeros(1,Len);
Annual=zeros(1,Year);
Total = 0;
WeightedAvgPrice=zeros(1,Len);
for i=1:Len
    DailyReward(i) = 40000000/Halving*ln(1/0.8)*0.8^(i/Halving);
    Total=Total+DailyReward(i);
    n = idivide(i-1,int32(365))+1;
    Annual(n)=Annual(n)+DailyReward(i);
end
figure(1);
hold on;
plot(DailyReward);
hold off;
figure(2);
hold on;
plot(Annual);
hold off;
****************
*/

// DailyReward(i) = ln(1/0.8)*0.8^(i/Halving)*40000000/Halving
func DailyReward(i uint64) float64 {
	return math.Log(1/fadeRate) * math.Pow(fadeRate, (float64(i)/float64(halvingDays))) * 40000000 / halvingDays
}

func ComputeEpochReleaseWithEmissionCurve(sequence uint64, baseSequence uint64) (*big.Int, error) {
	i := sequence - baseSequence
	if i > 0 {
		result, _ := big.NewFloat(DailyReward(i)).Int(big.NewInt(0))
		return result, nil
	} else {
		return big.NewInt(0), errors.New("sequence < baseSequence, not valid for emission curve")
	}
}

// calEpochReleaseWithInflation returns the release of MTRG for current epoch, it returns a 0 if curEpoch is less than startEpoch
// epochRelease = lastEpochRelease + lastEpochRelease * deltaRate
// whereas, deltaRate = inflationRate / 365 / nAuctionPerDay
func ComputeEpochReleaseWithInflation(sequence uint64, lastAuction *meter.AuctionCB) (*big.Int, error) {
	fmt.Println("Compute MTRG release with inflation (new)")
	fmt.Println(fmt.Sprintf("  current sequence: %d", sequence))

	// deltaRate = inflationRate / 365 / nAuctionPerDay
	// notice: rate is in the unit of Wei
	deltaRate := new(big.Int).Div(big.NewInt(meter.AuctionReleaseInflation), big.NewInt(int64(meter.NAuctionPerDay)))
	deltaRate.Div(deltaRate, big.NewInt(365))

	fmt.Println("  delta rate: ", deltaRate)

	if lastAuction == nil || (lastAuction.StartHeight == 0 && lastAuction.EndHeight == 0 && lastAuction.RlsdMTRG == nil && lastAuction.StartEpoch == 0 && lastAuction.EndEpoch == 0) {
		fmt.Println("  first: ", true, "sequence: ", sequence)
		// initEpochRelease = MTRReleaseBase * 1e18 / deltaRate / 1e18
		initEpochRelease := new(big.Int).Mul(big.NewInt(meter.AuctionReleaseBase), UnitWei) // multiply base with 1e18
		initEpochRelease.Mul(initEpochRelease, deltaRate)
		initEpochRelease.Div(initEpochRelease, UnitWei)
		fmt.Println("  init release: ", initEpochRelease)
		return initEpochRelease, nil
	}
	// fmt.Println("last auction: ", lastAuction)

	lastRelease := big.NewInt(0)

	if lastAuction.RlsdMTRG != nil {
		lastRelease.Add(lastRelease, lastAuction.RlsdMTRG)
	}
	if lastAuction.RsvdMTRG != nil {
		lastRelease.Add(lastRelease, lastAuction.RsvdMTRG)
	}
	delta := new(big.Int).Mul(lastRelease, deltaRate)
	delta.Div(delta, UnitWei) // divided by Wei

	curRelease := new(big.Int).Add(lastRelease, delta)

	release := big.NewInt(0)
	for i := 0; uint64(i) < lastAuction.Sequence; i++ {
		release.Add(release, new(big.Int).Mul(big.NewInt(meter.AuctionReleaseBase), UnitWei))
		release.Mul(release, deltaRate)
		release.Div(release, UnitWei)
	}
	fmt.Println("  last sequence:", lastAuction.Sequence)
	fmt.Println("  last release:", lastRelease, " (calibrate):", release)

	release.Add(release, new(big.Int).Mul(big.NewInt(meter.AuctionReleaseBase), UnitWei))
	release.Mul(release, deltaRate)
	release.Div(release, UnitWei)
	fmt.Println("  current release:", curRelease, " (calibrate):", release)

	return curRelease, nil
}

func buildAuctionStartData(start, startEpoch, end, endEpoch, sequence uint64, auctionCB *meter.AuctionCB) (ret []byte) {
	var releaseBigInt *big.Int
	reserveBigInt := big.NewInt(0)
	release, err := ComputeEpochReleaseWithInflation(sequence, auctionCB)
	releaseBigInt = release
	if err != nil {
		panic("calculate reward with inflation failed" + err.Error())
	}

	body := &auction.AuctionBody{
		Opcode:        meter.OP_START,
		Version:       uint32(0),
		StartHeight:   start,
		StartEpoch:    startEpoch,
		EndHeight:     end,
		EndEpoch:      endEpoch,
		Sequence:      sequence,
		Amount:        releaseBigInt,
		ReserveAmount: reserveBigInt,
		Timestamp:     0,
		Nonce:         0,
	}
	ret, _ = script.EncodeScriptData(body)
	return
}

func buildAuctionStartDataAfterFork11(start, startEpoch, end, endEpoch, sequence uint64, auctionCB *meter.AuctionCB, baseSequence *big.Int) (ret []byte) {
	var releaseBigInt *big.Int
	reserveBigInt := big.NewInt(0)

	release, err := ComputeEpochReleaseWithEmissionCurve(sequence, baseSequence.Uint64())
	releaseBigInt = release
	if err != nil {
		log.Error("calculate reward with emission curve failed", "err", err)
		return nil
	}

	body := &auction.AuctionBody{
		Opcode:        meter.OP_START,
		Version:       uint32(0),
		StartHeight:   start,
		StartEpoch:    startEpoch,
		EndHeight:     end,
		EndEpoch:      endEpoch,
		Sequence:      sequence,
		Amount:        releaseBigInt,
		ReserveAmount: reserveBigInt,
		Timestamp:     0,
		Nonce:         0,
	}
	ret, _ = script.EncodeScriptData(body)
	return
}

func buildAuctionStopData(start, startEpoch, end, endEpoch, sequence uint64, id *meter.Bytes32) (ret []byte) {
	body := &auction.AuctionBody{
		Opcode:      meter.OP_STOP,
		Version:     uint32(0),
		StartHeight: start,
		StartEpoch:  startEpoch,
		EndHeight:   end,
		EndEpoch:    endEpoch,
		Sequence:    sequence,
		AuctionID:   *id,
		Timestamp:   0,
		Nonce:       0,
	}
	ret, _ = script.EncodeScriptData(body)
	return
}

// height is current kblock, lastKBlock is last one
// so if current > boundary && last < boundary, take actions
func shouldAuctionStart(curEpoch, lastEpoch uint64) bool {
	if (curEpoch > lastEpoch) && (curEpoch-lastEpoch) >= meter.NEpochPerAuction {
		return true
	}
	return false
}

// deprecated
// func getHistoryPrices(reservedPrice *big.Int, list *meter.AuctionSummaryList) *[N]float64 {
// 	var i int
// 	history := [N]float64{}

// 	list, err := auction.GetAuctionSummaryList()
// 	if err != nil {
// 		panic("get auction summary failed")
// 	}
// 	size := len(list.Summaries)
// 	fmt.Println("getHistoryPrices", "history size", size)

// 	var price *big.Int
// 	for i = 0; i < N; i++ {
// 		if size >= N {
// 			price = list.Summaries[size-1-i].RsvdPrice
// 		} else {
// 			// not enough history, fill partially
// 			if i < N-size {
// 				price = reservedPrice
// 			} else {
// 				price = list.Summaries[i-(N-size)].RsvdPrice
// 			}
// 		}
// 		price = new(big.Int).Div(price, big.NewInt(1e6))
// 		history[N-1-i] = float64(price.Int64()) / 1e12
// 	}
// 	fmt.Println("history price", history)
// 	return &history
// }

// func calcWeightedAvgPrice(history *[N]float64) float64 {
// 	var i int
// 	var denominator float64 = float64((N + 1) * N / 2)
// 	var WeightedAvgPrice float64

// 	for i = 1; i <= N; i++ {
// 		price := history[i-1] * float64(i) / denominator
// 		WeightedAvgPrice = WeightedAvgPrice + price
// 	}
// 	return WeightedAvgPrice
// }

// released MTRG for a speciefic range
// func calcRewardEpochRange(startEpoch, endEpoch uint64, initialRelease float64, reservedPrice *big.Int) (totalReward float64, totalUnrelease float64, epochRewards []float64, err error) {
// 	fmt.Println("Compute MTRG release (old)")
// 	var epoch uint64
// 	var epochReward float64

// 	rp := new(big.Int).Div(reservedPrice, big.NewInt(1e6))
// 	convertedRP := float64(rp.Int64()) / 1e12

// 	initReleasePerEpoch := float64(initialRelease / 24)

// 	epochRewards = make([]float64, 0)
// 	halving := fadeYears * 365 * 24
// 	err = nil

// 	history := getHistoryPrices(reservedPrice)
// 	weightedAvgPrice := calcWeightedAvgPrice(history)

// 	totalReserve := float64(0)
// 	for epoch = startEpoch; epoch <= endEpoch; epoch++ {
// 		releaseLimit := initReleasePerEpoch + initReleasePerEpoch*(weightedAvgPrice-convertedRP)/convertedRP

// 		reward := float64(totoalRelease) / float64(halving)
// 		reward = reward * math.Log(1/fadeRate) * math.Pow(fadeRate, (float64(epoch)/float64(halving)))
// 		reserve := float64(0)
// 		if reward > releaseLimit {
// 			epochReward = releaseLimit
// 			reserve = reward - releaseLimit
// 		} else {
// 			epochReward = reward
// 		}

// 		totalReward = totalReward + epochReward
// 		epochRewards = append(epochRewards, epochReward)
// 		totalReserve = totalReserve + reserve
// 	}

// 	fmt.Println("meter gov released", "amount", totalReward, "reserve", totalReserve, "startEpoch", startEpoch, "endEpoch", endEpoch)
// 	return
// }
