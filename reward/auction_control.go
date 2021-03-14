// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package reward

import (
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"time"

	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/auction"
	"github.com/dfinlab/meter/tx"
	"github.com/ethereum/go-ethereum/rlp"
)

func BuildAuctionControlTx(height, epoch uint64, chainTag byte, bestNum uint32, initialRelease float64, reservedPrice *big.Int, chain *chain.Chain) *tx.Transaction {
	// check current active auction first if there is one
	var currentActive bool
	cb, err := auction.GetActiveAuctionCB()
	if err != nil {
		logger.Error("get auctionCB failed ...", "error", err)
		return nil
	}
	if cb.IsActive() == true {
		currentActive = true
	}

	// now start a new auction
	var lastEndHeight, lastEndEpoch, lastSequence uint64
	if currentActive == true {
		lastEndHeight = cb.EndHeight
		lastEndEpoch = cb.EndEpoch
		lastSequence = cb.Sequence
	} else {
		summaryList, err := auction.GetAuctionSummaryList()
		if err != nil {
			logger.Error("get summary list failed", "error", err)
			return nil //TBD: still create Tx?
		}
		size := len(summaryList.Summaries)
		if size != 0 {
			lastEndHeight = summaryList.Summaries[size-1].EndHeight
			lastEndEpoch = summaryList.Summaries[size-1].EndEpoch
			lastSequence = summaryList.Summaries[size-1].Sequence
		} else {
			if meter.IsMainChainTesla(uint32(height)) {
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
		logger.Debug("no auction Tx in the kblock ...", "height", height, "epoch", epoch)
		return nil
	}

	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestNum)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 10). // buffer for builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(12345678)

	if currentActive == true {
		builder.Clause(
			tx.NewClause(&auction.AuctionAccountAddr).
				WithValue(big.NewInt(0)).
				WithToken(meter.MTRG).
				WithData(buildAuctionStopData(cb.StartHeight, cb.StartEpoch, cb.EndHeight, cb.EndEpoch, cb.Sequence, &cb.AuctionID)))
	}

	builder.Clause(
		tx.NewClause(&auction.AuctionAccountAddr).
			WithValue(big.NewInt(0)).
			WithToken(meter.MTRG).
			WithData(buildAuctionStartData(lastEndHeight+1, lastEndEpoch+1, height, epoch, lastSequence+1, initialRelease, reservedPrice)))

	logger.Info("Auction Tx Built", "Height", height, "epoch", epoch)
	return builder.Build()
}

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
func getHistoryPrices(reservedPrice *big.Int) *[N]float64 {
	var i int
	history := [N]float64{}

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
		price = new(big.Int).Div(price, big.NewInt(1e6))
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

// calEpochReleaseWithInflation returns the release of MTRG for current epoch, it returns a 0 if curEpoch is less than startEpoch
// epochRelease = lastEpochRelease + lastEpochRelease * deltaRate
// whereas, deltaRate = inflationRate / 365 / nAuctionPerDay
func ComputeEpochReleaseWithInflation(sequence uint64, lastAuction *auction.AuctionCB) (*big.Int, error) {
	fmt.Println("Compute MTRG release with inflation (new)")
	fmt.Println(fmt.Sprintf("auction Sequence:%d", sequence))

	// deltaRate = inflationRate / 365 / nAuctionPerDay
	// notice: rate is in the unit of Wei
	deltaRate := new(big.Int).Div(big.NewInt(meter.AuctionReleaseInflation), big.NewInt(int64(meter.NAuctionPerDay)))
	deltaRate.Div(deltaRate, big.NewInt(365))

	fmt.Println("delta rate: ", deltaRate)

	if lastAuction == nil || (lastAuction.StartHeight == 0 && lastAuction.EndHeight == 0 && lastAuction.RlsdMTRG == nil && lastAuction.StartEpoch == 0 && lastAuction.EndEpoch == 0) {
		fmt.Println("first: ", true, "sequence: ", sequence)
		// initEpochRelease = MTRReleaseBase * 1e18 / deltaRate / 1e18
		initEpochRelease := new(big.Int).Mul(big.NewInt(meter.AuctionReleaseBase), UnitWei) // multiply base with 1e18
		initEpochRelease.Mul(initEpochRelease, deltaRate)
		initEpochRelease.Div(initEpochRelease, UnitWei)
		fmt.Println("init release: ", initEpochRelease)
		return initEpochRelease, nil
	}
	fmt.Println("last auction: ", lastAuction)

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
	fmt.Println("Sequence = ", lastAuction.Sequence)
	fmt.Println("last release:", lastRelease, " (calibrate):", release)

	release.Add(release, new(big.Int).Mul(big.NewInt(meter.AuctionReleaseBase), UnitWei))
	release.Mul(release, deltaRate)
	release.Div(release, UnitWei)
	fmt.Println("current release:", curRelease, " (calibrate):", release)

	return curRelease, nil
}

// released MTRG for a speciefic range
func calcRewardEpochRange(startEpoch, endEpoch uint64, initialRelease float64, reservedPrice *big.Int) (totalReward float64, totalUnrelease float64, epochRewards []float64, err error) {
	fmt.Println("Compute MTRG release (old)")
	var epoch uint64
	var epochReward float64

	rp := new(big.Int).Div(reservedPrice, big.NewInt(1e6))
	convertedRP := float64(rp.Int64()) / 1e12

	initReleasePerEpoch := float64(initialRelease / 24)

	epochRewards = make([]float64, 0)
	halving := fadeYears * 365 * 24
	err = nil

	history := getHistoryPrices(reservedPrice)
	weightedAvgPrice := calcWeightedAvgPrice(history)

	totalReserve := float64(0)
	for epoch = startEpoch; epoch <= endEpoch; epoch++ {
		releaseLimit := initReleasePerEpoch + initReleasePerEpoch*(weightedAvgPrice-convertedRP)/convertedRP

		reward := float64(totoalRelease) / float64(halving)
		reward = reward * math.Log(1/fadeRate) * math.Pow(fadeRate, (float64(epoch)/float64(halving)))
		reserve := float64(0)
		if reward > releaseLimit {
			epochReward = releaseLimit
			reserve = reward - releaseLimit
		} else {
			epochReward = reward
		}

		totalReward = totalReward + epochReward
		epochRewards = append(epochRewards, epochReward)
		totalReserve = totalReserve + reserve
	}

	fmt.Println("meter gov released", "amount", totalReward, "reserve", totalReserve, "startEpoch", startEpoch, "endEpoch", endEpoch)
	return
}

func buildAuctionStartData(start, startEpoch, end, endEpoch, sequence uint64, initialRelease float64, reservedPrice *big.Int) (ret []byte) {
	ret = []byte{}

	var releaseBigInt *big.Int
	reserveBigInt := big.NewInt(0)
	lastAuction, err := auction.GetActiveAuctionCB()
	if err != nil {
		fmt.Println("could not get last auction")
	}
	release, err := ComputeEpochReleaseWithInflation(sequence, lastAuction)
	releaseBigInt = release
	if err != nil {
		panic("calculate reward with inflation failed" + err.Error())
	}
	// release, reserve, _, err := calcRewardEpochRange(startEpoch, endEpoch, initialRelease, reservedPrice)
	// if err != nil {
	// 	panic("calculate reward failed" + err.Error())
	// }

	// releaseBigInt = FloatToBigInt(release)
	// reserveBigInt = FloatToBigInt(reserve)

	body := &auction.AuctionBody{
		Opcode:        auction.OP_START,
		Version:       uint32(0),
		StartHeight:   start,
		StartEpoch:    startEpoch,
		EndHeight:     end,
		EndEpoch:      endEpoch,
		Sequence:      sequence,
		Amount:        releaseBigInt,
		ReserveAmount: reserveBigInt,
		Timestamp:     uint64(time.Now().Unix()),
		Nonce:         rand.Uint64(),
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		fmt.Println("BuildAuctionStart auction error", err.Error())
		return
	}

	// fmt.Println("Payload Hex: ", hex.EncodeToString(payload))
	s := &script.Script{
		Header: script.ScriptHeader{
			Version: uint32(0),
			ModID:   script.AUCTION_MODULE_ID,
		},
		Payload: payload,
	}
	data, err := rlp.EncodeToBytes(s)
	if err != nil {
		fmt.Println("BuildAuctionStart script error", err.Error())
		return
	}
	data = append(script.ScriptPattern[:], data...)
	prefix := []byte{0xff, 0xff, 0xff, 0xff}
	ret = append(prefix, data...)
	//fmt.Println("auction start script Hex:", hex.EncodeToString(ret))
	return
}

func buildAuctionStopData(start, startEpoch, end, endEpoch, sequence uint64, id *meter.Bytes32) (ret []byte) {
	ret = []byte{}

	body := &auction.AuctionBody{
		Opcode:      auction.OP_STOP,
		Version:     uint32(0),
		StartHeight: start,
		StartEpoch:  startEpoch,
		EndHeight:   end,
		EndEpoch:    endEpoch,
		Sequence:    sequence,
		AuctionID:   *id,
		Timestamp:   uint64(time.Now().Unix()),
		Nonce:       rand.Uint64(),
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		fmt.Println("BuildAuctionStop auction error", err.Error())
		return
	}

	// fmt.Println("Payload Hex: ", hex.EncodeToString(payload))
	s := &script.Script{
		Header: script.ScriptHeader{
			Version: uint32(0),
			ModID:   script.AUCTION_MODULE_ID,
		},
		Payload: payload,
	}
	data, err := rlp.EncodeToBytes(s)
	if err != nil {
		fmt.Println("BuildAuctionStop script error", err.Error())
		return
	}
	data = append(script.ScriptPattern[:], data...)
	prefix := []byte{0xff, 0xff, 0xff, 0xff}
	ret = append(prefix, data...)
	//fmt.Println("auction stop script Hex:", hex.EncodeToString(ret))
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
