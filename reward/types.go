package reward

import (
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/staking"
)

type RewardInfo struct {
	Address meter.Address
	Amount  *big.Int
}

func (r RewardInfo) String() string {
	return fmt.Sprintf("RewardInfo: %v Amount:%v", r.Address.String(), r.Amount)
}

//// RewardMap
type RewardMapInfo struct {
	Address       meter.Address
	DistAmount    *big.Int
	AutobidAmount *big.Int
}

func (r *RewardMapInfo) String() string {
	return fmt.Sprintf("Reward: %v Dist: %v, Autobid: %v", r.Address.String(), r.DistAmount.Uint64(), r.AutobidAmount.Uint64())
}

type RewardMap map[meter.Address]*RewardMapInfo

func (rmap RewardMap) Add(dist, autobid *big.Int, addr meter.Address) error {
	info, ok := rmap[addr]
	if ok == true {
		info.DistAmount = info.DistAmount.Add(info.DistAmount, dist)
		info.AutobidAmount = info.AutobidAmount.Add(info.AutobidAmount, autobid)
	} else {
		rmap[addr] = &RewardMapInfo{
			Address:       addr,
			DistAmount:    dist,
			AutobidAmount: autobid,
		}
	}
	return nil
}

func (rmap RewardMap) GetDistList() []*RewardInfo {
	list := make([]*RewardInfo, 0)
	for _, info := range rmap {
		if info.DistAmount.Sign() != 0 {
			list = append(list, &RewardInfo{Address: info.Address, Amount: info.DistAmount})
		}
	}
	return list
}

func (rmap RewardMap) GetAutobidList() []*RewardInfo {
	list := make([]*RewardInfo, 0)
	for _, info := range rmap {
		if info.AutobidAmount.Sign() != 0 {
			list = append(list, &RewardInfo{Address: info.Address, Amount: info.AutobidAmount})
		}
	}
	return list
}

func (rmap RewardMap) ToList() (*big.Int, *big.Int, []*RewardMapInfo) {
	rewards := []*RewardMapInfo{}
	distSum := big.NewInt(0)
	autobidSum := big.NewInt(0)

	var r big.Int

	for _, info := range rmap {
		distSum = r.Add(distSum, info.DistAmount)
		autobidSum = r.Add(autobidSum, info.AutobidAmount)
		rewards = append(rewards, info)
	}

	return distSum, autobidSum, rewards
}

type missingLeaderInfo struct {
	Address meter.Address
	Info    staking.MissingLeaderInfo
}

type missingProposerInfo struct {
	Address meter.Address
	Info    staking.MissingProposerInfo
}

type missingVoterInfo struct {
	Address meter.Address
	Info    staking.MissingVoterInfo
}

type doubleSignerInfo struct {
	Address meter.Address
	Info    staking.DoubleSignerInfo
}

type StatEntry struct {
	Address    meter.Address
	Name       string
	PubKey     string
	Infraction staking.Infraction
}

func (e StatEntry) String() string {
	return fmt.Sprintf("StatEntry(Addr:%s Name:%s PubKey:%s Infraction:%s)", e.Address.String(), e.Name, e.PubKey, e.Infraction.String())
}

func FloatToBigInt(val float64) *big.Int {
	fval := float64(val * 1e09)
	bigval := big.NewInt(int64(fval))
	return bigval.Mul(bigval, big.NewInt(1e09))
}
