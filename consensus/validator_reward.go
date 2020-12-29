// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"math/big"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/auction"
	"github.com/dfinlab/meter/types"
)

const (
	MAX_VALIDATOR_REWARDS = 1200
)

type RewardInfo struct {
	Address meter.Address
	Amount  *big.Int
}

//// RewardInfoMap
type RewardMapInfo struct {
	Address       meter.Address
	DistAmount    *big.Int
	AutobidAmount *big.Int
}

type RewardInfoMap map[meter.Address]*RewardMapInfo

func (rmap RewardInfoMap) Add(dist, autobid *big.Int, addr meter.Address) error {
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

func (rmap RewardInfoMap) ToList() (*big.Int, *big.Int, []*RewardMapInfo) {
	rewards := []*RewardMapInfo{}
	distSum := big.NewInt(0)
	autobidSum := big.NewInt(0)

	for _, info := range rmap {
		distSum = distSum.Add(distSum, info.DistAmount)
		autobidSum = autobidSum.Add(autobidSum, info.AutobidAmount)
		rewards = append(rewards, info)
	}

	return distSum, autobidSum, rewards
}

//***************************************
//**********validator Rewards ***********
const N = 10 // smooth with 10 days

func (conR *ConsensusReactor) calcKblockValidatorRewards() (*big.Int, error) {
	state, err := conR.stateCreator.NewState(conR.chain.BestBlock().Header().StateRoot())
	if err != nil {
		conR.logger.Error("new state failed ...", "error", err)
		return big.NewInt(0), err
	}
	ValidatorBenefitRatio := builtin.Params.Native(state).Get(meter.KeyValidatorBenefitRatio)

	summaryList, err := auction.GetAuctionSummaryList()
	if err != nil {
		conR.logger.Error("get summary list failed", "error", err)
		return big.NewInt(0), err
	}

	size := len(summaryList.Summaries)
	if size == 0 {
		return big.NewInt(0), nil
	}

	var d, i int
	if size <= N {
		d = size
	} else {
		d = N
	}

	rewards := big.NewInt(0)
	for i = 0; i < d; i++ {
		reward := summaryList.Summaries[size-1-i].RcvdMTR
		rewards = rewards.Add(rewards, reward)
	}

	// last 10 auctions receved MTR * 40% / 240
	rewards = rewards.Mul(rewards, ValidatorBenefitRatio)
	rewards = rewards.Div(rewards, big.NewInt(1e18))
	rewards = rewards.Div(rewards, big.NewInt(int64(240)))

	conR.logger.Info("get Kblock validator rewards", "rewards", rewards)
	return rewards, nil
}

//1. distributes the base reward (meter.ValidatorBaseReward) for each validator. If there is remainning
//2. get the propotion reward for each validator based on the votingpower
//3. each validator takes commission first
//4. finally, distributor takes their propotions of rest
func (conR *ConsensusReactor) CalcValidatorRewards(totalReward *big.Int, delegates []*types.Delegate) (*big.Int, *big.Int, []*RewardMapInfo, error) {

	// no distribute
	if conR.sourceDelegates == fromDelegatesFile {
		return big.NewInt(0), big.NewInt(0), []*RewardMapInfo{}, nil
	}

	rewardMap := RewardInfoMap{}
	var baseRewardsOnly bool
	size := len(delegates)

	// distribute the base reward
	state, err := conR.stateCreator.NewState(conR.chain.BestBlock().Header().StateRoot())
	if err != nil {
		panic("get state failed")
	}

	validatorBaseReward := builtin.Params.Native(state).Get(meter.KeyValidatorBaseReward)
	baseRewards := new(big.Int).Mul(validatorBaseReward, big.NewInt(int64(size)))
	if baseRewards.Cmp(totalReward) >= 0 {
		baseRewards = totalReward
		baseRewardsOnly = true
	}

	// base reward do not autobid
	var i int
	baseReward := new(big.Int).Div(baseRewards, big.NewInt(int64(size)))
	for i = 0; i < size; i++ {
		rewardMap.Add(baseReward, big.NewInt(0), delegates[i].Address)
	}

	if baseRewardsOnly == true {
		distSum, autobidSum, mapInfo := rewardMap.ToList()
		conR.logger.Info("validator dist rewards", "distributed", distSum, "autobid", autobidSum)
		return distSum, autobidSum, mapInfo, nil
	}

	// distributes the remaining. The distributing is based on
	// propotion of voting power
	votingPowerSum := big.NewInt(0)
	for i = 0; i < size; i++ {
		votingPowerSum = votingPowerSum.Add(votingPowerSum, big.NewInt(delegates[i].VotingPower))
	}

	distReward := big.NewInt(0)
	autobidReward := big.NewInt(0)
	commission := big.NewInt(0)
	rewards := new(big.Int).Sub(totalReward, baseRewards)
	for i = 0; i < size; i++ {

		// calculate the propotion of each validator
		eachReward := new(big.Int).Mul(rewards, big.NewInt(delegates[i].VotingPower))
		eachReward = eachReward.Div(eachReward, votingPowerSum)

		// distribute commission to delegate, commission unit is shannon, aka, 1e09
		commission = commission.Mul(eachReward, big.NewInt(int64(delegates[i].Commission)))
		commission = commission.Div(commission, big.NewInt(1e09))
		rewardMap.Add(commission, big.NewInt(0), delegates[i].Address)

		actualReward := new(big.Int).Sub(eachReward, commission)

		// now distributes actualReward to each distributor
		if len(delegates[i].DistList) == 0 {
			// no distributor, 100% goes to benefiicary
			rewardMap.Add(actualReward, big.NewInt(0), delegates[i].Address)
		} else {
			// as percentage to each distributor， the unit of Shares is shannon， ie， 1e09
			for _, dist := range delegates[i].DistList {
				r := new(big.Int).Mul(actualReward, big.NewInt(int64(dist.Shares)))
				r = r.Div(r, big.NewInt(1e09))

				autobidReward = r.Mul(r, big.NewInt(int64(dist.Autobid)))
				autobidReward = autobidReward.Div(autobidReward, big.NewInt(100))
				distReward = r.Sub(r, autobidReward)
				rewardMap.Add(distReward, autobidReward, dist.Address)
			}
		}
	}
	conR.logger.Info("distriubted validators rewards", "total", totalReward.String())

	distSum, autobidSum, mapInfo := rewardMap.ToList()
	conR.logger.Info("validator dist rewards", "distributed", distSum, "autobid", autobidSum)
	return distSum, autobidSum, mapInfo, nil
}

func (conR *ConsensusReactor) GetValidatorRewards(totalReward *big.Int, delegates []*types.Delegate) ([]*RewardInfo, []*RewardInfo, error) {
	distSum, autobidSum, rMapList, err := conR.CalcValidatorRewards(totalReward, delegates)
	if err != nil {
		return []*RewardInfo{}, []*RewardInfo{}, err
	}

	conR.logger.Info("validator dist rewards", "distributed", distSum, "autobid", autobidSum)

	dist := []*RewardInfo{}
	autobid := []*RewardInfo{}
	for _, r := range rMapList {
		if r.DistAmount.Sign() != 0 {
			dist = append(dist, &RewardInfo{r.Address, r.DistAmount})
		}

		if r.AutobidAmount.Sign() != 0 {
			autobid = append(autobid, &RewardInfo{r.Address, r.AutobidAmount})
		}
	}

	return dist, autobid, nil
}
