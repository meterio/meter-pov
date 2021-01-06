// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"fmt"
	"math"
	"math/big"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/auction"
)

const (
	totoalRelease = 160000000 //total released 160M MTRG
	totalYears    = 500       // 500 years
	fadeYears     = 6         // halve every 6 years
	fadeRate      = 0.8       // fade rate 0.8
	N             = 24        // history buffer size
)

/***************
clear;
Year = 500;
Len = Year*365;
Halving = 15*365;
DailyReward=zeros(1,Len);
Annual=zeros(1,Year);
Total = 0;
for i=1:Len
    DailyReward(i) = 400000000/Halving*log(1/0.8)*0.8^(i/Halving);
    Total=Total+DailyReward(i);
    n = idivide(i-1,int32(365))+1;
    Annual(n)=Annual(n)+DailyReward(i);
end
figure(1);
plot(DailyReward);
figure(2);
plot(Annual);
*****************/
func getHistoryPrices() *[N]float64 {
	var i int
	history := [N]float64{}
	reservedPrice := GetAuctionReservedPrice()

	list, err := auction.GetAuctionSummaryList()
	if err != nil {
		panic("get auction summary failed")
	}
	size := len(list.Summaries)
	fmt.Println("getHistoryPrices", "history size", size)

	var price *big.Int
	for i = 0; i < N; i++ {
		if size >= N {
			price = list.Summaries[size-1-i].RsvdPrice
		} else {
			// not enough history, fill partially
			if i < N-size {
				price = reservedPrice
			} else {
				price = list.Summaries[i-(N-size)].RsvdPrice
			}
		}
		price = big.NewInt(0).Div(price, big.NewInt(1e6))
		history[N-1-i] = float64(price.Int64()) / 1e12
	}
	fmt.Println("history price", history)
	return &history
}

func calcWeightedAvgPrice(history *[N]float64) float64 {
	var i int
	var denominator float64 = float64((N + 1) * N / 2)
	var WeightedAvgPrice float64

	for i = 1; i <= N; i++ {
		price := history[i-1] * float64(i) / denominator
		WeightedAvgPrice = WeightedAvgPrice + price
	}
	return WeightedAvgPrice
}

// released MTRG for a speciefic range
func CalcRewardEpochRange(startEpoch, endEpoch uint64) (totalReward float64, totalUnrelease float64, epochRewards []float64, err error) {
	var epoch uint64
	var epochReward float64
	var InitialRelease float64
	var ReservePrice float64

	rp := new(big.Int).Div(GetAuctionReservedPrice(), big.NewInt(1e6))
	ReservePrice = float64(rp.Int64()) / 1e12

	InitialRelease = GetAuctionInitialRelease() // initial is 1000 mtrg
	InitReleasePerEpoch := float64(InitialRelease / 24)

	epochRewards = make([]float64, 0)
	Halving := fadeYears * 365 * 24
	err = nil

	history := getHistoryPrices()
	weightedAvgPrice := calcWeightedAvgPrice(history)

	totalReserve := float64(0)
	for epoch = startEpoch; epoch <= endEpoch; epoch++ {
		ReleaseLimit := InitReleasePerEpoch + InitReleasePerEpoch*(weightedAvgPrice-ReservePrice)/ReservePrice

		reward := float64(totoalRelease) / float64(Halving)
		reward = reward * math.Log(1/fadeRate) * math.Pow(fadeRate, (float64(epoch)/float64(Halving)))
		reserve := float64(0)
		if reward > ReleaseLimit {
			epochReward = ReleaseLimit
			reserve = reward - ReleaseLimit
		} else {
			epochReward = reward
		}

		totalReward = totalReward + epochReward
		epochRewards = append(epochRewards, epochReward)
		totalReserve = totalReserve + reserve
	}

	fmt.Println("meter gov released", "amount", totalReward, "reserve", totalReserve, "startEpoch", startEpoch, "endEpoch", endEpoch)
	//fmt.Println("each epoch reward", epochRewards)
	return
}

func FloatToBigInt(val float64) *big.Int {
	fval := float64(val * 1e09)
	bigval := big.NewInt(int64(fval))
	return bigval.Mul(bigval, big.NewInt(1e09))
}

func GetAuctionReservedPrice() *big.Int {
	conR := GetConsensusGlobInst()
	if conR == nil {
		panic("get global consensus reactor failed")
	}

	best := conR.chain.BestBlock()
	state, err := conR.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		panic("get state failed")
	}

	return builtin.Params.Native(state).Get(meter.KeyAuctionReservedPrice)
}

func GetAuctionInitialRelease() float64 {
	conR := GetConsensusGlobInst()
	if conR == nil {
		panic("get global consensus reactor failed")
	}

	best := conR.chain.BestBlock()
	state, err := conR.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		panic("get state failed")
	}

	r := builtin.Params.Native(state).Get(meter.KeyAuctionInitRelease)
	r = r.Div(r, big.NewInt(1e09))
	fr := new(big.Float).SetInt(r)
	initRelease, accuracy := fr.Float64()
	initRelease = initRelease / (1e09)

	conR.logger.Info("get inital release", "value", initRelease, "accuracy", accuracy)
	return initRelease
}
