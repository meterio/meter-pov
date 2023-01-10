package governor

import (
	"bytes"
	"fmt"
	"math/big"
	"sort"

	"github.com/meterio/meter-pov/meter"
)

type RewardMap map[meter.Address]*meter.RewardInfoV2

func (rmap RewardMap) Add(dist, autobid *big.Int, addr meter.Address) error {
	info, ok := rmap[addr]
	if ok == true {
		info.DistAmount = info.DistAmount.Add(info.DistAmount, dist)
		info.AutobidAmount = info.AutobidAmount.Add(info.AutobidAmount, autobid)
	} else {
		rmap[addr] = &meter.RewardInfoV2{
			Address:       addr,
			DistAmount:    dist,
			AutobidAmount: autobid,
		}
	}
	return nil
}

func (rmap RewardMap) GetDistList() []*meter.RewardInfo {
	list := make([]*meter.RewardInfo, 0)
	for _, info := range rmap {
		if info.DistAmount.Sign() != 0 {
			list = append(list, &meter.RewardInfo{Address: info.Address, Amount: info.DistAmount})
		}
	}
	return list
}

func (rmap RewardMap) GetAutobidList() []*meter.RewardInfo {
	list := make([]*meter.RewardInfo, 0)
	for _, info := range rmap {
		if info.AutobidAmount.Sign() != 0 {
			list = append(list, &meter.RewardInfo{Address: info.Address, Amount: info.AutobidAmount})
		}
	}
	return list
}

func (rmap RewardMap) ToList() (*big.Int, *big.Int, []*meter.RewardInfoV2) {
	rewards := []*meter.RewardInfoV2{}
	distSum := big.NewInt(0)
	autobidSum := big.NewInt(0)

	var r big.Int

	for _, info := range rmap {
		distSum = r.Add(distSum, info.DistAmount)
		autobidSum = r.Add(autobidSum, info.AutobidAmount)
		rewards = append(rewards, info)
	}

	sort.SliceStable(rewards, func(i, j int) bool {
		return bytes.Compare(rewards[i].Address[:], rewards[j].Address[:]) <= 0
	})

	return distSum, autobidSum, rewards
}

type missingLeaderInfo struct {
	Address meter.Address
	Info    meter.MissingLeaderInfo
}

type missingProposerInfo struct {
	Address meter.Address
	Info    meter.MissingProposerInfo
}

type missingVoterInfo struct {
	Address meter.Address
	Info    meter.MissingVoterInfo
}

type doubleSignerInfo struct {
	Address meter.Address
	Info    meter.DoubleSignerInfo
}

type StatEntry struct {
	Address    meter.Address
	Name       string
	PubKey     string
	Infraction meter.Infraction
}

func (e StatEntry) String() string {
	return fmt.Sprintf("StatEntry(Addr:%s Name:%s PubKey:%s Infraction:%s)", e.Address.String(), e.Name, e.PubKey, e.Infraction.String())
}

func FloatToBigInt(val float64) *big.Int {
	fval := float64(val * 1e09)
	bigval := big.NewInt(int64(fval))
	return bigval.Mul(bigval, big.NewInt(1e09))
}
