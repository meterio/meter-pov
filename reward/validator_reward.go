// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package reward

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/auction"
	"github.com/dfinlab/meter/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func ComputeRewardMapV3(baseReward, totalRewards *big.Int, delegates []*types.Delegate, committeeMembers []*types.Validator) (RewardMap, error) {
	memberKeys := make([][]byte, 0)
	for _, m := range committeeMembers {
		keyBytes := crypto.FromECDSAPub(&m.PubKey)
		memberKeys = append(memberKeys, keyBytes)
	}

	memberDelegates := make([]*types.Delegate, 0)
	for _, d := range delegates {
		keyBytes := crypto.FromECDSAPub(&d.PubKey)
		for _, kb := range memberKeys {
			if bytes.Compare(kb, keyBytes) == 0 {
				memberDelegates = append(memberDelegates, d)
				break
			}
		}
	}
	return ComputeRewardMap(baseReward, totalRewards, memberDelegates, true)
}

func ComputeRewardMapV2(baseReward, totalRewards *big.Int, delegates []*types.Delegate, committeeMembers []*types.Validator) (RewardMap, error) {
	memberKeys := make([][]byte, 0)
	for _, m := range committeeMembers {
		keyBytes := crypto.FromECDSAPub(&m.PubKey)
		memberKeys = append(memberKeys, keyBytes)
	}

	memberDelegates := make([]*types.Delegate, 0)
	for _, d := range delegates {
		keyBytes := crypto.FromECDSAPub(&d.PubKey)
		for _, kb := range memberKeys {
			if bytes.Compare(kb, keyBytes) == 0 {
				memberDelegates = append(memberDelegates, d)
				break
			}
		}
	}
	return ComputeRewardMap(baseReward, totalRewards, memberDelegates, false)
}

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
func ComputeRewardMap(baseReward, totalRewards *big.Int, delegates []*types.Delegate, v3 bool) (RewardMap, error) {
	rewardMap := RewardMap{}
	size := len(delegates)
	baseRewards := new(big.Int).Mul(baseReward, big.NewInt(int64(size)))

	fmt.Println("-----------------------------------------------------------------------")
	fmt.Println(fmt.Sprintf("Calculate Reward Map, baseRewards:%v, totalRewards:%v (for this epoch)", baseRewards, totalRewards))
	fmt.Println(fmt.Sprintf("baseReward per member:%v, size:%v", baseReward, size))
	fmt.Println("-----------------------------------------------------------------------")

	var i int
	baseRewardsOnly := false

	// distribute the base reward
	if baseRewards.Cmp(totalRewards) >= 0 {
		baseRewards = totalRewards
		baseRewardsOnly = true
	}
	baseReward = new(big.Int).Div(baseRewards, big.NewInt(int64(size)))

	// only enough for base reward
	if baseRewardsOnly == true {
		for i = 0; i < size; i++ {
			var d *types.Distributor
			var err error
			if v3 {
				d, err = getSelfDistributorV3(delegates[i])
			} else {
				d, err = getSelfDistributor(delegates[i])
			}
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
		var d *types.Distributor
		var err error
		if v3 {
			d, err = getSelfDistributorV3(delegates[i])
		} else {
			d, err = getSelfDistributor(delegates[i])
		}
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
	fmt.Println("NEpochPerDay: ", meter.NEpochPerDay)
	epochBaseReward := new(big.Int).Div(validatorBaseReward, big.NewInt(int64(meter.NEpochPerDay)))
	fmt.Println("Epoch base reward: ", epochBaseReward)
	return epochBaseReward
}

func ComputeEpochTotalReward(benefitRatio *big.Int, nDays int, nAuctionPerDay int) (*big.Int, error) {
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
	if size <= nDays*nAuctionPerDay {
		d = size
	} else {
		d = nDays * nAuctionPerDay
	}

	// sumReward = sum(receivedMTR in last NDays)
	sumReward := big.NewInt(0)
	for i = 0; i < d; i++ {
		s := summaryList.Summaries[size-1-i]
		fmt.Println("Use auction summary: ", s.AuctionID)
		sumReward.Add(sumReward, s.RcvdMTR)
	}
	fmt.Println("sumReward: ", sumReward.String(), "benefitRatio", benefitRatio)

	// epochTotalRewards = sumReward * benefitRatio / NDays / NEpochPerDay
	epochTotalRewards := new(big.Int).Mul(sumReward, benefitRatio)
	epochTotalRewards.Div(epochTotalRewards, big.NewInt(int64(nDays)))
	epochTotalRewards.Div(epochTotalRewards, big.NewInt(int64(meter.NEpochPerDay)))
	epochTotalRewards.Div(epochTotalRewards, big.NewInt(1e18))

	fmt.Println("Epoch total rewards:", epochTotalRewards)
	return epochTotalRewards, nil
}

func getSelfDistributor(delegate *types.Delegate) (*types.Distributor, error) {
	for _, dist := range delegate.DistList {
		if dist.Address == delegate.Address {
			return dist, nil
		}
	}
	return nil, errors.New("distributor not found")
}

func getSelfDistributorV3(delegate *types.Delegate) (*types.Distributor, error) {
	shares := uint64(0)
	autobid := uint8(0)

	for _, dist := range delegate.DistList {
		if dist.Address == delegate.Address {
			if dist.Autobid > autobid {
				autobid = dist.Autobid
			}
			shares = shares + dist.Shares
		}
	}
	if shares > 0 {
		return &types.Distributor{
			Address: delegate.Address,
			Autobid: autobid,
			Shares:  shares,
		}, nil

	}
	return nil, errors.New("distributor not found")
}
