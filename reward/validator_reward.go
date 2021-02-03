// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package reward

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/script/auction"
	"github.com/dfinlab/meter/types"
)

//***************************************
//**********validator Rewards ***********

//1. distributes the base reward (meter.ValidatorBaseReward) for each validator. If there is remainning
//2. get the propotion reward for each validator based on the votingpower
//3. each validator takes commission first
//4. finally, distributor takes their propotions of rest
func ComputeRewardMap(benefitRatio *big.Int, validatorBaseReward *big.Int, delegates []*types.Delegate) (RewardMap, error) {
	rewardMap := RewardMap{}

	totalReward, err := ComputeEpochReward(benefitRatio)

	fmt.Println("-----------------------------------------------------------------------")
	fmt.Println(fmt.Sprintf("Calculate Reward Map, benefitRatio:%v%%, baseRewards:%v, totalRewards:%v", float64(new(big.Int).Div(benefitRatio, big.NewInt(1e16)).Int64())/100, validatorBaseReward, totalReward))
	fmt.Println("-----------------------------------------------------------------------")

	if err != nil {
		logger.Error("calculate validator reward failed")
		return rewardMap, err
	}

	rewardMap, err = ComputeValidatorRewards(validatorBaseReward, totalReward, delegates)

	logger.Info("**** Dist List")
	for _, d := range rewardMap.GetDistList() {
		fmt.Println(d.String())
	}
	logger.Info("**** Autobid List")
	for _, a := range rewardMap.GetAutobidList() {
		fmt.Println(a.String())
	}
	return rewardMap, err
}

func ComputeEpochReward(benefitRatio *big.Int) (*big.Int, error) {
	summaryList, err := auction.GetAuctionSummaryList()
	if err != nil {
		logger.Error("get summary list failed", "error", err)
		return big.NewInt(0), err
	}
	size := len(summaryList.Summaries)
	if size == 0 {
		return big.NewInt(0), nil
	}

	var d, i int
	if size <= NDays {
		d = size
	} else {
		d = NDays
	}

	nEpochPerDay := 1
	if AuctionInterval < 24 {
		nEpochPerDay = int(24) / int(AuctionInterval)
	}
	size = size * nEpochPerDay

	dailyReward := big.NewInt(0)
	for i = 0; i < d; i++ {
		s := summaryList.Summaries[size-1-i]
		dailyReward.Add(dailyReward, s.RcvdMTR)
	}

	// last 10 auctions receved MTR * 40% / 240
	dailyReward = new(big.Int).Mul(dailyReward, benefitRatio)
	dailyReward.Div(dailyReward, big.NewInt(1e18))

	epochReward := new(big.Int).Div(dailyReward, big.NewInt(int64(nEpochPerDay)))
	fmt.Println("final total reward: ", epochReward)

	logger.Info("get Kblock validator rewards", "rewards", epochReward)
	return epochReward, nil
}

func getSelfDistributor(delegate *types.Delegate) (*types.Distributor, error) {
	for _, dist := range delegate.DistList {
		if dist.Address == delegate.Address {
			return dist, nil
		}
	}
	return nil, errors.New("distributor not found")
}

//1. distributes the base reward (meter.ValidatorBaseReward) for each validator. If there is remainning
//2. get the propotion reward for each validator based on the votingpower
//3. each validator takes commission first
//4. finally, distributor takes their propotions of rest
func ComputeValidatorRewards(validatorBaseReward *big.Int, totalRewards *big.Int, delegates []*types.Delegate) (RewardMap, error) {
	var i int
	rewardMap := RewardMap{}
	baseRewardsOnly := false
	size := len(delegates)

	// distribute the base reward
	baseRewards := new(big.Int).Mul(validatorBaseReward, big.NewInt(int64(size)))
	if baseRewards.Cmp(totalRewards) >= 0 {
		baseRewards = totalRewards
		baseRewardsOnly = true
	}
	baseReward := new(big.Int).Div(baseRewards, big.NewInt(int64(size)))

	// only enough for base reward
	if baseRewardsOnly == true {
		for i = 0; i < size; i++ {
			d, err := getSelfDistributor(delegates[i])
			if err != nil {
				logger.Error("get self-distributor failed, treat as 0", "error", err)
				rewardMap.Add(baseReward, big.NewInt(0), delegates[i].Address)
			} else {
				autobidAmount := new(big.Int).Mul(baseReward, big.NewInt(int64(d.Autobid)))
				autobidAmount = new(big.Int).Div(autobidAmount, big.NewInt(100))

				distAmount := new(big.Int).Sub(baseReward, autobidAmount)
				rewardMap.Add(distAmount, autobidAmount, delegates[i].Address)
			}

		}

		return rewardMap, nil
	}

	// distributes the remaining. The distributing is based on
	// propotion of voting power
	votingPowerSum := big.NewInt(0)
	for i = 0; i < size; i++ {
		votingPowerSum = new(big.Int).Add(votingPowerSum, big.NewInt(delegates[i].VotingPower))
	}

	rewards := new(big.Int).Sub(totalRewards, baseRewards)
	fmt.Println(rewards)
	for i = 0; i < size; i++ {

		// calculate the propotion of each validator
		eachReward := new(big.Int).Mul(rewards, big.NewInt(delegates[i].VotingPower))
		eachReward = new(big.Int).Div(eachReward, votingPowerSum)

		// distribute commission to delegate, commission unit is shannon, aka, 1e09
		commission := new(big.Int).Mul(eachReward, big.NewInt(int64(delegates[i].Commission)))
		commission = new(big.Int).Div(commission, big.NewInt(1e09))

		actualReward := new(big.Int).Sub(eachReward, commission)

		delegateSelf := new(big.Int).Add(baseReward, commission)

		// plus base reward
		d, err := getSelfDistributor(delegates[i])
		if err != nil {
			logger.Error("get the autobid param failed, treat as 0", "error", err)
		} else {
			// delegate's proportion
			selfPortion := new(big.Int).Mul(actualReward, big.NewInt(int64(d.Shares)))
			selfPortion = new(big.Int).Div(selfPortion, big.NewInt(1e09))
			delegateSelf = new(big.Int).Add(delegateSelf, selfPortion)

			// distribute delegate itself
			autobidAmount := new(big.Int).Mul(delegateSelf, big.NewInt(int64(d.Autobid)))
			autobidAmount = new(big.Int).Div(autobidAmount, big.NewInt(100))
			distAmount := new(big.Int).Sub(delegateSelf, autobidAmount)
			rewardMap.Add(distAmount, autobidAmount, delegates[i].Address)
		}

		// now distributes actualReward (remaining part) to each distributor
		// as percentage to each distributor， the unit of Shares is shannon， ie， 1e09
		for _, dist := range delegates[i].DistList {
			// delegate self already distributed, skip
			if dist.Address == delegates[i].Address {
				continue
			}

			voterReward := new(big.Int).Mul(actualReward, big.NewInt(int64(dist.Shares)))
			voterReward = new(big.Int).Div(voterReward, big.NewInt(1e09))

			autobidReward := new(big.Int).Mul(voterReward, big.NewInt(int64(dist.Autobid)))
			autobidReward = new(big.Int).Div(autobidReward, big.NewInt(100))
			distReward := new(big.Int).Sub(voterReward, autobidReward)
			rewardMap.Add(distReward, autobidReward, dist.Address)
		}
	}
	logger.Info("distriubted validators rewards", "total", totalRewards.String())

	return rewardMap, nil
}
