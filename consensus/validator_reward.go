// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"bytes"
	"math/big"
	"sort"

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
type RewardInfoMap map[meter.Address]*RewardInfo

func (rmap RewardInfoMap) Add(amount *big.Int, addr meter.Address) error {
	info, ok := rmap[addr]
	if ok == true {
		info.Amount = info.Amount.Add(info.Amount, amount)
	} else {
		rmap[addr] = &RewardInfo{
			Address: addr,
			Amount:  amount,
		}
	}
	return nil
}

func (rmap RewardInfoMap) ToList() (*big.Int, []*RewardInfo) {
	rewards := []*RewardInfo{}
	sum := big.NewInt(0)

	for _, info := range rmap {
		sum = sum.Add(sum, info.Amount)
		rewards = append(rewards, info)
	}
	sort.SliceStable(rewards, func(i, j int) bool {
		return (bytes.Compare(rewards[i].Address.Bytes(), rewards[j].Address.Bytes()) <= 0)
	})

	return sum, rewards
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
func (conR *ConsensusReactor) CalcValidatorRewards(totalReward *big.Int, delegates []*types.Delegate) (*big.Int, []*RewardInfo, error) {

	// no distribute
	if conR.sourceDelegates == fromDelegatesFile {
		return totalReward, []*RewardInfo{}, nil
	}

	rewardMap := RewardInfoMap{}

	var i int
	var baseRewardsOnly bool
	size := len(delegates)
	votingPowerSum := big.NewInt(0)
	distReward := big.NewInt(0)
	commission := big.NewInt(0)

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

	baseReward := new(big.Int).Div(baseRewards, big.NewInt(int64(size)))
	for i = 0; i < size; i++ {
		rewardMap.Add(baseReward, delegates[i].Address)
	}

	if baseRewardsOnly == true {
		sum, rinfo := rewardMap.ToList()
		conR.logger.Info("validator dist rewards", "rewards", sum)
		return sum, rinfo, nil
	}

	// distributes the remaining. The distributing is based on
	// propotion of voting power
	for i = 0; i < size; i++ {
		votingPowerSum = votingPowerSum.Add(votingPowerSum, big.NewInt(delegates[i].VotingPower))
	}

	rewards := new(big.Int).Sub(totalReward, baseRewards)
	for i = 0; i < size; i++ {

		// calculate the propotion of each validator
		eachReward := new(big.Int).Mul(rewards, big.NewInt(delegates[i].VotingPower))
		eachReward = eachReward.Div(eachReward, votingPowerSum)

		// distribute commission to delegate, commission unit is shannon, aka, 1e09
		commission = commission.Mul(eachReward, big.NewInt(int64(delegates[i].Commission)))
		commission = commission.Div(commission, big.NewInt(1e09))
		rewardMap.Add(commission, delegates[i].Address)

		actualReward := new(big.Int).Sub(eachReward, commission)

		// now distributes actualReward to each distributor
		if len(delegates[i].DistList) == 0 {
			// no distributor, 100% goes to benefiicary
			rewardMap.Add(actualReward, delegates[i].Address)
		} else {
			// as percentage to each distributor， the unit of Shares is shannon， ie， 1e09
			for _, dist := range delegates[i].DistList {
				distReward = new(big.Int).Mul(actualReward, big.NewInt(int64(dist.Shares)))
				distReward = distReward.Div(distReward, big.NewInt(1e09))
				rewardMap.Add(distReward, dist.Address)
			}
		}
	}
	conR.logger.Info("distriubted validators rewards", "total", totalReward.Uint64())
	sum, rinfo := rewardMap.ToList()
	return sum, rinfo, nil
}

func (conR *ConsensusReactor) GetValidatorRewards(totalReward *big.Int, delegates []*types.Delegate) ([]*RewardInfo, []*RewardInfo, error) {
	_, rlist, err := conR.CalcValidatorRewards(totalReward, delegates)
	if err != nil {
		return []*RewardInfo{}, []*RewardInfo{}, err
	}

	delegatesMap := make(map[meter.Address]*types.Delegate)
	for _, d := range delegates {
		delegatesMap[d.Address] = d
	}

	var rinfo *RewardInfo
	var i int
	dist := []*RewardInfo{}
	autobid := []*RewardInfo{}
	for i = 0; i < len(rlist); i++ {
		rinfo = rlist[i]
		delegate, ok := delegatesMap[rinfo.Address]
		if ok != true {
			continue
		}

		distAmount := new(big.Int).Mul(rinfo.Amount, big.NewInt(int64(delegate.Autobid)))
		distAmount = distAmount.Div(distAmount, big.NewInt(100))
		autobidAmount := new(big.Int).Sub(rinfo.Amount, distAmount)

		dist = append(dist, &RewardInfo{rinfo.Address, distAmount})
		autobid = append(autobid, &RewardInfo{rinfo.Address, autobidAmount})
	}

	return dist, autobid, nil
}
