package auction

import (
    "math"
    "math/big"
)

const (
    totoalRelease = 400000000 //total released 400M MTRG
    totalYears    = 500       // 500 years
    fadeYears     = 15        // halve every 15 years
    fadeRate      = 0.8       // fade rate 0.8
    blockInterval = 3         // block interval 3 s
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
// released MTRG for a speciefic range
func calcRewardRange(start, end uint64) (reward float64, dReward []float64, err error) {
    var i uint64
    var heightReward float64
    dReward = make([]float64, 0)
    Halving := fadeYears * 365 * 3600 * 24 / blockInterval
    err = nil

    for i = start; i < end; i++ {
        heightReward = float64(totoalRelease) / float64(Halving)
        heightReward = heightReward * math.Log(1/fadeRate) * math.Pow(fadeRate, (float64(i)/float64(Halving)))

        reward = reward + heightReward
        dReward = append(dReward, heightReward)
    }
    log.Info("meter gov released", "amount", reward, "start", start, "end", end)
    return
}

func FloatToBigInt(val float64) *big.Int {
    bigval := new(big.Float)
    bigval.SetFloat64(val)

    coin := new(big.Float)
    coin.SetInt(big.NewInt(1000000000000000000))
    bigval.Mul(bigval, coin)

    result := new(big.Int)
    result, accuracy := bigval.Int(result)
    log.Debug("big int", "value", result, "accuracy", accuracy)
    return result
}
