// Copyright (c) 2020 The Meter.io developerslopers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"bytes"
	"errors"
	"math/big"
	"strings"

	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/ethereum/go-ethereum/rlp"
)

// the global variables in staking
var (
	StakingModuleAddr      = meter.BytesToAddress([]byte("staking-module-address")) // 0x616B696e672D6D6F64756c652d61646472657373
	DelegateListKey        = meter.Blake2b([]byte("delegate-list-key"))
	CandidateListKey       = meter.Blake2b([]byte("candidate-list-key"))
	StakeHolderListKey     = meter.Blake2b([]byte("stake-holder-list-key"))
	BucketListKey          = meter.Blake2b([]byte("global-bucket-list-key"))
	StatisticsListKey      = meter.Blake2b([]byte("delegate-statistics-list-key"))
	StatisticsEpochKey     = meter.Blake2b([]byte("delegate-statistics-epoch-key"))
	InJailListKey          = meter.Blake2b([]byte("delegate-injail-list-key"))
	ValidatorRewardListKey = meter.Blake2b([]byte("validator-reward-list-key"))
)

// Candidate List
func (s *Staking) GetCandidateList(state *state.State) (result *CandidateList) {
	state.DecodeStorage(StakingModuleAddr, CandidateListKey, func(raw []byte) error {
		candidates := make([]*Candidate, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &candidates)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding candidate list.", "err", err)
					return err
				}
			}
		}

		result = NewCandidateList(candidates)
		return nil
	})
	return
}

func (s *Staking) SetCandidateList(candList *CandidateList, state *state.State) {
	/*****
	sort.SliceStable(candList.candidates, func(i, j int) bool {
		return bytes.Compare(candList.candidates[i].Addr.Bytes(), candList.candidates[j].Addr.Bytes()) <= 0
	})
	*****/

	state.EncodeStorage(StakingModuleAddr, CandidateListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(candList.candidates)
	})
}

// StakeHolder List
func (s *Staking) GetStakeHolderList(state *state.State) (result *StakeholderList) {
	state.DecodeStorage(StakingModuleAddr, StakeHolderListKey, func(raw []byte) error {
		stakeholders := make([]*Stakeholder, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &stakeholders)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding bucket list.", "err", err)
					return err
				}
			}
		}

		result = newStakeholderList(stakeholders)
		return nil
	})
	return
}

func (s *Staking) SetStakeHolderList(holderList *StakeholderList, state *state.State) {
	/***
	sort.SliceStable(holderList.holders, func(i, j int) bool {
		return bytes.Compare(holderList.holders[i].Holder.Bytes(), holderList.holders[j].Holder.Bytes()) <= 0
	})
	***/

	state.EncodeStorage(StakingModuleAddr, StakeHolderListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(holderList.holders)
	})
}

// Bucket List
func (s *Staking) GetBucketList(state *state.State) (result *BucketList) {
	state.DecodeStorage(StakingModuleAddr, BucketListKey, func(raw []byte) error {
		buckets := make([]*Bucket, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &buckets)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding bucket list.", "err", err)
					return err
				}
			}
		}

		result = newBucketList(buckets)
		return nil
	})
	return
}

func (s *Staking) SetBucketList(bucketList *BucketList, state *state.State) {
	/***
	sort.SliceStable(bucketList.buckets, func(i, j int) bool {
		return bytes.Compare(bucketList.buckets[i].BucketID.Bytes(), bucketList.buckets[j].BucketID.Bytes()) <= 0
	})
	***/

	state.EncodeStorage(StakingModuleAddr, BucketListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(bucketList.buckets)
	})
}

// Delegates List
func (s *Staking) GetDelegateList(state *state.State) (result *DelegateList) {
	state.DecodeStorage(StakingModuleAddr, DelegateListKey, func(raw []byte) error {
		delegates := make([]*Delegate, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &delegates)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding delegate list.", "err", err)
					return err
				}
			}
		}

		result = newDelegateList(delegates)
		return nil
	})
	return
}

func (s *Staking) SetDelegateList(delegateList *DelegateList, state *state.State) {
	/***
	sort.SliceStable(delegateList.delegates, func(i, j int) bool {
		return bytes.Compare(delegateList.delegates[i].Address.Bytes(), delegateList.delegates[j].Address.Bytes()) <= 0
	})
	***/

	state.EncodeStorage(StakingModuleAddr, DelegateListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(delegateList.delegates)
	})
}

//====
// Statistics List, unlike others, save/get list
func (s *Staking) GetStatisticsList(state *state.State) (result *StatisticsList) {
	state.DecodeStorage(StakingModuleAddr, StatisticsListKey, func(raw []byte) error {
		stats := make([]*DelegateStatistics, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &stats)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding stat list.", "err", err)
					return err
				}
			}
		}

		result = NewStatisticsList(stats)
		return nil
	})
	return
}

func (s *Staking) SetStatisticsList(list *StatisticsList, state *state.State) {
	/***
	sort.SliceStable(list.delegates, func(i, j int) bool {
		return bytes.Compare(list.delegates[i].Addr.Bytes(), list.delegates[j].Addr.Bytes()) <= 0
	})
	***/

	state.EncodeStorage(StakingModuleAddr, StatisticsListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(list.delegates)
	})
}

// Statistics List, unlike others, save/get list
func (s *Staking) GetStatisticsEpoch(state *state.State) (result uint32) {
	state.DecodeStorage(StakingModuleAddr, StatisticsEpochKey, func(raw []byte) error {
		//stats := make([]*DelegateStatistics, 0)
		epoch := uint32(0)
		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &epoch)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding phaseout epoch.", "err", err)
					return err
				}
			}
		}
		result = epoch
		return nil
	})
	return
}

func (s *Staking) SetStatisticsEpoch(phaseOutEpoch uint32, state *state.State) {
	/***
	sort.SliceStable(list.delegates, func(i, j int) bool {
		return bytes.Compare(list.delegates[i].Addr.Bytes(), list.delegates[j].Addr.Bytes()) <= 0
	})
	***/
	state.EncodeStorage(StakingModuleAddr, StatisticsEpochKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(phaseOutEpoch)
	})
}

// inJail List
func (s *Staking) GetInJailList(state *state.State) (result *DelegateInJailList) {
	state.DecodeStorage(StakingModuleAddr, InJailListKey, func(raw []byte) error {
		inJails := make([]*DelegateJailed, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &inJails)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding inJail list.", "err", err)
					return err
				}
			}
		}

		result = NewDelegateInJailList(inJails)
		return nil
	})
	return
}

func (s *Staking) SetInJailList(list *DelegateInJailList, state *state.State) {
	/****
	sort.SliceStable(list.inJails, func(i, j int) bool {
		return bytes.Compare(list.inJails[i].Addr.Bytes(), list.inJails[j].Addr.Bytes()) <= 0
	})
	***/
	state.EncodeStorage(StakingModuleAddr, InJailListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(list.inJails)
	})
}

// validator reward list
func (s *Staking) GetValidatorRewardList(state *state.State) (result *ValidatorRewardList) {
	state.DecodeStorage(StakingModuleAddr, ValidatorRewardListKey, func(raw []byte) error {
		rewards := make([]*ValidatorReward, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &rewards)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding rewards list.", "err", err)
					return err
				}
			}
		}

		result = NewValidatorRewardList(rewards)
		return nil
	})
	return
}

func (s *Staking) SetValidatorRewardList(list *ValidatorRewardList, state *state.State) {
	/***
	sort.SliceStable(list.rewards, func(i, j int) bool {
		return list.rewards[i].Epoch <= list.rewards[j].Epoch
	})
	***/

	state.EncodeStorage(StakingModuleAddr, ValidatorRewardListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(list.rewards)
	})
}

//==================== bound/unbound account ===========================
func (s *Staking) BoundAccountMeter(addr meter.Address, amount *big.Int, state *state.State) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterBalance := state.GetEnergy(addr)
	meterBoundedBalance := state.GetBoundedEnergy(addr)

	// meterBalance should >= amount
	if meterBalance.Cmp(amount) == -1 {
		log.Error("not enough meter balance", "account", addr, "bound amount", amount)
		return errors.New("not enough meter balance")
	}

	state.SetEnergy(addr, new(big.Int).Sub(meterBalance, amount))
	state.SetBoundedEnergy(addr, new(big.Int).Add(meterBoundedBalance, amount))
	return nil
}

func (s *Staking) UnboundAccountMeter(addr meter.Address, amount *big.Int, state *state.State) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterBalance := state.GetEnergy(addr)
	meterBoundedBalance := state.GetBoundedEnergy(addr)

	// meterBoundedBalance should >= amount
	if meterBoundedBalance.Cmp(amount) < 0 {
		log.Error("not enough bounded meter balance", "account", addr, "unbound amount", amount)
		return errors.New("not enough bounded meter balance")
	}

	state.SetEnergy(addr, new(big.Int).Add(meterBalance, amount))
	state.SetBoundedEnergy(addr, new(big.Int).Sub(meterBoundedBalance, amount))
	return nil

}

// bound a meter gov in an account -- move amount from balance to bounded balance
func (s *Staking) BoundAccountMeterGov(addr meter.Address, amount *big.Int, state *state.State) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterGov := state.GetBalance(addr)
	meterGovBounded := state.GetBoundedBalance(addr)

	// meterGov should >= amount
	if meterGov.Cmp(amount) == -1 {
		log.Error("not enough meter-gov balance", "account", addr, "bound amount", amount)
		return errors.New("not enough meter-gov balance")
	}

	state.SetBalance(addr, new(big.Int).Sub(meterGov, amount))
	state.SetBoundedBalance(addr, new(big.Int).Add(meterGovBounded, amount))
	return nil
}

// unbound a meter gov in an account -- move amount from bounded balance to balance
func (s *Staking) UnboundAccountMeterGov(addr meter.Address, amount *big.Int, state *state.State) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterGov := state.GetBalance(addr)
	meterGovBounded := state.GetBoundedBalance(addr)

	// meterGovBounded should >= amount
	if meterGovBounded.Cmp(amount) < 0 {
		log.Error("not enough bounded meter-gov balance", "account", addr, "unbound amount", amount)
		return errors.New("not enough bounded meter-gov balance")
	}

	state.SetBalance(addr, new(big.Int).Add(meterGov, amount))
	state.SetBoundedBalance(addr, new(big.Int).Sub(meterGovBounded, amount))
	return nil
}

// collect bail to StakingModuleAddr. addr ==> StakingModuleAddr
func (s *Staking) CollectBailMeterGov(addr meter.Address, amount *big.Int, state *state.State) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterGov := state.GetBalance(addr)
	if meterGov.Cmp(amount) < 0 {
		log.Error("not enough bounded meter-gov balance", "account", addr)
		return errors.New("not enough meter-gov balance")
	}

	state.AddBalance(StakingModuleAddr, amount)
	state.SubBalance(addr, amount)
	return nil
}

//from meter.ValidatorBenefitAddr ==> addr
func (s *Staking) TransferValidatorReward(amount *big.Int, addr meter.Address, state *state.State) error {
	if amount.Sign() == 0 {
		return nil
	}

	meterBalance := state.GetEnergy(meter.ValidatorBenefitAddr)
	if meterBalance.Cmp(amount) < 0 {
		return errors.New("not enough meter")
	}

	state.AddEnergy(addr, amount)
	state.SubEnergy(meter.ValidatorBenefitAddr, amount)
	return nil
}

//1. distributes the base reward (meter.ValidatorBaseReward) for each validator. If there is remainning
//2. get the propotion reward for each validator based on the votingpower
//3. each validator takes commission first
//4. finally, distributor takes their propotions of rest
func (s *Staking) DistValidatorRewards(amount *big.Int, validators []*meter.Address, list *DelegateList, state *state.State) (*big.Int, []*RewardInfo, error) {
	rewardMap := RewardInfoMap{}
	delegatesMap := make(map[meter.Address]*Delegate)
	for _, d := range list.delegates {
		delegatesMap[d.Address] = d
	}

	var i int
	var baseRewardsOnly bool
	size := len(validators)
	votingPowerSum := big.NewInt(0)
	distReward := big.NewInt(0)
	commission := big.NewInt(0)

	// distribute the base reward
	validatorBaseReward := builtin.Params.Native(state).Get(meter.KeyValidatorBaseReward)
	baseRewards := new(big.Int).Mul(validatorBaseReward, big.NewInt(int64(size)))
	if baseRewards.Cmp(amount) >= 0 {
		baseRewards = amount
		baseRewardsOnly = true
	}

	baseReward := new(big.Int).Div(baseRewards, big.NewInt(int64(size)))
	for i = 0; i < size; i++ {
		delegate, ok := delegatesMap[*validators[i]]
		if ok == false {
			// not delegate
			log.Warn("not delegate", "address", *validators[i])
			continue
		}
		s.TransferValidatorReward(baseReward, delegate.Address, state)
		rewardMap.Add(baseReward, delegate.Address)
	}
	if baseRewardsOnly == true {
		// only cover validator base rewards
		sum, rinfo := rewardMap.ToList()
		return sum, rinfo, nil
	}

	// distributes the remaining
	rewards := new(big.Int).Sub(amount, baseRewards)
	for i = 0; i < size; i++ {
		delegate, ok := delegatesMap[*validators[i]]
		if ok == false {
			// not delegate
			log.Warn("not delegate", "address", *validators[i])
			continue
		}
		votingPowerSum = votingPowerSum.Add(votingPowerSum, delegate.VotingPower)
	}

	//
	for i = 0; i < size; i++ {
		delegate, ok := delegatesMap[*validators[i]]
		if ok == false {
			// not delegate
			log.Warn("not delegate", "address", *validators[i])
			continue
		}
		// calculate the propotion of each validator
		eachReward := new(big.Int).Mul(rewards, delegate.VotingPower)
		eachReward = eachReward.Div(eachReward, votingPowerSum)

		// distribute commission to delegate, commission unit is shannon, aka, 1e09
		commission = commission.Mul(eachReward, big.NewInt(int64(delegate.Commission)))
		commission = commission.Div(commission, big.NewInt(1e09))
		s.TransferValidatorReward(commission, delegate.Address, state)
		rewardMap.Add(commission, delegate.Address)

		actualReward := new(big.Int).Sub(eachReward, commission)

		// now distributes actualReward to each distributor
		if len(delegate.DistList) == 0 {
			// no distributor, 100% goes to benefiicary
			s.TransferValidatorReward(actualReward, delegate.Address, state)
		} else {
			// as percentage to each distributor， the unit of Shares is shannon， ie， 1e09
			for _, dist := range delegate.DistList {
				distReward = new(big.Int).Mul(actualReward, big.NewInt(int64(dist.Shares)))
				distReward = distReward.Div(distReward, big.NewInt(1e09))
				s.TransferValidatorReward(distReward, dist.Address, state)
				rewardMap.Add(distReward, dist.Address)
			}
		}
	}
	log.Info("distriubted validators rewards", "total", amount.Uint64())
	sum, rinfo := rewardMap.ToList()
	return sum, rinfo, nil
}
