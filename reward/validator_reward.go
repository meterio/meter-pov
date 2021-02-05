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

// Epoch Reward includes these parts:
//                |------ extra_reward -----|
//  |------|   |------------|   |---------------|
//  | base | + | commission | + | actual_reward |  =  each_reward
//  |------|   |------------|   |---------------|
//
// such that:
// sum(extra_reward) = total_rewards - base * delegate_size
// commission = sum(extra_reward) * voting_power_ratio * commission_rate
// actual_reward = sum(extra_reward) * voting_power_ratio - commission
// delgate_reward = base + commission + actual_reward * delgate_share_ratio
// voter_reward = actual_reward * voter_share_ratio
//
// Steps:
// 1. distributes the base reward (meter.ValidatorBaseReward) for each validator. If there is remainning
// 2. get the propotion reward for each validator based on the votingpower
// 3. each validator takes commission first
// 4. finally, distributor takes their propotions of rest
func ComputeRewardMap(epochBaseReward, epochTotalRewards *big.Int, delegates []*types.Delegate) (RewardMap, error) {
	rewardMap := RewardMap{}

	fmt.Println("-----------------------------------------------------------------------")
	fmt.Println(fmt.Sprintf("Calculate Reward Map, baseRewards:%v, totalRewards:%v (for this epoch)", epochBaseReward, epochTotalRewards))
	fmt.Println("-----------------------------------------------------------------------")

	var i int
	baseRewardsOnly := false
	size := len(delegates)

	// distribute the base reward
	totalRewards := new(big.Int).Add(big.NewInt(0), epochTotalRewards)
	baseRewards := new(big.Int).Mul(epochBaseReward, big.NewInt(int64(size)))
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
				// autobidAmount = baseReward * Autobid / 100
				autobidAmount := new(big.Int).Mul(baseReward, big.NewInt(int64(d.Autobid)))
				autobidAmount.Div(autobidAmount, big.NewInt(100))

				// distAmount = baseReward - autobidAmount
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
		votingPowerSum.Add(votingPowerSum, big.NewInt(delegates[i].VotingPower))
	}

	// rewards = totalRewards - baseRewards
	rewards := new(big.Int).Sub(totalRewards, baseRewards)
	for i = 0; i < size; i++ {

		// calculate the propotion of each validator
		// eachReward = rewards * VotingPower / votingPowerSum
		eachReward := new(big.Int).Mul(rewards, big.NewInt(delegates[i].VotingPower))
		eachReward.Div(eachReward, votingPowerSum)

		// distribute commission to delegate, commission unit is shannon, aka, 1e09
		// commission = eachReward * Commission / 1e09
		commission := new(big.Int).Mul(eachReward, big.NewInt(int64(delegates[i].Commission)))
		commission.Div(commission, big.NewInt(1e09))

		// actualReward will be distributed to voters based on their shares
		// actualReward = eachReward - commission
		actualReward := new(big.Int).Sub(eachReward, commission)

		// delegateSelf = baseReward + commission
		delegateSelf := new(big.Int).Add(baseReward, commission)

		// plus base reward
		d, err := getSelfDistributor(delegates[i])
		if err != nil {
			logger.Error("get the autobid param failed, treat as 0", "error", err)
		} else {
			// delegate's proportion
			// selfPortion = actualReward * Shares / 1e09
			selfPortion := new(big.Int).Mul(actualReward, big.NewInt(int64(d.Shares)))
			selfPortion.Div(selfPortion, big.NewInt(1e09))

			// delegateSelf = delegateSelf + selfPortion
			delegateSelf = new(big.Int).Add(delegateSelf, selfPortion)

			// distribute delegate itself
			// autobidAmount = delegateSelf * Autobid / 100
			autobidAmount := new(big.Int).Mul(delegateSelf, big.NewInt(int64(d.Autobid)))
			autobidAmount.Div(autobidAmount, big.NewInt(100))

			// distAmount = delegateSelf - autobidAmount
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

			// voterReward = actualReward * Shares / 1e09
			voterReward := new(big.Int).Mul(actualReward, big.NewInt(int64(dist.Shares)))
			voterReward.Div(voterReward, big.NewInt(1e09))

			// autobidReward = voterReward * Autobid / 100
			autobidReward := new(big.Int).Mul(voterReward, big.NewInt(int64(dist.Autobid)))
			autobidReward.Div(autobidReward, big.NewInt(100))

			distReward := new(big.Int).Sub(voterReward, autobidReward)
			rewardMap.Add(distReward, autobidReward, dist.Address)
		}
	}
	logger.Info("distriubted validators rewards", "total", totalRewards.String())

	return rewardMap, nil
}

func ComputeEpochBaseReward(validatorBaseReward *big.Int) *big.Int {
	nEpochPerDay := 1
	if AuctionInterval < 24 {
		nEpochPerDay = int(24) / int(AuctionInterval)
	}
	fmt.Println("N Epoch Per Day: ", nEpochPerDay)
	epochBaseReward := new(big.Int).Div(validatorBaseReward, big.NewInt(int64(nEpochPerDay)))
	fmt.Println("Epoch Base Reward: ", epochBaseReward)
	return epochBaseReward
}

func ComputeEpochTotalReward(benefitRatio *big.Int) (*big.Int, error) {
	summaryList, err := auction.GetAuctionSummaryList()
	if err != nil {
		logger.Error("get summary list failed", "error", err)
		return big.NewInt(0), err
	}
	size := len(summaryList.Summaries)
	if size == 0 {
		return big.NewInt(0), nil
	}
	nEpochPerDay := 1
	if AuctionInterval < 24 {
		nEpochPerDay = int(24) / int(AuctionInterval)
	}

	var d, i int
	if size <= NDays*nEpochPerDay {
		d = size
	} else {
		d = NDays * nEpochPerDay
	}

	dailyReward := big.NewInt(0)
	for i = 0; i < d; i++ {
		s := summaryList.Summaries[size-1-i]
		dailyReward.Add(dailyReward, s.RcvdMTR)
	}

	// last NDay * nEpochPerDay auctions receved MTR * 40% / 240
	dailyReward = new(big.Int).Mul(dailyReward, benefitRatio)
	dailyReward.Div(dailyReward, big.NewInt(240))
	dailyReward.Div(dailyReward, big.NewInt(1e18))

	epochReward := new(big.Int).Div(dailyReward, big.NewInt(int64(nEpochPerDay)))
	logger.Info("final Kblock epoch rewards", "rewards", epochReward)
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
