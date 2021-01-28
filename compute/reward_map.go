package compute

import (
	"math/big"

	"github.com/dfinlab/meter/meter"
)



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
